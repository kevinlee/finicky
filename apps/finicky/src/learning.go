package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"finicky/browser"
	"finicky/learning"
	"finicky/rules"
	"finicky/window"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Serialize pause/reset/correction with recording, including requests whose
// chooser was already open when the user changed the learning controls.
var learningMu sync.Mutex
var learningEpoch atomic.Uint64

type learningDestination struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func learningTargetID(target browser.BrowserConfig) string {
	if target.Private || len(target.Args) != 0 || target.Name == "" || target.Name == "prompt" {
		return ""
	}
	// A display-name-only Chromium profile can be renamed or ambiguous. Learn
	// only from concrete directory identities (or an unprofiled browser).
	if target.Profile != "" && target.ProfileDirectory == "" {
		return ""
	}
	identity, _ := json.Marshal([]string{browser.CanonicalIdentifier(target.Name), target.ProfileDirectory})
	digest := sha256.Sum256(identity)
	return hex.EncodeToString(digest[:])
}

func learningDestinations(rf rules.RulesFile) []learningDestination {
	result := []learningDestination{}
	seen := map[string]bool{}
	// Match pickerDestinations: hiding a regular identity also suppresses
	// another favorite that points at the same browser/profile directory.
	for _, profile := range rf.Profiles {
		if profile.Hidden && !profile.Private {
			id := learningTargetID(browser.BrowserConfig{Name: profile.Browser, Profile: profile.Profile, ProfileDirectory: profile.ProfileDirectory})
			seen[id] = true
		}
	}
	for _, profile := range rf.Profiles {
		if profile.Hidden || profile.Private {
			continue
		}
		id := learningTargetID(browser.BrowserConfig{Name: profile.Browser, Profile: profile.Profile, ProfileDirectory: profile.ProfileDirectory})
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		name := profile.Name
		if name == "" {
			name = profile.Browser
		}
		result = append(result, learningDestination{ID: id, Name: name})
	}
	return result
}

func learningStore() (*learning.Store, error) {
	path, err := rules.GetPath()
	if err != nil {
		return nil, err
	}
	// Custom preview/test settings also get their own private learning store.
	return learning.NewStore(filepath.Join(filepath.Dir(path), "learning", "observations.json")), nil
}

func eligibleLearningSource(info URLInfo) bool {
	if info.Request.SourcePrivacy != "public" || info.Opener == nil {
		return false
	}
	// The URL parameter is browser-reported metadata, not authentication. Do
	// not accept a claim from an unrelated sender, CLI, Services or Shortcuts.
	switch info.Opener.BundleID {
	case "com.google.Chrome", "com.google.Chrome.beta", "com.google.Chrome.dev", "com.google.Chrome.canary",
		"org.chromium.Chromium", "org.mozilla.firefox", "org.mozilla.firefoxdeveloperedition",
		"com.brave.Browser", "com.microsoft.edgemac", "com.vivaldi.Vivaldi":
		return true
	}
	return false
}

func learningEnabled(rf rules.RulesFile) bool {
	return rf.Features != nil && rf.Features.LearningEnabled
}

// A settings save invalidates every pending request, including when a new
// candidate was added while its chooser was open. Serialize lifecycle pruning
// with recording so an old request cannot recreate removed profile data.
func reconcileLearning(rf rules.RulesFile) error {
	learningMu.Lock()
	defer learningMu.Unlock()
	learningEpoch.Add(1)
	store, err := learningStore()
	if err != nil {
		return err
	}
	ids := []string{}
	for _, destination := range learningDestinations(rf) {
		ids = append(ids, destination.ID)
	}
	_, err = store.RetainDestinations(ids)
	return err
}

func recordLearningChoice(info URLInfo, initial rules.RulesFile, target browser.BrowserConfig, neutral bool, launchErr error) {
	if dryRun || launchErr != nil || !learningEnabled(initial) || !eligibleLearningSource(info) ||
		target.Private || info.Request.Private || target.MatchedRule != nil || info.Request.Browser != "" || info.ReceivedAt.IsZero() {
		return
	}
	learningMu.Lock()
	defer learningMu.Unlock()
	if info.LearningEpoch != learningEpoch.Load() {
		return
	}
	latest, err := rules.Load()
	if err != nil || !learningEnabled(latest) {
		return
	}
	selectedStillAvailable := false
	for _, destination := range learningDestinations(latest) {
		selectedStillAvailable = selectedStillAvailable || destination.ID == learningTargetID(target)
	}
	if !selectedStillAvailable {
		return
	}
	store, err := learningStore()
	if err == nil {
		// Preserve the chooser's candidate set even if a settings file write
		// precedes its lifecycle callback while this lock is held.
		err = recordLearningInStore(store, info, initial, target, neutral, time.Now().UTC())
	}
	if err != nil && !errors.Is(err, learning.ErrExcluded) && !errors.Is(err, learning.ErrDisabled) {
		// Error details must not include the incoming URL or page content.
		slog.Warn("Local learning could not record a choice", "error", err)
	}
	// Replaying the whole evaluation belongs to an explicit settings refresh,
	// not the link-opening path. The panel's Refresh control reads new examples.
}

func recordLearningInStore(store *learning.Store, info URLInfo, rf rules.RulesFile, target browser.BrowserConfig, neutral bool, observedAt time.Time) error {
	if !learningEnabled(rf) || !eligibleLearningSource(info) || target.Private || info.Request.Private ||
		target.MatchedRule != nil || info.Request.Browser != "" {
		return learning.ErrExcluded
	}
	destinations := learningDestinations(rf)
	selected := learningTargetID(target)
	available := false
	choices := make([]learning.Destination, 0, len(destinations))
	for _, destination := range destinations {
		choices = append(choices, learning.Destination{ID: destination.ID})
		available = available || selected == destination.ID
	}
	if !available {
		return learning.ErrExcluded
	}
	request, err := store.Observe(learning.Observation{
		Enabled: true, URL: info.Request.URL, SourceBundle: info.Opener.BundleID,
		SourcePrivacy: learning.Public, ReceivedAt: info.ReceivedAt, ObservedAt: observedAt,
		Destinations: choices,
	})
	if err != nil || !neutral {
		return err
	}
	// Routing is already complete. This labels only a neutral chooser action,
	// never the fallback destination, an explicit rule, or absence of feedback.
	return store.Confirm(request.ID, selected, observedAt.Add(time.Nanosecond))
}

func learningSnapshot(store *learning.Store, rf rules.RulesFile, asOf time.Time) (map[string]interface{}, error) {
	snapshot, err := store.Snapshot()
	if err != nil {
		return nil, err
	}
	evaluation, err := store.Evaluate(asOf)
	if err != nil {
		return nil, err
	}
	destinations := learningDestinations(rf)
	names := map[string]string{}
	for _, destination := range destinations {
		names[destination.ID] = destination.Name
	}
	latest := map[string]learning.Feedback{}
	for _, feedback := range snapshot.Feedback {
		if feedback.AvailableAt.After(asOf) {
			continue
		}
		previous, ok := latest[feedback.RequestID]
		if !ok || feedback.Version > previous.Version {
			latest[feedback.RequestID] = feedback
		}
	}
	observations := []map[string]interface{}{}
	for i := len(snapshot.Requests) - 1; i >= 0 && len(observations) < 100; i-- {
		request := snapshot.Requests[i]
		item := map[string]interface{}{"id": request.ID, "domain": request.Domain,
			"sourceApp": request.SourceBundle, "observedAt": request.ReceivedAt}
		if feedback, ok := latest[request.ID]; ok {
			name := names[feedback.DestinationID]
			if name == "" {
				name = "Unavailable profile"
			}
			item["confirmedDestinationId"], item["confirmedDestinationName"] = feedback.DestinationID, name
			item["feedbackKind"] = feedback.Kind
		}
		if prediction := request.CapturedPrediction; prediction != nil {
			if prediction.DestinationID != "" {
				name := names[prediction.DestinationID]
				if name == "" {
					name = "Unavailable profile"
				}
				item["predictedDestinationId"], item["predictedDestinationName"] = prediction.DestinationID, name
				item["reason"] = fmt.Sprintf("%d earlier confirmed choices supported this shadow prediction.", prediction.Support)
			} else {
				item["reason"] = "Not enough eligible earlier choices for a shadow prediction."
			}
		} else {
			item["reason"] = "Prediction cleared after learning data was deleted."
		}
		observations = append(observations, item)
	}
	return map[string]interface{}{
		"enabled": learningEnabled(rf), "observations": observations,
		"feedbackCount": evaluation.Labeled, "destinations": destinations,
		"evaluation": map[string]int{"evaluated": evaluation.Labeled, "predicted": evaluation.Predicted,
			"correct": evaluation.Correct, "abstained": evaluation.Abstained},
		"limitations": append(evaluation.Limitations,
			"Predictions are shadow-only and never change routing. Scores describe confirmed choices, not all links.",
			"Learning uses browser-reported regular tabs. Private and unknown source contexts are skipped.",
			"Up to 2,000 sanitized requests are retained. Only the latest 100 are shown here."),
	}, nil
}

func handleLearningMessage(msg map[string]interface{}) bool {
	kind, _ := msg["type"].(string)
	switch kind {
	case "getLearning", "setLearningEnabled", "clearLearning", "correctLearning", "deleteLearning":
	default:
		return false
	}
	// saveAndApply calls the shared lifecycle handler, which takes learningMu.
	// Keep this configuration mutation outside that lock; all data operations
	// below remain serialized with recording and profile reconciliation.
	if kind == "setLearningEnabled" {
		rf, err := rules.Load()
		enabled, valid := msg["enabled"].(bool)
		if !valid {
			err = fmt.Errorf("an enabled value is required")
		}
		if err == nil {
			if rf.Features == nil {
				rf.Features = &rules.Features{}
			}
			rf.Features.LearningEnabled = enabled
			err = saveAndApply(rf)
			if err == nil {
				window.SendMessageToWebView("features", rf.Features)
			}
		}
		if err != nil {
			window.SendMessageToWebView("learningResult", map[string]interface{}{
				"success": false, "operation": kind, "error": err.Error(),
			})
			return true
		}
	}
	learningMu.Lock()
	defer learningMu.Unlock()
	rf, err := rules.Load()
	var store *learning.Store
	if err == nil {
		store, err = learningStore()
	}
	if err == nil {
		switch kind {
		case "clearLearning":
			err = store.Clear()
			if err == nil {
				learningEpoch.Add(1)
			}
		case "deleteLearning":
			id, _ := msg["requestId"].(string)
			err = store.DeleteRequest(id)
			if err == nil {
				learningEpoch.Add(1)
			}
		case "correctLearning":
			id, _ := msg["requestId"].(string)
			destinationID, _ := msg["destinationId"].(string)
			available := false
			for _, destination := range learningDestinations(rf) {
				available = available || destination.ID == destinationID
			}
			if !available {
				err = fmt.Errorf("select an available regular saved profile")
			} else {
				err = store.Correct(id, destinationID, time.Now().UTC())
			}
		}
	}
	if err == nil {
		var snapshot map[string]interface{}
		snapshot, err = learningSnapshot(store, rf, time.Now().UTC())
		if err == nil {
			window.SendMessageToWebView("learning", snapshot)
		}
	}
	result := map[string]interface{}{"success": err == nil, "operation": kind}
	if err != nil {
		result["error"] = err.Error()
	}
	window.SendMessageToWebView("learningResult", result)
	return true
}
