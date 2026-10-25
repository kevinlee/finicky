package main

/*
#include <stdlib.h>
#include "main.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"finicky/browser"
	"finicky/browseraccess"
	"finicky/linkfeatures"
	"finicky/picker"
	"finicky/resolver"
	"finicky/rules"
	"finicky/urlrequest"
	"finicky/util"
	"finicky/window"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

func printTestResult(config *browser.BrowserConfig, err error) {
	result := map[string]interface{}{"browser": config}
	if err != nil {
		result["error"] = err.Error()
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func history() *linkfeatures.Store {
	path, _ := rules.GetPath()
	return linkfeatures.NewStore(filepath.Join(filepath.Dir(path), "history.json"))
}

func enqueueURL(raw string, opener *resolver.OpenerInfo, background, prompt bool) {
	r, err := urlrequest.Parse(raw)
	if err != nil {
		slog.Error("Invalid link request", "error", err)
		return
	}
	r.Prompt = r.Prompt || prompt
	info := URLInfo{ReceivedAt: time.Now().UTC(), LearningEpoch: learningEpoch.Load(), URL: r.URL, Request: r, Opener: opener, OpenInBackground: background || r.Background, Modifiers: util.GetModifierKeys()}
	// Apple event callbacks run on the main thread: never block the UI on a picker.
	select {
	case urlListener <- info:
	default:
		slog.Error("Link queue is full")
	}
}

func routeRequest(info URLInfo) {
	r := info.Request
	rf, err := rules.Load()
	if err != nil {
		handleRuntimeError(err)
		return
	}
	f := rf.Features
	if f == nil {
		f = &rules.Features{}
	}
	switch r.Action {
	case "settings":
		ShowConfigWindow()
		return
	case "clipboard":
		for _, link := range urlrequest.Extract(readClipboard()) {
			enqueueURL(link, info.Opener, info.OpenInBackground, r.Prompt)
		}
		return
	case "set-default":
		if err = applyAndSaveDefaultRequest(rf, r, saveAndApply); err != nil {
			handleRuntimeError(err)
		}
		return
	}
	if r.Action == "copy" {
		cleanOptions := *f
		cleanOptions.StripTracking = true
		cleanOptions.UnwrapRedirects = true
		cleaned, cleanErr := linkfeatures.PrepareURL(r.URL, &cleanOptions)
		if cleanErr != nil {
			slog.Warn("Copy clean URL", "error", cleanErr)
		}
		copyText(cleaned)
		return
	}
	var target *browser.BrowserConfig
	if r.Browser != "" {
		cleaned, e := linkfeatures.PrepareURL(r.URL, f)
		if e != nil {
			slog.Warn("Link processing", "error", e)
		}
		target = &browser.BrowserConfig{Name: r.Browser, AppType: appType(r.Browser), Profile: r.Profile, ProfileDirectory: r.ProfileDirectory, URL: cleaned, Private: r.Private, NewWindow: r.NewWindow}
		if r.Browser == "prompt" {
			target.Action = "prompt"
			target.AppType = "none"
			target.Name = ""
		}
	} else {
		vmMu.Lock()
		target, err = resolver.ResolveURL(vm, r.URL, info.Opener, info.OpenInBackground)
		vmMu.Unlock()
		if err != nil {
			handleRuntimeError(err)
			return
		}
	}
	if target == nil {
		return
	}
	if r.Action == "share" {
		shareURL(target.URL)
		return
	}
	modifier := f.PromptModifier
	if modifier == "" {
		modifier = "fn"
	}
	modifierHeld := modifier != "none" && info.Modifiers[modifier]
	neutralChoice := false
	prompt := r.Prompt || target.Action == "prompt" || f.PromptMode == "always" || f.PromptMode == "unmatched" && target.MatchedRule == nil
	if modifierHeld {
		if f.AlternativeBrowser != "" && f.AlternativeBrowser != "prompt" {
			prompt = false
			alternative := alternativeBrowserTarget(*target, r, f)
			target = &alternative
		} else {
			prompt = true
		}
	}
	if prompt {
		destinations := pickerDestinations(rf)
		choice := <-picker.Show(requestForPicker(*target, r, info.OpenInBackground, destinations))
		if choice.Cancelled {
			return
		}
		if choice.Copy {
			copyText(target.URL)
			return
		}
		if choice.Share {
			shareURL(target.URL)
			return
		}
		if choice.Index < 0 || choice.Index >= len(destinations) {
			return
		}
		selected := selectedPickerTarget(*target, destinations[choice.Index].Browser, r, choice)
		target = &selected
		neutralChoice = true
		if e := browser.ValidateTarget(*target); e != nil {
			handleRuntimeError(e)
			return
		}
		if choice.Remember {
			// Settings may have changed while the user considered the picker.
			latest, loadErr := rules.Load()
			if loadErr != nil {
				handleRuntimeError(loadErr)
				return
			}
			if parsed, e := url.Parse(target.URL); e == nil && parsed.Hostname() != "" {
				rule := rules.Rule{Name: parsed.Hostname(), Match: []string{"https://" + parsed.Host + "/*", "http://" + parsed.Host + "/*"}, Browser: target.Name, Profile: target.Profile, ProfileDirectory: target.ProfileDirectory, Private: target.Private, NewWindow: target.NewWindow}
				latest.Rules = append([]rules.Rule{rule}, latest.Rules...)
				if e = saveAndApply(latest); e != nil {
					handleRuntimeError(e)
				}
			}
		}
	} else if f.NativeApps && target.AppType != "none" && !target.Private && !r.Private && !target.NewWindow && !r.NewWindow && target.MatchedRule == nil && r.Browser == "" && !modifierHeld {
		if native, ok := linkfeatures.NativeDestinationWithServices(target.URL, f.NativeAppServices); ok {
			target = &native
		}
	}
	target.Private = target.Private || r.Private
	target.NewWindow = target.NewWindow || r.NewWindow
	err = browser.LaunchBrowser(*target, dryRun, info.OpenInBackground)
	if err != nil {
		handleRuntimeError(err)
	} else {
		C.SetStatusItemError(false)
	}
	recordLearningChoice(info, rf, *target, neutralChoice, err)
	if f.HistoryEnabled && !dryRun && !target.Private {
		entry := linkfeatures.HistoryEntry{MatchedRule: target.MatchedRule, URL: target.URL, OriginalURL: r.URL, Browser: target.Name, Profile: target.Profile, ProfileDirectory: target.ProfileDirectory, Timestamp: time.Now()}
		if info.Opener != nil {
			entry.SourceApp = info.Opener.BundleID
		}
		if err != nil {
			entry.Error = err.Error()
		}
		if e := history().Add(entry); e != nil {
			slog.Warn("Could not save history", "error", e)
		}
	}
}

func requestForPicker(target browser.BrowserConfig, r urlrequest.Request, background bool, destinations []picker.Destination) picker.Request {
	if target.OpenInBackground != nil {
		background = *target.OpenInBackground
	}
	return picker.Request{
		URL: target.URL, Destinations: destinations,
		Private: target.Private || r.Private, NewWindow: target.NewWindow || r.NewWindow,
		Background: background,
	}
}

// Changing the destination must not discard the link's required launch modes.
// Browser-specific arguments and profile identity remain those of the choice.
func retargetURL(resolved, selected browser.BrowserConfig, r urlrequest.Request) browser.BrowserConfig {
	selected.URL = resolved.URL
	selected.MatchedRule = resolved.MatchedRule
	selected.Private = selected.Private || resolved.Private || r.Private
	selected.NewWindow = selected.NewWindow || resolved.NewWindow || r.NewWindow
	if selected.OpenInBackground == nil {
		selected.OpenInBackground = resolved.OpenInBackground
	}
	return selected
}

func alternativeBrowserTarget(resolved browser.BrowserConfig, r urlrequest.Request, f *rules.Features) browser.BrowserConfig {
	return retargetURL(resolved, browser.BrowserConfig{
		Name: f.AlternativeBrowser, AppType: appType(f.AlternativeBrowser),
		Profile: f.AlternativeProfile, ProfileDirectory: f.AlternativeProfileDirectory,
		Private: f.AlternativePrivate,
	}, r)
}

func selectedPickerTarget(resolved, selected browser.BrowserConfig, r urlrequest.Request, choice picker.Selection) browser.BrowserConfig {
	selected = retargetURL(resolved, selected, r)
	selected.Private = selected.Private || choice.Private
	selected.NewWindow = selected.NewWindow || choice.NewWindow
	if choice.BackgroundSpecified || choice.Background {
		background := choice.Background
		selected.OpenInBackground = &background
	}
	return selected
}

func appType(name string) string {
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "~/") {
		return "path"
	}
	if strings.Contains(name, ".") && !strings.Contains(name, " ") {
		return "bundleId"
	}
	return "appName"
}

func destinationKey(target browser.BrowserConfig) string {
	profile := target.Profile
	if target.ProfileDirectory != "" {
		profile = target.ProfileDirectory
	}
	return browser.CanonicalIdentifier(target.Name) + "|" + profile + fmt.Sprint(target.Private)
}

func pickerDestinations(rf rules.RulesFile) []picker.Destination {
	result := []picker.Destination{}
	seen := map[string]bool{}
	profileCache := map[string][]browser.BrowserProfile{}
	profilesFor := func(name string) []browser.BrowserProfile {
		key := browser.CanonicalIdentifier(name)
		if profiles, ok := profileCache[key]; ok {
			return profiles
		}
		profiles := browser.GetBrowserProfiles(name)
		profileCache[key] = profiles
		return profiles
	}
	add := func(id, name, subtitle, shortcut, color string, target browser.BrowserConfig) {
		if name == "" || target.Name == "prompt" || strings.Contains(strings.ToLower(target.Name), "finicky") {
			return
		}
		key := destinationKey(target)
		if seen[key] {
			return
		}
		seen[key] = true
		result = append(result, picker.Destination{ID: id, Name: name, Subtitle: subtitle, Shortcut: shortcut, Color: color, Browser: target})
	}
	for _, p := range rf.Profiles {
		target := browser.BrowserConfig{Name: p.Browser, AppType: appType(p.Browser), Profile: p.Profile, ProfileDirectory: p.ProfileDirectory, Private: p.Private}
		target = stableFavorite(target, profilesFor(p.Browser))
		if p.Hidden {
			seen[destinationKey(target)] = true
		}
	}
	for _, p := range rf.Profiles {
		if !p.Hidden {
			target := stableFavorite(browser.BrowserConfig{Name: p.Browser, AppType: appType(p.Browser), Profile: p.Profile, ProfileDirectory: p.ProfileDirectory, Private: p.Private}, profilesFor(p.Browser))
			add(p.ID, p.Name, p.Browser, p.Shortcut, p.Color, target)
		}
	}
	for _, name := range browser.GetInstalledBrowsers() {
		add(name, name, "Browser", "", "", browser.BrowserConfig{Name: name, AppType: "appName"})
		for _, p := range profilesFor(name) {
			add(p.ID, p.Name, name, "", "", browser.BrowserConfig{Name: name, AppType: "appName", Profile: p.Profile, ProfileDirectory: p.ProfileDirectory})
		}
	}
	return result
}

func stableFavorite(target browser.BrowserConfig, profiles []browser.BrowserProfile) browser.BrowserConfig {
	if target.ProfileDirectory != "" || target.Profile == "" {
		return target
	}
	matches := []browser.BrowserProfile{}
	for _, profile := range profiles {
		if profile.Profile == target.Profile || profile.Name == target.Profile {
			matches = append(matches, profile)
		}
	}
	if len(matches) == 1 {
		target.Profile = matches[0].Profile
		target.ProfileDirectory = matches[0].ProfileDirectory
	}
	return target
}

func saveAndApply(rf rules.RulesFile) error {
	if err := rules.Save(rf); err != nil {
		return err
	}
	if window.SaveRulesHandler != nil {
		if err := window.SaveRulesHandler(rf); err != nil {
			return &savedRulesApplyError{err}
		}
	}
	current, err := rules.Load()
	if err != nil {
		return &savedRulesApplyError{err}
	}
	window.SendMessageToWebView("rules", current)
	refreshMenuIcon(current)
	return nil
}

// The rules reached disk even if updating a runtime option subsequently failed.
// Focus restoration must remain available in that case.
type savedRulesApplyError struct{ error }

func (e *savedRulesApplyError) Unwrap() error { return e.error }

func focusDefaultPath() (string, error) {
	path, err := rules.GetPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "focus-default.json"), nil
}

func applyAndSaveDefaultRequest(rf rules.RulesFile, r urlrequest.Request, save func(rules.RulesFile) error) error {
	path, err := focusDefaultPath()
	if err != nil {
		return err
	}
	newSnapshot := false
	if r.Focus == "active" {
		_, statErr := os.Stat(path)
		if statErr != nil && !os.IsNotExist(statErr) {
			return statErr
		}
		newSnapshot = os.IsNotExist(statErr)
	}
	if err = applyDefaultRequest(&rf, r); err != nil {
		return err
	}
	saveErr := save(rf)
	var appliedErr *savedRulesApplyError
	persisted := saveErr == nil || errors.As(saveErr, &appliedErr)
	// Once restoration reached disk, its snapshot is consumed even if applying
	// a runtime option failed. A later Focus must capture the next user's default.
	if r.Focus == "clear" && persisted || newSnapshot && !persisted {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return errors.Join(saveErr, fmt.Errorf("could not clear Focus snapshot: %w", removeErr))
		}
	}
	return saveErr
}

// Focus filters persist the previous default so leaving a Focus restores it,
// including after the router has restarted. No account data is stored here.
func applyDefaultRequest(rf *rules.RulesFile, r urlrequest.Request) error {
	path, err := focusDefaultPath()
	if err != nil {
		return err
	}
	type defaults struct {
		Browser, Profile, Directory string
		Private                     bool
	}
	if r.Focus == "clear" {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		var previous defaults
		if err = json.Unmarshal(data, &previous); err != nil {
			return err
		}
		r.Browser, r.Profile, r.ProfileDirectory, r.Private = previous.Browser, previous.Profile, previous.Directory, previous.Private
	}
	proposed := *rf
	proposed.DefaultBrowser = r.Browser
	proposed.DefaultProfile = r.Profile
	proposed.DefaultProfileDirectory = r.ProfileDirectory
	proposed.DefaultPrivate = r.Private
	if err = rules.Validate(proposed); err != nil {
		return err
	}
	if proposed.DefaultBrowser != "" && proposed.DefaultBrowser != "prompt" {
		if err = browser.ValidateTarget(browser.BrowserConfig{Name: proposed.DefaultBrowser, AppType: appType(proposed.DefaultBrowser), Profile: proposed.DefaultProfile, ProfileDirectory: proposed.DefaultProfileDirectory, Private: proposed.DefaultPrivate}); err != nil {
			return err
		}
	}
	if r.Focus == "active" {
		if _, err := os.Stat(path); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return err
			}
			data, _ := json.Marshal(defaults{rf.DefaultBrowser, rf.DefaultProfile, rf.DefaultProfileDirectory, rf.DefaultPrivate})
			if err = os.WriteFile(path, data, 0600); err != nil {
				return err
			}
		}
	}
	*rf = proposed
	return nil
}

func setupFeatureBridge() {
	window.FeatureMessageHandler = func(msg map[string]interface{}) bool {
		if handleSyncMessage(msg) {
			return true
		}
		if handleLearningMessage(msg) {
			return true
		}
		kind, _ := msg["type"].(string)
		switch kind {
		case "grantBrowserAccess":
			result := browseraccess.Request()
			go func() { sendOutcome("browserAccessResult", <-result) }()
		case "getFeatures":
			rf, e := rules.Load()
			if e == nil {
				window.SendMessageToWebView("features", rf.Features)
			}
		case "saveFeatures":
			rf, e := rules.Load()
			if e == nil {
				data, _ := json.Marshal(msg["features"])
				e = json.Unmarshal(data, &rf.Features)
			}
			if e == nil {
				e = saveAndApply(rf)
			}
			sendOutcome("featuresSaved", e)
		case "getHistory":
			entries, e := history().List()
			if e != nil {
				window.SendMessageToWebView("history", []interface{}{})
			} else {
				window.SendMessageToWebView("history", entries)
			}
		case "clearHistory":
			sendOutcome("historyCleared", history().Clear())
		case "openPicker":
			raw, _ := msg["url"].(string)
			if raw == "" {
				raw = "https://example.com"
			}
			enqueueURL(raw, nil, false, true)
			sendOutcome("openPickerResult", nil)
		case "openClipboard":
			enqueueURL("finicky://clipboard?prompt=true", nil, false, false)
			sendOutcome("openClipboardResult", nil)
		case "exportSettings":
			rf, e := rules.Load()
			if e == nil {
				data, e := json.MarshalIndent(rf, "", "  ")
				if e == nil {
					window.SendMessageToWebView("settingsExport", map[string]string{"json": string(data)})
				}
			}
		case "importSettings":
			text, _ := msg["json"].(string)
			var rf rules.RulesFile
			e := json.Unmarshal([]byte(text), &rf)
			if e == nil {
				e = saveAndApply(rf)
			}
			sendOutcome("settingsImported", e)
		case "getIntegrations":
			exe, _ := os.Executable()
			window.SendMessageToWebView("integrations", map[string]string{"appPath": exe, "bundleId": "com.kevinlee.finicky", "scheme": "finicky-plus"})
		default:
			return false
		}
		return true
	}
}

func sendOutcome(kind string, err error) {
	result := map[string]interface{}{"success": err == nil}
	if err != nil {
		result["error"] = err.Error()
	}
	window.SendMessageToWebView(kind, result)
}
func readClipboard() string {
	p := C.ReadClipboard()
	if p == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(p))
	return C.GoString(p)
}
func copyText(value string) { p := C.CString(value); defer C.free(unsafe.Pointer(p)); C.CopyText(p) }
func shareURL(value string) { p := C.CString(value); defer C.free(unsafe.Pointer(p)); C.ShareURL(p) }

//export HandleSelectedText
func HandleSelectedText(text *C.char, prompt C.bool) {
	for _, link := range urlrequest.Extract(C.GoString(text)) {
		enqueueURL(link, nil, false, bool(prompt))
	}
}

//export GetMenuDestinations
func GetMenuDestinations() *C.char {
	rf, err := rules.Load()
	if err != nil {
		return nil
	}
	entries := []map[string]string{}
	// Menu opening happens on the main thread. Saved targets avoid synchronous
	// browser discovery or protected profile reads while the menu is tracking.
	for _, p := range rf.Profiles {
		if !p.Hidden {
			entries = append(entries, map[string]string{"id": p.ID, "name": p.Name})
		}
	}
	data, _ := json.Marshal(entries)
	return C.CString(string(data))
}

//export ChooseMenuDestination
func ChooseMenuDestination(identifier *C.char, launch C.bool) {
	id := C.GoString(identifier)
	go func() {
		rf, e := rules.Load()
		if e != nil {
			return
		}
		for _, p := range rf.Profiles {
			if p.ID == id {
				if bool(launch) {
					cfg := browser.BrowserConfig{Name: p.Browser, AppType: appType(p.Browser), Profile: p.Profile, ProfileDirectory: p.ProfileDirectory, Private: p.Private, URL: "about:blank"}
					if e = browser.LaunchBrowser(cfg, dryRun, false); e != nil {
						handleRuntimeError(e)
					}
				} else {
					rf.DefaultBrowser = p.Browser
					rf.DefaultProfile = p.Profile
					rf.DefaultProfileDirectory = p.ProfileDirectory
					rf.DefaultPrivate = p.Private
					if e = saveAndApply(rf); e != nil {
						handleRuntimeError(e)
					}
				}
				return
			}
		}
	}()
}

func refreshMenuIcon(rf rules.RulesFile) {
	style := "default"
	if rf.Features != nil && rf.Features.MenuIcon != "" {
		style = rf.Features.MenuIcon
	}
	a, b := C.CString(style), C.CString(rf.DefaultBrowser)
	defer C.free(unsafe.Pointer(a))
	defer C.free(unsafe.Pointer(b))
	C.SetMenuIcon(a, b)
}
