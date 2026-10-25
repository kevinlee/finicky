package learning

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	path string
	mu   *sync.Mutex
}

var locks sync.Map
var bundlePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,199}$`)
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:| -]{0,255}$`)
var requestPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var domainLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func NewStore(path string) *Store {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	lock, _ := locks.LoadOrStore(path, &sync.Mutex{})
	return &Store{path: path, mu: lock.(*sync.Mutex)}
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Finicky Plus", "learning", "observations.json"), nil
}

// NormalizeDomain returns only a public DNS hostname. URL credentials, IPs,
// localhost, single-label hosts, and local/internal/onion hosts are excluded.
func NormalizeDomain(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return "", ErrExcluded
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if !validDomain(host) {
		return "", ErrExcluded
	}
	return host, nil
}

func validDomain(host string) bool {
	if len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}
	// Browsers also accept shortened/hex IPv4 forms that netip rejects. A
	// public DNS name has a nonnumeric final label; reject those variants too.
	lastLabel := host[strings.LastIndexByte(host, '.')+1:]
	if !strings.ContainsAny(lastLabel, "abcdefghijklmnopqrstuvwxyz") || strings.HasPrefix(lastLabel, "0x") {
		return false
	}
	for _, suffix := range []string{".local", ".localhost", ".internal", ".onion", ".home", ".lan"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	for _, label := range strings.Split(host, ".") {
		if !domainLabelPattern.MatchString(label) {
			return false
		}
	}
	return true
}

func (s *Store) Observe(input Observation) (Request, error) {
	if !input.Enabled {
		return Request{}, ErrDisabled
	}
	if input.SourcePrivacy != Public || !bundlePattern.MatchString(input.SourceBundle) {
		return Request{}, ErrExcluded
	}
	domain, err := NormalizeDomain(input.URL)
	if err != nil {
		return Request{}, err
	}
	ids := make([]string, 0, len(input.Destinations))
	seen := map[string]bool{}
	for _, destination := range input.Destinations {
		if destination.Private {
			continue
		}
		if !idPattern.MatchString(destination.ID) {
			return Request{}, fmt.Errorf("invalid destination identity")
		}
		if !seen[destination.ID] {
			ids = append(ids, destination.ID)
			seen[destination.ID] = true
		}
	}
	if len(ids) == 0 || len(ids) > 100 {
		return Request{}, ErrExcluded
	}
	sort.Strings(ids)
	observed := input.ObservedAt.UTC()
	if observed.IsZero() {
		observed = time.Now().UTC()
	}
	received := input.ReceivedAt.UTC()
	if received.IsZero() {
		received = observed
	}
	if received.After(observed) {
		return Request{}, fmt.Errorf("request received after observation")
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return Request{}, err
	}
	request := Request{ID: hex.EncodeToString(id), ReceivedAt: received, ObservedAt: observed, Domain: domain, SourceBundle: input.SourceBundle, SourcePrivacy: Public, DestinationIDs: ids}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return Request{}, err
	}
	data.Requests = append(data.Requests, request)
	trim(&data)
	// Trimming can delete historical labels. Capture against the retained
	// dataset, never against evidence that this same transaction discards.
	if _, retained := findRequest(data, request.ID); !retained {
		return Request{}, ErrExcluded
	}
	prediction := predict(data, request, received)
	request.CapturedPrediction = &prediction
	for i := range data.Requests {
		if data.Requests[i].ID == request.ID {
			data.Requests[i] = request
			break
		}
	}
	if err := s.write(data); err != nil {
		return Request{}, err
	}
	return request, nil
}

// Confirm is exclusively for an uninfluenced, explicit chooser decision. It
// must not be called for rules, fallback routing, silence, or shadow outputs.
func (s *Store) Confirm(requestID, destinationID string, availableAt time.Time) error {
	return s.feedback(requestID, destinationID, availableAt, false)
}

func (s *Store) Correct(requestID, destinationID string, availableAt time.Time) error {
	return s.feedback(requestID, destinationID, availableAt, true)
}

func (s *Store) feedback(requestID, destinationID string, availableAt time.Time, correction bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	request, ok := findRequest(data, requestID)
	if !ok {
		return ErrNotFound
	}
	if !contains(request.DestinationIDs, destinationID) {
		return fmt.Errorf("destination was not a nonprivate candidate for this request")
	}
	if availableAt.IsZero() {
		availableAt = time.Now().UTC()
	}
	availableAt = availableAt.UTC()
	if availableAt.Before(request.ObservedAt) {
		return fmt.Errorf("feedback precedes request observation")
	}
	var previous *Feedback
	for i := range data.Feedback {
		if data.Feedback[i].RequestID == requestID {
			previous = &data.Feedback[i]
		}
	}
	kind := "neutral_chooser"
	version := 1
	if correction {
		if previous == nil {
			return fmt.Errorf("a correction requires an existing explicit confirmation")
		}
		if !availableAt.After(previous.AvailableAt) {
			return fmt.Errorf("correction availability must advance")
		}
		kind = "explicit_correction"
		version = previous.Version + 1
	} else if previous != nil {
		return fmt.Errorf("request is already confirmed; use an explicit correction")
	}
	data.Feedback = append(data.Feedback, Feedback{requestID, version, destinationID, kind, availableAt})
	trim(&data)
	return s.write(data)
}

func (s *Store) Snapshot() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

// Clear also recovers from corrupt state; other writes fail closed on corruption.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(emptySnapshot())
}

func (s *Store) DeleteRequest(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	if _, exists := findRequest(data, id); !exists {
		return nil
	}
	removeRequests(&data, map[string]bool{id: true})
	clearCapturedPredictions(&data)
	return s.write(data)
}

// DeleteDestination removes every request ever labeled with this destination,
// including all its feedback versions, so deletion cannot revive an old label.
func (s *Store) DeleteDestination(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return err
	}
	if !pruneDestinations(&data, func(candidate string) bool { return candidate != id }) {
		return nil
	}
	return s.write(data)
}

// RetainDestinations reconciles stored evidence with the currently eligible,
// nonprivate saved destinations. Identity is independent of display names, so a
// rename with the same identity is a no-op. Call after configuration changes and
// on startup. Return true only after changed state is durably written.
func (s *Store) RetainDestinations(ids []string) (bool, error) {
	retained := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !idPattern.MatchString(id) {
			return false, fmt.Errorf("invalid destination identity")
		}
		retained[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return false, err
	}
	if !pruneDestinations(&data, func(id string) bool { return retained[id] }) {
		return false, nil
	}
	if err := s.write(data); err != nil {
		return false, err
	}
	return true, nil
}

func pruneDestinations(data *Snapshot, keep func(string) bool) bool {
	removed := map[string]bool{}
	changed := false
	for _, f := range data.Feedback {
		if !keep(f.DestinationID) {
			removed[f.RequestID] = true
			changed = true
		}
	}
	for i := range data.Requests {
		r := &data.Requests[i]
		ids := r.DestinationIDs[:0]
		for _, candidate := range r.DestinationIDs {
			if keep(candidate) {
				ids = append(ids, candidate)
			} else {
				changed = true
			}
		}
		r.DestinationIDs = ids
		if len(ids) == 0 {
			removed[r.ID] = true
		}
	}
	if changed {
		removeRequests(data, removed)
		clearCapturedPredictions(data)
	}
	return changed
}

func emptySnapshot() Snapshot { return Snapshot{1, []Request{}, []Feedback{}} }

// Deletion erases derived evidence too. Remaining records can be replayed but
// replay is never relabeled as the original shadow capture.
func clearCapturedPredictions(data *Snapshot) {
	for i := range data.Requests {
		data.Requests[i].CapturedPrediction = nil
	}
}
func findRequest(data Snapshot, id string) (Request, bool) {
	for _, r := range data.Requests {
		if r.ID == id {
			return r, true
		}
	}
	return Request{}, false
}
func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func removeRequests(data *Snapshot, removed map[string]bool) {
	requests := data.Requests[:0]
	for _, r := range data.Requests {
		if !removed[r.ID] {
			requests = append(requests, r)
		}
	}
	data.Requests = requests
	feedback := data.Feedback[:0]
	for _, f := range data.Feedback {
		if !removed[f.RequestID] {
			feedback = append(feedback, f)
		}
	}
	data.Feedback = feedback
}

func trim(data *Snapshot) {
	sort.Slice(data.Requests, func(i, j int) bool {
		a, b := data.Requests[i], data.Requests[j]
		if a.ObservedAt.Equal(b.ObservedAt) {
			return a.ID < b.ID
		}
		return a.ObservedAt.Before(b.ObservedAt)
	})
	trimmed := false
	for len(data.Requests) > RequestLimit || len(data.Feedback) > FeedbackLimit {
		removeRequests(data, map[string]bool{data.Requests[0].ID: true})
		trimmed = true
	}
	if trimmed {
		clearCapturedPredictions(data)
	}
}

func (s *Store) read() (Snapshot, error) {
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return emptySnapshot(), nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxFileBytes {
		return Snapshot{}, fmt.Errorf("invalid or oversized learning store")
	}
	if info.Mode().Perm()&0077 != 0 {
		if err := os.Chmod(s.path, 0600); err != nil {
			return Snapshot{}, err
		}
	}
	file, err := os.Open(s.path)
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, MaxFileBytes+1))
	decoder.DisallowUnknownFields()
	var data Snapshot
	if err := decoder.Decode(&data); err != nil {
		return Snapshot{}, fmt.Errorf("invalid learning store: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Snapshot{}, fmt.Errorf("trailing learning store data")
	}
	if err := validate(data); err != nil {
		return Snapshot{}, fmt.Errorf("invalid learning store: %w", err)
	}
	return data, nil
}

func validate(data Snapshot) error {
	if data.Version != 1 || len(data.Requests) > RequestLimit || len(data.Feedback) > FeedbackLimit {
		return fmt.Errorf("unsupported version or retention bounds")
	}
	requests := map[string]Request{}
	for _, r := range data.Requests {
		if !requestPattern.MatchString(r.ID) || requests[r.ID].ID != "" || !validDomain(r.Domain) || r.SourcePrivacy != Public || !bundlePattern.MatchString(r.SourceBundle) || r.ObservedAt.IsZero() || r.ReceivedAt.IsZero() || r.ReceivedAt.After(r.ObservedAt) || len(r.DestinationIDs) == 0 || len(r.DestinationIDs) > 100 {
			return fmt.Errorf("invalid request")
		}
		seen := map[string]bool{}
		for _, id := range r.DestinationIDs {
			if !idPattern.MatchString(id) || seen[id] {
				return fmt.Errorf("invalid destination")
			}
			seen[id] = true
		}
		if r.CapturedPrediction != nil {
			if err := validateCapture(r, *r.CapturedPrediction); err != nil {
				return err
			}
		}
		requests[r.ID] = r
	}
	previous := map[string]Feedback{}
	for _, f := range data.Feedback {
		r, ok := requests[f.RequestID]
		p := previous[f.RequestID]
		if !ok || !contains(r.DestinationIDs, f.DestinationID) || f.AvailableAt.Before(r.ObservedAt) || f.Version != p.Version+1 {
			return fmt.Errorf("invalid feedback")
		}
		if (f.Version == 1 && f.Kind != "neutral_chooser") || (f.Version > 1 && (f.Kind != "explicit_correction" || !f.AvailableAt.After(p.AvailableAt))) {
			return fmt.Errorf("invalid feedback provenance")
		}
		previous[f.RequestID] = f
	}
	return nil
}

func validateCapture(r Request, p Prediction) error {
	if p.RequestID != r.ID || p.Support < 0 || p.Support > p.MatchingLabels || p.MatchingLabels > RequestLimit || len(p.Provenance) > 5 || len(p.Provenance) > p.Support {
		return fmt.Errorf("invalid captured prediction")
	}
	if p.DestinationID != "" {
		if !contains(r.DestinationIDs, p.DestinationID) || p.AbstainReason != "" || (p.Method != "domain" && p.Method != "domain_and_source") || p.Support == 0 {
			return fmt.Errorf("invalid captured destination")
		}
	} else {
		if p.Method != "abstain" {
			return fmt.Errorf("invalid captured abstention")
		}
		switch p.AbstainReason {
		case "no_confirmed_history", "new_or_unsupported_destination", "unseen_domain", "historical_destination_unavailable":
		default:
			return fmt.Errorf("invalid captured reason")
		}
	}
	for _, e := range p.Provenance {
		if !requestPattern.MatchString(e.RequestID) || e.FeedbackVersion < 1 || !e.AvailableAt.Before(r.ReceivedAt) || (e.Kind != "neutral_chooser" && e.Kind != "explicit_correction") {
			return fmt.Errorf("invalid captured provenance")
		}
	}
	return nil
}

func (s *Store) write(data Snapshot) error {
	if err := validate(data); err != nil {
		return err
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if len(encoded) > MaxFileBytes {
		return fmt.Errorf("learning store exceeds byte limit")
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Callers provide a dedicated directory; do not follow a symlink for it.
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("learning directory is not a directory")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(s.path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("learning store is not a regular file")
	}
	file, err := os.CreateTemp(dir, ".learning-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := io.Copy(file, bytes.NewReader(encoded)); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, s.path); err != nil {
		return err
	}
	// Sync the directory to make the rename durable where supported.
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
