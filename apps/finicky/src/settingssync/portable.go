package settingssync

import (
	"encoding/json"
	"finicky/browser"
	"finicky/rules"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var logicalID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
var revisionID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var browserID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,199}$`)

func canonicalBrowser(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	canonical := browser.CanonicalIdentifier(value)
	if !browserID.MatchString(canonical) || browser.IsRouter(canonical) {
		return "", fmt.Errorf("browser must be a portable bundle identifier or a recognized browser")
	}
	return canonical, nil
}

func validateBinding(b Binding) error {
	canonical, err := canonicalBrowser(b.Browser)
	if err != nil {
		return err
	}
	if canonical == "" {
		return fmt.Errorf("mapping requires a browser")
	}
	if len(b.Profile) > 200 || strings.ContainsAny(b.Profile, "/\\\x00\r\n") {
		return fmt.Errorf("mapping requires a profile name, not a path")
	}
	if b.ProfileDirectory != "" && (len(b.ProfileDirectory) > 200 || b.ProfileDirectory == "." || b.ProfileDirectory == ".." || strings.ContainsAny(b.ProfileDirectory, "/\\\x00\r\n")) {
		return fmt.Errorf("mapping requires a single profile directory name")
	}
	return browser.ValidateProfileDirectory(browser.BrowserConfig{Name: canonical, ProfileDirectory: b.ProfileDirectory})
}

func sameBinding(a, b Binding) bool {
	ab, _ := canonicalBrowser(a.Browser)
	bb, _ := canonicalBrowser(b.Browser)
	if ab != bb {
		return false
	}
	if a.ProfileDirectory != "" || b.ProfileDirectory != "" {
		return a.ProfileDirectory == b.ProfileDirectory
	}
	return a.Profile == b.Profile
}

func profileBinding(p rules.SavedProfile) Binding {
	return Binding{p.Browser, p.Profile, p.ProfileDirectory}
}

func exportSettings(current rules.RulesFile) (portable, error) {
	if err := rules.Validate(current); err != nil {
		return portable{}, err
	}
	p := portable{Profiles: []profile{}, Rules: []sharedRule{}}
	seen := map[string]bool{}
	for _, favorite := range current.Profiles {
		if !logicalID.MatchString(favorite.ID) || seen[favorite.ID] {
			return p, fmt.Errorf("saved profiles require unique portable IDs")
		}
		seen[favorite.ID] = true
		if err := validateBinding(profileBinding(favorite)); err != nil {
			return p, err
		}
		p.Profiles = append(p.Profiles, profile{favorite.ID, favorite.Name, favorite.Color, favorite.Shortcut, favorite.Hidden, favorite.Private})
	}
	convert := func(b Binding, private bool) (target, error) {
		name, err := canonicalBrowser(b.Browser)
		if err != nil {
			return target{}, err
		}
		if b.Profile == "" && b.ProfileDirectory == "" {
			return target{Browser: name, Private: private}, nil
		}
		matches := []string{}
		for _, favorite := range current.Profiles {
			if sameBinding(b, profileBinding(favorite)) {
				matches = append(matches, favorite.ID)
			}
		}
		if len(matches) != 1 {
			return target{}, fmt.Errorf("every concrete rule/default/alternative profile must match exactly one saved profile")
		}
		return target{ProfileID: matches[0], Private: private}, nil
	}
	var err error
	p.Default, err = convert(Binding{current.DefaultBrowser, current.DefaultProfile, current.DefaultProfileDirectory}, current.DefaultPrivate)
	if err != nil {
		return p, err
	}
	for _, r := range current.Rules {
		if r.TransformScript != "" {
			return p, fmt.Errorf("transform scripts cannot be synced; remove the script before publishing settings")
		}
		t, err := convert(Binding{r.Browser, r.Profile, r.ProfileDirectory}, r.Private)
		if err != nil {
			return p, err
		}
		p.Rules = append(p.Rules, sharedRule{append([]string{}, r.Match...), t, r.Name, append([]string(nil), r.SourceApps...), r.Enabled, r.NewWindow})
	}
	if current.Options != nil {
		p.Options = sharedOptions{current.Options.KeepRunning, current.Options.HideIcon}
	}
	if f := current.Features; f != nil {
		if f.TransformScript != "" {
			return p, fmt.Errorf("transform scripts cannot be synced; remove the script before publishing settings")
		}
		a, err := convert(Binding{f.AlternativeBrowser, f.AlternativeProfile, f.AlternativeProfileDirectory}, f.AlternativePrivate)
		if err != nil {
			return p, err
		}
		p.Features = sharedFeatures{f.MenuIcon, f.PromptMode, f.PromptModifier, f.StripTracking, f.UnwrapRedirects, f.ExpandShortURLs, append([]string(nil), f.ExtraShorteners...), f.NativeApps, copyStrings(f.NativeAppServices), a}
	}
	return p, validatePortable(p)
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
func equalSettings(a, b portable) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func validatePortable(p portable) error {
	if len(p.Profiles) > 100 || len(p.Rules) > 500 {
		return fmt.Errorf("portable settings exceed profile or rule limits")
	}
	profiles := map[string]bool{}
	for _, v := range p.Profiles {
		if !logicalID.MatchString(v.ID) || profiles[v.ID] || len(v.Name) > 200 || len(v.Color) > 50 || len(v.Shortcut) > 50 {
			return fmt.Errorf("invalid shared profile")
		}
		profiles[v.ID] = true
	}
	checkTarget := func(t target) error {
		if t.ProfileID != "" {
			if t.Browser != "" || !profiles[t.ProfileID] {
				return fmt.Errorf("unknown or ambiguous logical profile")
			}
			return nil
		}
		canonical, err := canonicalBrowser(t.Browser)
		if err != nil {
			return err
		}
		if canonical != t.Browser {
			return fmt.Errorf("shared browser must be canonical")
		}
		return nil
	}
	if err := checkTarget(p.Default); err != nil {
		return err
	}
	if err := checkTarget(p.Features.Alternative); err != nil {
		return err
	}
	for _, r := range p.Rules {
		if err := checkTarget(r.Target); err != nil {
			return err
		}
		if len(r.Match) > 100 || len(r.SourceApps) > 100 || len(r.Name) > 500 {
			return fmt.Errorf("shared rule exceeds limits")
		}
		for _, m := range r.Match {
			if len(m) > 4096 || strings.ContainsRune(m, 0) {
				return fmt.Errorf("invalid rule pattern")
			}
		}
		for _, source := range r.SourceApps {
			if !browserID.MatchString(source) {
				return fmt.Errorf("invalid source application identifier")
			}
		}
	}
	f := p.Features
	return rules.ValidateFeatures(&rules.Features{MenuIcon: f.MenuIcon, PromptMode: f.PromptMode, PromptModifier: f.PromptModifier, ExtraShorteners: f.ExtraShorteners})
}

func missingBindings(p portable, bindings map[string]Binding) []string {
	missing := []string{}
	for _, v := range p.Profiles {
		if b, ok := bindings[v.ID]; !ok || validateBinding(b) != nil {
			missing = append(missing, v.ID)
		}
	}
	sort.Strings(missing)
	return missing
}

func materialize(p portable, bindings map[string]Binding, local rules.RulesFile) (rules.RulesFile, error) {
	if err := validatePortable(p); err != nil {
		return rules.RulesFile{}, err
	}
	if missing := missingBindings(p, bindings); len(missing) > 0 {
		return rules.RulesFile{}, fmt.Errorf("map shared profiles to local destinations before applying")
	}
	result := rules.RulesFile{Rules: []rules.Rule{}, Profiles: []rules.SavedProfile{}}
	resolve := func(t target) Binding {
		if t.ProfileID != "" {
			return bindings[t.ProfileID]
		}
		return Binding{Browser: t.Browser}
	}
	d := resolve(p.Default)
	result.DefaultBrowser = d.Browser
	result.DefaultProfile = d.Profile
	result.DefaultProfileDirectory = d.ProfileDirectory
	result.DefaultPrivate = p.Default.Private
	for _, v := range p.Profiles {
		b := bindings[v.ID]
		result.Profiles = append(result.Profiles, rules.SavedProfile{ID: v.ID, Name: v.Name, Color: v.Color, Shortcut: v.Shortcut, Hidden: v.Hidden, Private: v.Private, Browser: b.Browser, Profile: b.Profile, ProfileDirectory: b.ProfileDirectory})
	}
	for _, r := range p.Rules {
		b := resolve(r.Target)
		result.Rules = append(result.Rules, rules.Rule{Match: copyStrings(r.Match), Browser: b.Browser, Profile: b.Profile, ProfileDirectory: b.ProfileDirectory, Name: r.Name, SourceApps: copyStrings(r.SourceApps), Enabled: r.Enabled, Private: r.Target.Private, NewWindow: r.NewWindow})
	}
	result.Options = &rules.Options{KeepRunning: p.Options.KeepRunning, HideIcon: p.Options.HideIcon}
	if local.Options != nil {
		result.Options.LogRequests = local.Options.LogRequests
		result.Options.CheckForUpdates = local.Options.CheckForUpdates
	}
	f := p.Features
	a := resolve(f.Alternative)
	result.Features = &rules.Features{MenuIcon: f.MenuIcon, PromptMode: f.PromptMode, PromptModifier: f.PromptModifier, StripTracking: f.StripTracking, UnwrapRedirects: f.UnwrapRedirects, ExpandShortURLs: f.ExpandShortURLs, ExtraShorteners: copyStrings(f.ExtraShorteners), NativeApps: f.NativeApps, NativeAppServices: copyStrings(f.NativeAppServices), AlternativeBrowser: a.Browser, AlternativeProfile: a.Profile, AlternativeProfileDirectory: a.ProfileDirectory, AlternativePrivate: f.Alternative.Private}
	if local.Features != nil {
		result.Features.HistoryEnabled = local.Features.HistoryEnabled
		result.Features.LearningEnabled = local.Features.LearningEnabled
	}
	for _, p := range result.Profiles {
		if p.Private && !browser.Capabilities(p.Browser).Private {
			return result, fmt.Errorf("mapped browser does not support the shared private destination")
		}
	}
	if result.DefaultPrivate && !browser.Capabilities(result.DefaultBrowser).Private {
		return result, fmt.Errorf("mapped default browser does not support private mode")
	}
	if result.Features.AlternativePrivate && !browser.Capabilities(result.Features.AlternativeBrowser).Private {
		return result, fmt.Errorf("mapped alternative browser does not support private mode")
	}
	for _, r := range result.Rules {
		capabilities := browser.Capabilities(r.Browser)
		if (r.Private && !capabilities.Private) || (r.NewWindow && !capabilities.NewWindow) {
			return result, fmt.Errorf("mapped rule browser does not support its requested launch mode")
		}
	}
	return result, rules.Validate(result)
}
