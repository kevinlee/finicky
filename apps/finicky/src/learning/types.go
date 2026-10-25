// Package learning records explicitly confirmed choices for a local, shadow-only
// baseline. It never launches a browser or changes a routing decision.
package learning

import (
	"errors"
	"time"
)

const (
	// These are storage limits, not accuracy or evidence thresholds.
	RequestLimit  = 2000
	FeedbackLimit = 8000
	MaxFileBytes  = 8 << 20
)

var (
	ErrDisabled = errors.New("local learning is disabled")
	ErrExcluded = errors.New("request excluded by learning privacy policy")
	ErrNotFound = errors.New("learning request not found")
)

type Privacy string

const (
	Public  Privacy = "public"
	Private Privacy = "private"
	Unknown Privacy = "unknown"
)

type Destination struct {
	ID      string
	Private bool
}

// Observation is transient. URL is normalized before any file is written.
// Public must mean the caller positively knows this is not private browsing;
// absence of a private flag is insufficient for an unknown browser source.
type Observation struct {
	Enabled       bool
	URL           string
	SourceBundle  string
	SourcePrivacy Privacy
	ReceivedAt    time.Time
	ObservedAt    time.Time
	Destinations  []Destination
}

type Request struct {
	ID                 string      `json:"id"`
	ReceivedAt         time.Time   `json:"receivedAt"`
	ObservedAt         time.Time   `json:"observedAt"`
	Domain             string      `json:"domain"`
	SourceBundle       string      `json:"sourceBundle"`
	SourcePrivacy      Privacy     `json:"sourcePrivacy"`
	DestinationIDs     []string    `json:"destinationIds"`
	CapturedPrediction *Prediction `json:"capturedPrediction,omitempty"`
}

type Feedback struct {
	RequestID     string    `json:"requestId"`
	Version       int       `json:"version"`
	DestinationID string    `json:"destinationId"`
	Kind          string    `json:"kind"` // neutral_chooser or explicit_correction
	AvailableAt   time.Time `json:"availableAt"`
}

type Snapshot struct {
	Version  int        `json:"version"`
	Requests []Request  `json:"requests"`
	Feedback []Feedback `json:"feedback"`
}

type Evidence struct {
	RequestID       string    `json:"requestId"`
	FeedbackVersion int       `json:"feedbackVersion"`
	Kind            string    `json:"kind"`
	AvailableAt     time.Time `json:"availableAt"`
}

type Prediction struct {
	RequestID      string     `json:"requestId"`
	DestinationID  string     `json:"destinationId,omitempty"`
	Method         string     `json:"method"`
	AbstainReason  string     `json:"abstainReason,omitempty"`
	Support        int        `json:"support"`
	MatchingLabels int        `json:"matchingLabels"`
	Provenance     []Evidence `json:"provenance"`
}

type Evaluation struct {
	AsOf            time.Time `json:"asOf"`
	Observed        int       `json:"observed"`
	Labeled         int       `json:"labeled"`
	Predicted       int       `json:"predicted"`
	Correct         int       `json:"correct"`
	Errors          int       `json:"errors"`
	Abstained       int       `json:"abstained"`
	NeutralLabels   int       `json:"neutralLabels"`
	CorrectedLabels int       `json:"correctedLabels"`
	Coverage        float64   `json:"coverage"`            // among labeled requests only
	ErrorRate       *float64  `json:"errorRate,omitempty"` // among predictions only
	Support         int       `json:"support"`             // sum of supporting historical labels
	Status          string    `json:"status"`
	Limitations     []string  `json:"limitations"`
}
