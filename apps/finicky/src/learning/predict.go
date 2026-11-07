package learning

import (
	"sort"
	"time"
)

// Predict recomputes a shadow suggestion from the immutable request candidate
// list. It is not the captured prediction. For current availability, observe a
// new request with the current candidates rather than modifying an old request.
func (s *Store) Predict(requestID string, cutoff time.Time) (Prediction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return Prediction{}, err
	}
	r, ok := findRequest(data, requestID)
	if !ok {
		return Prediction{}, ErrNotFound
	}
	if cutoff.IsZero() {
		cutoff = r.ReceivedAt
	}
	return predict(data, r, cutoff), nil
}

type labeledRequest struct {
	request  Request
	feedback Feedback
}

func predict(data Snapshot, query Request, cutoff time.Time) Prediction {
	p := Prediction{RequestID: query.ID, Method: "abstain", Provenance: []Evidence{}}
	// A later requested cutoff must not make future knowledge appear available
	// at the request's original decision time.
	if cutoff.After(query.ReceivedAt) {
		cutoff = query.ReceivedAt
	}
	labels := map[string]Feedback{}
	for _, f := range data.Feedback {
		if f.AvailableAt.Before(cutoff) {
			if old, ok := labels[f.RequestID]; !ok || f.Version > old.Version {
				labels[f.RequestID] = f
			}
		}
	}
	eligible := []labeledRequest{}
	supported := map[string]bool{}
	for _, r := range data.Requests {
		if r.ID == query.ID || !r.ReceivedAt.Before(query.ReceivedAt) || !r.ObservedAt.Before(cutoff) {
			continue
		}
		f, ok := labels[r.ID]
		if !ok {
			continue
		}
		eligible = append(eligible, labeledRequest{r, f})
		supported[f.DestinationID] = true
	}
	if len(eligible) == 0 {
		p.AbstainReason = "no_confirmed_history"
		return p
	}
	// Introducing a never-confirmed candidate invalidates support for choosing
	// among this set. This is a structural cold-start check, not confidence.
	for _, id := range query.DestinationIDs {
		if !supported[id] {
			p.AbstainReason = "new_or_unsupported_destination"
			return p
		}
	}
	matches := []labeledRequest{}
	for _, row := range eligible {
		if row.request.Domain == query.Domain && row.request.SourceBundle == query.SourceBundle {
			matches = append(matches, row)
		}
	}
	p.Method = "domain_and_source"
	if len(matches) == 0 {
		p.Method = "domain"
		for _, row := range eligible {
			if row.request.Domain == query.Domain {
				matches = append(matches, row)
			}
		}
	}
	if len(matches) == 0 {
		p.Method = "abstain"
		p.AbstainReason = "unseen_domain"
		return p
	}
	p.MatchingLabels = len(matches)
	type vote struct {
		id       string
		count    int
		latest   time.Time
		examples []Evidence
	}
	votes := map[string]*vote{}
	for _, row := range matches {
		f := row.feedback
		v := votes[f.DestinationID]
		if v == nil {
			v = &vote{id: f.DestinationID}
			votes[f.DestinationID] = v
		}
		v.count++
		if f.AvailableAt.After(v.latest) {
			v.latest = f.AvailableAt
		}
		v.examples = append(v.examples, Evidence{f.RequestID, f.Version, f.Kind, f.AvailableAt})
	}
	ranked := make([]*vote, 0, len(votes))
	for _, v := range votes {
		ranked = append(ranked, v)
	}
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.count != b.count {
			return a.count > b.count
		}
		if !a.latest.Equal(b.latest) {
			return a.latest.After(b.latest)
		}
		return a.id < b.id
	})
	winner := ranked[0]
	p.Support = winner.count
	// Rank against all historical labels first; dropping unavailable choices
	// before ranking would silently promote a different answer.
	if !contains(query.DestinationIDs, winner.id) {
		p.Method = "abstain"
		p.AbstainReason = "historical_destination_unavailable"
		return p
	}
	p.DestinationID = winner.id
	sort.Slice(winner.examples, func(i, j int) bool {
		a, b := winner.examples[i], winner.examples[j]
		if a.AvailableAt.Equal(b.AvailableAt) {
			return a.RequestID < b.RequestID
		}
		return a.AvailableAt.After(b.AvailableAt)
	})
	if len(winner.examples) > 5 {
		winner.examples = winner.examples[:5]
	}
	p.Provenance = winner.examples
	return p
}

// Evaluate replays at each request's ReceivedAt. Outcomes use the first explicit
// neutral-chooser label available by asOf; later corrections are counted but do
// not silently rewrite that original evaluation outcome. Training at each point
// uses the latest correction that was already available at that point.
func (s *Store) Evaluate(asOf time.Time) (Evaluation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.read()
	if err != nil {
		return Evaluation{}, err
	}
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	result := Evaluation{AsOf: asOf, Status: "data_not_ready", Limitations: []string{
		"Only explicit neutral chooser decisions are labeled; this is not accuracy over all opened links.",
		"Predictions replay strict historical availability, not retained original capture results.",
		"Outcomes use first neutral chooser labels; later corrections are counted separately.",
		"Small, selected and correlated samples do not establish calibrated confidence or justify automatic routing or a GNN.",
		"Retention and deletion change the replay dataset; excluded private and unknown sources are never evaluated.",
	}}
	first := map[string]Feedback{}
	corrected := map[string]bool{}
	for _, f := range data.Feedback {
		if f.AvailableAt.After(asOf) {
			continue
		}
		if f.Version == 1 {
			first[f.RequestID] = f
		} else {
			corrected[f.RequestID] = true
		}
	}
	for _, r := range data.Requests {
		if r.ObservedAt.After(asOf) {
			continue
		}
		result.Observed++
		label, ok := first[r.ID]
		if !ok {
			continue
		}
		result.Labeled++
		result.NeutralLabels++
		if corrected[r.ID] {
			result.CorrectedLabels++
		}
		prediction := predict(data, r, r.ReceivedAt)
		if prediction.DestinationID == "" {
			result.Abstained++
			continue
		}
		result.Predicted++
		result.Support += prediction.Support
		if prediction.DestinationID == label.DestinationID {
			result.Correct++
		} else {
			result.Errors++
		}
	}
	if result.Labeled > 0 {
		result.Coverage = float64(result.Predicted) / float64(result.Labeled)
		result.Status = "descriptive_labeled_only"
	}
	if result.Predicted > 0 {
		rate := float64(result.Errors) / float64(result.Predicted)
		result.ErrorRate = &rate
	}
	return result, nil
}
