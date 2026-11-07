package resolver

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"finicky/browser"
	"finicky/config"
	"finicky/linkfeatures"
	"finicky/rules"
)

// OpenerInfo describes the process that triggered the URL open.
type OpenerInfo struct {
	Name        string `json:"name"`
	BundleID    string `json:"bundleId"`
	Path        string `json:"path"`
	WindowTitle string `json:"windowTitle,omitempty"`
}

var (
	evaluationMu    sync.Mutex
	cachedRulesMu   sync.Mutex
	cachedRulesFile rules.RulesFile
)

// SetCachedRules stores a snapshot of the JSON rules so evaluateURL can use
// them without hitting disk on every URL open.
func SetCachedRules(rf rules.RulesFile) {
	cachedRulesMu.Lock()
	cachedRulesFile = rf
	cachedRulesMu.Unlock()
}

func getCachedRules() rules.RulesFile {
	cachedRulesMu.Lock()
	defer cachedRulesMu.Unlock()
	return cachedRulesFile
}

// ResolveURL determines which browser to use for the given URL.
//
// vm may be nil (no configuration at all). Whether to merge JSON rules is
// derived from vm.IsJSConfig().
//
// Always returns a non-nil config. Returns a non-nil error only when JS
// evaluation failed.
func ResolveURL(vm *config.VM, urlStr string, opener *OpenerInfo, openInBackground bool) (*browser.BrowserConfig, error) {
	evaluationMu.Lock()
	defer evaluationMu.Unlock()
	if vm != nil {
		cfg, err := evaluateURL(vm, urlStr, opener)
		if err != nil {
			return defaultBrowserConfig(urlStr, openInBackground), err
		}
		// A saved default is an explicit UI/Shortcuts choice. JS handlers keep
		// priority, while the saved fallback takes effect on unmatched links.
		rf := getCachedRules()
		if vm.IsJSConfig() && cfg.MatchedRule == nil && rf.DefaultBrowser != "" {
			originalURL := cfg.URL
			cfg = &browser.BrowserConfig{Name: rf.DefaultBrowser, Profile: rf.DefaultProfile, ProfileDirectory: rf.DefaultProfileDirectory, Private: rf.DefaultPrivate, URL: originalURL, AppType: "appName"}
			if strings.Contains(rf.DefaultBrowser, ".") && !strings.Contains(rf.DefaultBrowser, " ") {
				cfg.AppType = "bundleId"
			}
			if strings.HasPrefix(rf.DefaultBrowser, "/") || strings.HasPrefix(rf.DefaultBrowser, "~/") {
				cfg.AppType = "path"
			}
			if rf.DefaultBrowser == "prompt" {
				cfg.Action = "prompt"
				cfg.AppType = "none"
				cfg.Name = ""
			}
		}
		cfg.OpenInBackground = mergeBackground(cfg.OpenInBackground, openInBackground)
		return cfg, nil
	}
	cleaned, err := linkfeatures.PrepareURL(urlStr, getCachedRules().Features)
	if err != nil {
		slog.Warn("Link processing", "error", err)
	}
	return defaultBrowserConfig(cleaned, openInBackground), nil
}

func mergeBackground(fromConfig *bool, requested bool) *bool {
	if fromConfig != nil {
		return fromConfig
	}
	return &requested
}

func evaluateURL(vm *config.VM, url string, opener *OpenerInfo) (*browser.BrowserConfig, error) {
	runtime := vm.Runtime()

	resolvedURL, err := linkfeatures.PrepareURL(url, getCachedRules().Features)
	runtime.Set("originalUrl", url)
	if err != nil {
		slog.Info("Failed to resolve short URL", "error", err, "url", url, "using", resolvedURL)
	}
	url = resolvedURL
	runtime.Set("url", resolvedURL)

	if opener != nil {
		openerMap := map[string]interface{}{
			"name":     opener.Name,
			"bundleId": opener.BundleID,
			"path":     opener.Path,
		}
		if opener.WindowTitle != "" {
			openerMap["windowTitle"] = opener.WindowTitle
		}
		runtime.Set("opener", openerMap)
		slog.Debug("Setting opener", "name", opener.Name, "bundleId", opener.BundleID, "path", opener.Path, "windowTitle", opener.WindowTitle)
	} else {
		runtime.Set("opener", nil)
		slog.Debug("No opener detected")
	}

	// When there is a JS config, append cached JSON rules as lower-priority handlers.
	var evalScript string
	if vm.IsJSConfig() {
		rf := getCachedRules()
		runtime.Set("_jsonHandlers", rules.ToJSHandlers(rf.Rules))
		transformScript := ""
		if rf.Features != nil {
			transformScript = rf.Features.TransformScript
		}
		runtime.Set("_jsonTransformScript", transformScript)
		evalScript = `finickyConfigAPI.openUrl(url, opener, originalUrl, Object.assign({}, finalConfig, {
			options: Object.assign({}, finalConfig.options || {}, _jsonTransformScript ? { transformScript: _jsonTransformScript } : {}),
			handlers: (finalConfig.handlers || []).concat(_jsonHandlers)
		}))`
	} else {
		evalScript = "finickyConfigAPI.openUrl(url, opener, originalUrl, finalConfig)"
	}

	// A mistaken loop in an editable transform must not wedge the router.
	timeoutDone := make(chan struct{})
	timer := time.AfterFunc(2*time.Second, func() {
		runtime.Interrupt("URL evaluation exceeded two seconds")
		close(timeoutDone)
	})
	defer func() {
		if !timer.Stop() {
			<-timeoutDone
		}
		runtime.ClearInterrupt()
	}()
	openResult, err := runtime.RunString(evalScript)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate URL in config: %v", err)
	}

	resultJSON := openResult.ToObject(runtime).Export()
	resultBytes, err := json.Marshal(resultJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to process browser configuration: %v", err)
	}

	var browserResult browser.BrowserResult
	if err := json.Unmarshal(resultBytes, &browserResult); err != nil {
		return nil, fmt.Errorf("failed to parse browser configuration: %v", err)
	}
	for _, warning := range browserResult.Warnings {
		slog.Warn(warning, "url", url)
	}

	slog.Debug("Final browser options",
		"name", browserResult.Browser.Name,
		"openInBackground", browserResult.Browser.OpenInBackground,
		"profile", browserResult.Browser.Profile,
		"args", browserResult.Browser.Args,
		"appType", browserResult.Browser.AppType,
	)

	var resultErr error
	if browserResult.Error != "" {
		resultErr = fmt.Errorf("%s", browserResult.Error)
	}
	return &browserResult.Browser, resultErr
}

func defaultBrowserConfig(urlStr string, openInBackground bool) *browser.BrowserConfig {
	bg := openInBackground
	return &browser.BrowserConfig{
		Name:             "com.apple.Safari",
		AppType:          "bundleId",
		OpenInBackground: &bg,
		Args:             []string{},
		URL:              urlStr,
	}
}
