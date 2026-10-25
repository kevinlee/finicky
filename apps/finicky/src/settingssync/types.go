// Package settingssync exchanges portable settings revisions. It never reads
// browsing data, changes live routing, or applies a proposed RulesFile itself.
package settingssync

import (
	"finicky/rules"
	"time"
)

const MaxRevisions = 512
const MaxRevisionBytes = 1 << 20

type Binding struct {
	Browser          string `json:"browser"`
	Profile          string `json:"profile,omitempty"`
	ProfileDirectory string `json:"profileDirectory,omitempty"`
}

type ProfileInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color,omitempty"`
	Private bool   `json:"private,omitempty"`
	Hidden  bool   `json:"hidden,omitempty"`
	Mapped  bool   `json:"mapped"`
}

type Status struct {
	Enabled           bool               `json:"enabled"`
	State             string             `json:"state"`
	Message           string             `json:"message"`
	DeviceID          string             `json:"deviceId,omitempty"`
	AppliedRevision   string             `json:"appliedRevision,omitempty"`
	Heads             []string           `json:"heads"`
	HeadDetails       []HeadInfo         `json:"headDetails"`
	Profiles          []ProfileInfo      `json:"profiles"`
	Bindings          map[string]Binding `json:"bindings"`
	MissingProfileIDs []string           `json:"missingProfileIds"`
}

type HeadInfo struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"deviceId"`
	CreatedAt time.Time `json:"createdAt"`
}

type Update struct {
	Revision string          `json:"revision"`
	Rules    rules.RulesFile `json:"rules"`
}

type target struct {
	ProfileID string `json:"profileId,omitempty"`
	Browser   string `json:"browser,omitempty"`
	Private   bool   `json:"private,omitempty"`
}

type profile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	Shortcut string `json:"shortcut,omitempty"`
	Hidden   bool   `json:"hidden,omitempty"`
	Private  bool   `json:"private,omitempty"`
}

type sharedRule struct {
	Match      []string `json:"match"`
	Target     target   `json:"target"`
	Name       string   `json:"name,omitempty"`
	SourceApps []string `json:"sourceApps,omitempty"`
	Enabled    *bool    `json:"enabled,omitempty"`
	NewWindow  bool     `json:"newWindow,omitempty"`
}

type sharedOptions struct {
	KeepRunning *bool `json:"keepRunning,omitempty"`
	HideIcon    *bool `json:"hideIcon,omitempty"`
}

type sharedFeatures struct {
	MenuIcon          string   `json:"menuIcon,omitempty"`
	PromptMode        string   `json:"promptMode,omitempty"`
	PromptModifier    string   `json:"promptModifier,omitempty"`
	StripTracking     bool     `json:"stripTracking,omitempty"`
	UnwrapRedirects   bool     `json:"unwrapRedirects,omitempty"`
	ExpandShortURLs   *bool    `json:"expandShortURLs,omitempty"`
	ExtraShorteners   []string `json:"extraShorteners,omitempty"`
	NativeApps        bool     `json:"nativeApps,omitempty"`
	NativeAppServices []string `json:"nativeAppServices"`
	Alternative       target   `json:"alternative"`
}

type portable struct {
	Default  target         `json:"default"`
	Profiles []profile      `json:"profiles"`
	Rules    []sharedRule   `json:"rules"`
	Options  sharedOptions  `json:"options"`
	Features sharedFeatures `json:"features"`
}

type revision struct {
	Version   int       `json:"version"`
	ID        string    `json:"id"`
	DeviceID  string    `json:"deviceId"`
	Parents   []string  `json:"parents"`
	CreatedAt time.Time `json:"createdAt"`
	Settings  portable  `json:"settings"`
}

type state struct {
	Version           int                `json:"version"`
	Enabled           bool               `json:"enabled"`
	DeviceID          string             `json:"deviceId"`
	Applied           string             `json:"applied,omitempty"`
	Base              *portable          `json:"base,omitempty"`
	Bindings          map[string]Binding `json:"bindings"`
	AppliedBindings   map[string]Binding `json:"appliedBindings,omitempty"`
	Pending           *revision          `json:"pending,omitempty"`
	PendingResolution bool               `json:"pendingResolution,omitempty"`
	HistoryObserved   bool               `json:"historyObserved,omitempty"`
}
