package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework CoreServices
#include <stdlib.h>
#include "main.h"
*/
import "C"

import (
	_ "embed"
	"finicky/browser"
	"finicky/browseraccess"
	"finicky/config"
	"finicky/logger"
	"finicky/resolver"
	"finicky/rules"
	"finicky/urlrequest"
	"finicky/util"
	"finicky/version"
	"finicky/window"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

//go:embed assets/finickyConfigAPI.js
var finickyConfigAPIJS []byte

type UpdateInfo struct {
	ReleaseInfo        *version.ReleaseInfo
	UpdateCheckEnabled bool
}

type URLInfo struct {
	ReceivedAt       time.Time
	LearningEpoch    uint64
	Request          urlrequest.Request
	Modifiers        map[string]bool
	URL              string
	Opener           *resolver.OpenerInfo
	OpenInBackground bool
}

type ConfigInfo struct {
	Handlers       int16
	Rewrites       int16
	DefaultBrowser string
	ConfigPath     string
}

var urlListener chan URLInfo = make(chan URLInfo, 256)
var windowClosed chan struct{} = make(chan struct{}, 1)
var vm *config.VM
var vmMu sync.Mutex

var forceWindowOpen bool = false
var queueWindowOpen chan bool = make(chan bool, 1)
var dryRun bool = false
var previewMode bool = false
var skipJSConfig bool = false
var configInfo *ConfigInfo
var shouldKeepRunning atomic.Bool

func main() {
	shouldKeepRunning.Store(true)
	startTime := time.Now()
	logger.SetWindowHandler(func(message string) { window.SendMessageToWebView("log", message) })
	logger.Setup()
	runtime.LockOSThread()

	// Define command line flags
	configPathPtr := flag.String("config", "", "Path to custom JS configuration file")
	rulesPathPtr := flag.String("rules", "", "Path to custom rules JSON file")
	noConfigPtr := flag.Bool("no-config", false, "Skip JS configuration file entirely")
	windowPtr := flag.Bool("window", false, "Force window to open")
	dryRunPtr := flag.Bool("dry-run", false, "Simulate without actually opening browsers")
	previewPtr := flag.Bool("preview", false, "Preview settings without claiming default-browser status, stopping other instances, or opening browsers")
	testURLPtr := flag.String("test-url", "", "Resolve a URL and print JSON without launching")
	promptPtr := flag.Bool("prompt", false, "Show the picker for positional URLs")
	flag.Parse()

	// Use the parsed values
	customConfigPath := *configPathPtr
	if customConfigPath != "" {
		slog.Debug("Using custom config path", "path", customConfigPath)
	}

	if *rulesPathPtr != "" {
		slog.Debug("Using custom rules path", "path", *rulesPathPtr)
		rules.SetCustomPath(*rulesPathPtr)
	}

	if *noConfigPtr {
		slog.Debug("Skipping JS config")
		skipJSConfig = true
	}

	if *windowPtr {
		forceWindowOpen = true
	}

	previewMode = *previewPtr
	if previewMode && *rulesPathPtr == "" {
		fmt.Fprintln(os.Stderr, "Preview mode requires an explicit --rules path for isolated settings")
		os.Exit(2)
	}
	dryRun = *dryRunPtr || previewMode
	C.SetPreviewMode(C.bool(previewMode))

	currentVersion := version.GetCurrentVersion()
	commitHash, buildDate := version.GetBuildInfo()
	slog.Info("Starting Waystile", "version", currentVersion)
	slog.Debug("Build info", "buildDate", buildDate, "commitHash", commitHash)

	go func() {
		if previewMode || dryRun || *testURLPtr != "" {
			slog.Debug("Preview mode: leaving the default browser unchanged")
			return
		}
		is_default_browser, err := setDefaultBrowser()
		if err != nil {
			slog.Debug("Failed checking if we are the default browser", "error", err)
		} else if !is_default_browser {
			slog.Debug("Waystile is not the default browser")
		} else {
			slog.Debug("Waystile is the default browser")
		}
	}()

	namespace := "finickyConfig"
	configChange := make(chan struct{}, 1)
	cfw, err := config.NewConfigFileWatcher(customConfigPath, namespace, configChange)

	if err != nil {
		handleFatalError(fmt.Sprintf("Failed to setup config file watcher: %v", err))
	}

	vm, err = setupVM(cfw, namespace)
	if err != nil {
		handleFatalError(err.Error())
	}

	slog.Debug("VM setup complete", "duration", fmt.Sprintf("%.2fms", float64(time.Since(startTime).Microseconds())/1000))

	if *testURLPtr != "" {
		result, resolveErr := resolver.ResolveURL(vm, *testURLPtr, nil, false)
		printTestResult(result, resolveErr)
		return
	}
	setupFeatureBridge()
	if accessErr := browseraccess.Restore(); accessErr != nil {
		slog.Warn("Browser profile access", "error", accessErr)
	}
	if rf, e := rules.Load(); e == nil {
		refreshMenuIcon(rf)
		if err := reconcileLearning(rf); err != nil {
			slog.Warn("Could not reconcile local learning data", "error", err)
		}
	}
	for _, raw := range flag.Args() {
		enqueueURL(raw, nil, false, *promptPtr)
	}
	go checkForUpdates()

	// Set up test URL handler
	window.TestUrlHandler = func(url string, opener *resolver.OpenerInfo) {
		go TestURLInternal(url, opener)
	}

	// Set up rules save handler.
	// When there is no JS config, rebuild the VM from the updated rules.
	// When there is a JS config, JSON rules are loaded fresh in evaluateURL — nothing to do.
	window.SaveRulesHandler = func(_ rules.RulesFile) error {
		return applyLatestSavedRules(func(rf rules.RulesFile) error {
			if err := reconcileLearning(rf); err != nil {
				// Rules have already been saved. A corrupt optional learning store
				// must not prevent the new routing settings from taking effect.
				slog.Warn("Could not reconcile local learning data", "error", err)
			}
			vmMu.Lock()
			defer vmMu.Unlock()
			configPath := ""
			if configInfo != nil {
				configPath = configInfo.ConfigPath
			}
			// Imports can remove overrides. Publish the resulting effective values,
			// including when a runtime side effect fails after the VM was updated.
			defer func() { publishConfiguration(vm, configPath) }()
			slog.Debug("Rules updated", "count", len(rf.Rules))
			resolver.SetCachedRules(rf)
			refreshMenuIcon(rf)
			if vm == nil || !vm.IsJSConfig() {
				if rf.DefaultBrowser == "" && len(rf.Rules) == 0 && rf.Options == nil && (rf.Features == nil || rf.Features.TransformScript == "") {
					vm = nil
					configPath = ""
					shouldKeepRunning.Store(vm.GetAllConfigOptions().KeepRunning)
					return logger.SetupFile(false)
				}
				script, err := rules.ToJSConfigScript(rf, namespace)
				if err != nil {
					slog.Error("Failed to generate config from rules", "error", err)
					return err
				}
				newVM, err := config.NewFromScript(finickyConfigAPIJS, namespace, script)
				if err != nil {
					slog.Error("Failed to rebuild VM from rules", "error", err)
					return err
				}
				vm = newVM
				configPath, _ = rules.GetPath()
				if vm != nil {
					opts := vm.GetAllConfigOptions()
					shouldKeepRunning.Store(opts.KeepRunning)
					if logErr := logger.SetupFile(opts.LogRequests && !dryRun); logErr != nil {
						return logErr
					}
					go checkForUpdates()
				}
			} else {
				if err := applyVisualOptions(vm, rf); err != nil {
					return err
				}
				opts := vm.GetAllConfigOptions()
				shouldKeepRunning.Store(opts.KeepRunning)
				if err := logger.SetupFile(opts.LogRequests && !dryRun); err != nil {
					return err
				}
			}
			return nil
		})
	}
	setupSettingsSync()

	const oneDay = 24 * time.Hour

	var showingWindow bool = false
	timeoutChan := time.After(1 * time.Second)
	updateChan := time.After(oneDay)

	if vm != nil {
		shouldKeepRunning.Store(vm.GetAllConfigOptions().KeepRunning)
	}
	if shouldKeepRunning.Load() {
		timeoutChan = nil
	}

	go func() {
		slog.Info("Listening for events...")
		for {
			select {
			case urlInfo := <-urlListener:
				startTime := time.Now()

				routeRequest(urlInfo)

				slog.Debug("Time taken evaluating URL and opening browser", "duration", fmt.Sprintf("%.2fms", float64(time.Since(startTime).Microseconds())/1000))

				if !showingWindow && !shouldKeepRunning.Load() {
					timeoutChan = time.After(2 * time.Second)
				} else {
					timeoutChan = nil
				}

			case <-configChange:
				startTime := time.Now()
				var setupErr error
				slog.Debug("Config has changed")
				settingsApplyMu.Lock()
				vmMu.Lock()
				vm, setupErr = setupVM(cfw, namespace)
				vmMu.Unlock()
				settingsApplyMu.Unlock()
				if setupErr != nil {
					handleRuntimeError(setupErr)
				} else {
					C.SetStatusItemError(false)
				}
				slog.Debug("VM refresh complete", "duration", fmt.Sprintf("%.2fms", float64(time.Since(startTime).Microseconds())/1000))
				vmMu.Lock()
				shouldKeepRunning.Store(vm.GetAllConfigOptions().KeepRunning)
				vmMu.Unlock()
				go checkForUpdates()

			case shouldShowWindow := <-queueWindowOpen:
				if !showingWindow && shouldShowWindow {
					go ShowConfigWindow()
					showingWindow = true
					timeoutChan = nil
				}

			case <-updateChan:
				go checkForUpdates()
				updateChan = time.After(oneDay)

			case <-windowClosed:
				showingWindow = false
				if !shouldKeepRunning.Load() {
					slog.Info("Exiting due to window closed")
					tearDown()
				} else {
					slog.Debug("Window closed")
				}

			case <-timeoutChan:
				slog.Info("Exiting due to timeout")
				tearDown()
			}
		}
	}()

	vmMu.Lock()
	shouldHideIcon := false
	if vm != nil {
		shouldHideIcon = vm.GetAllConfigOptions().HideIcon
	}
	vmMu.Unlock()
	C.RunApp(C.bool(forceWindowOpen), C.bool(!shouldHideIcon), C.bool(shouldKeepRunning.Load()))
}

func handleRuntimeError(err error) {
	slog.Error("Failed evaluating url", "error", err)
	C.SetStatusItemError(true)
}

//export HandleURL
func HandleURL(url *C.char, name *C.char, bundleId *C.char, path *C.char, windowTitle *C.char, openInBackground C.bool) {
	var opener resolver.OpenerInfo

	if name != nil && bundleId != nil && path != nil {
		opener = resolver.OpenerInfo{
			Name:     C.GoString(name),
			BundleID: C.GoString(bundleId),
			Path:     C.GoString(path),
		}
		if windowTitle != nil {
			opener.WindowTitle = C.GoString(windowTitle)
		}
	}

	enqueueURL(C.GoString(url), &opener, bool(openInBackground), false)
}

//export TestURL
func TestURL(url *C.char) {
	urlString := C.GoString(url)
	TestURLInternal(urlString, nil)
}

func TestURLInternal(urlString string, opener *resolver.OpenerInfo) {
	slog.Debug("Testing URL", "url", urlString)

	vmMu.Lock()
	config, err := resolver.ResolveURL(vm, urlString, opener, false)
	vmMu.Unlock()
	if err == nil && config.Action != "prompt" {
		err = browser.ValidateTarget(*config)
	}
	if err != nil {
		slog.Error("Failed to evaluate URL", "error", err)
		window.SendMessageToWebView("testUrlResult", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	window.SendMessageToWebView("testUrlResult", map[string]interface{}{
		"url":              config.URL,
		"action":           config.Action,
		"matchedRule":      config.MatchedRule,
		"browser":          config.Name,
		"openInBackground": config.OpenInBackground,
		"profile":          config.Profile,
		"profileDirectory": config.ProfileDirectory,
		"private":          config.Private,
		"newWindow":        config.NewWindow,
		"args":             config.Args,
	})
}

func handleFatalError(errorMessage string) {
	slog.Error("Fatal error", "msg", errorMessage)
	C.SetStatusItemError(true)
}

//export QueueWindowDisplay
func QueueWindowDisplay(openWindow int32) {
	select {
	case queueWindowOpen <- openWindow != 0:
	default:
	}
}

//export ShowConfigWindow
func ShowConfigWindow() {
	slog.Debug("Showing window")
	window.ShowWindow()

	// Send version information
	currentVersion := version.GetCurrentVersion()
	window.SendMessageToWebView("version", currentVersion)

}

//export WindowDidClose
func WindowDidClose() {
	select {
	case windowClosed <- struct{}{}:
	default:
	}
}

//export GetCurrentConfigPath
func GetCurrentConfigPath() *C.char {
	vmMu.Lock()
	defer vmMu.Unlock()
	if configInfo != nil && configInfo.ConfigPath != "" {
		cPath := C.CString(configInfo.ConfigPath)
		return cPath
	} else {
		return nil
	}
}

func checkForUpdates() {
	if previewMode {
		return
	}
	vmMu.Lock()
	var runtime *goja.Runtime
	if vm != nil {
		runtime = vm.Runtime()
	}

	releaseInfo, updateCheckEnabled, err := version.CheckForUpdatesIfEnabled(runtime)
	vmMu.Unlock()
	if err != nil {
		slog.Error("Error checking for updates", "error", err)
	}

	updateInfo := UpdateInfo{
		ReleaseInfo:        releaseInfo,
		UpdateCheckEnabled: updateCheckEnabled,
	}

	if updateInfo.ReleaseInfo != nil && updateInfo.ReleaseInfo.HasUpdate {
		slog.Info("New version is available", "version", updateInfo.ReleaseInfo.LatestVersion)
	}

	if updateInfo.ReleaseInfo != nil {
		window.SendMessageToWebView("updateInfo", map[string]interface{}{
			"version":            updateInfo.ReleaseInfo.LatestVersion,
			"hasUpdate":          updateInfo.ReleaseInfo.HasUpdate,
			"updateCheckEnabled": updateInfo.UpdateCheckEnabled,
			"downloadUrl":        updateInfo.ReleaseInfo.DownloadUrl,
			"releaseUrl":         updateInfo.ReleaseInfo.ReleaseUrl,
		})
	} else {
		window.SendMessageToWebView("updateInfo", map[string]interface{}{
			"version":            "",
			"hasUpdate":          false,
			"updateCheckEnabled": updateInfo.UpdateCheckEnabled,
			"downloadUrl":        "",
			"releaseUrl":         "",
		})
	}
}

func tearDown() {
	checkForUpdates()
	slog.Info("Exiting...")
	os.Exit(0)
}

func setupVM(cfw *config.ConfigFileWatcher, namespace string) (*config.VM, error) {
	logRequests := false
	var err error

	defer func() {
		err = logger.SetupFile(logRequests && !dryRun)
		if err != nil {
			slog.Warn("Failed to setup file logging", "error", err)
		}
	}()

	var currentBundlePath, configPath string
	if !skipJSConfig {
		var err2 error
		currentBundlePath, configPath, err2 = cfw.BundleConfig()
		if err2 != nil {
			return nil, fmt.Errorf("failed to read config: %v", err2)
		}
	}

	// Always seed the cached rules from disk so JSON rules are applied
	// immediately on startup, even when a JS config is also present.
	if rf, rulesErr := rules.Load(); rulesErr != nil {
		slog.Warn("Failed to pre-load rules cache", "error", rulesErr)
	} else {
		resolver.SetCachedRules(rf)
	}

	var newVM *config.VM

	if currentBundlePath != "" {
		newVM, err = config.New(finickyConfigAPIJS, namespace, currentBundlePath)
		if err != nil {
			return nil, fmt.Errorf("failed to setup VM: %v", err)
		}
	} else {
		rf, rulesErr := rules.Load()
		if rulesErr != nil {
			slog.Warn("Failed to load rules file", "error", rulesErr)
		} else {
			resolver.SetCachedRules(rf)
			if rf.DefaultBrowser != "" || len(rf.Rules) > 0 || rf.Options != nil || rf.Features != nil && rf.Features.TransformScript != "" {
				script, scriptErr := rules.ToJSConfigScript(rf, namespace)
				if scriptErr != nil {
					return nil, fmt.Errorf("failed to generate config from rules: %v", scriptErr)
				}
				newVM, err = config.NewFromScript(finickyConfigAPIJS, namespace, script)
				if err != nil {
					return nil, fmt.Errorf("failed to setup VM from rules: %v", err)
				}
				configPath, _ = rules.GetPath()
			}
		}
	}

	if newVM == nil {
		publishConfiguration(nil, "")
		return nil, nil
	}
	if rf, loadErr := rules.Load(); loadErr == nil {
		if err = applyVisualOptions(newVM, rf); err != nil {
			return nil, err
		}
	}

	opts := newVM.GetAllConfigOptions()
	logRequests = opts.LogRequests
	publishConfiguration(newVM, configPath)

	return newVM, nil
}

// configurationSnapshot keeps the complete configuration message together so
// refreshing effective options cannot erase the source path or routing counts.
// Callers must serialize access to target's runtime.
func configurationSnapshot(target *config.VM, path string) (*ConfigInfo, map[string]interface{}) {
	info := &ConfigInfo{DefaultBrowser: "Safari", ConfigPath: path}
	isJSConfig := false
	if target != nil {
		isJSConfig = target.IsJSConfig()
		if state := target.GetConfigState(); state != nil {
			info.Handlers = state.Handlers
			info.Rewrites = state.Rewrites
			info.DefaultBrowser = state.DefaultBrowser
		}
	}
	opts := target.GetAllConfigOptions()
	return info, map[string]interface{}{
		"handlers":       info.Handlers,
		"rewrites":       info.Rewrites,
		"defaultBrowser": info.DefaultBrowser,
		"configPath":     util.ShortenPath(info.ConfigPath),
		"isJSConfig":     isJSConfig,
		"options": map[string]interface{}{
			"keepRunning":     opts.KeepRunning,
			"hideIcon":        opts.HideIcon,
			"logRequests":     opts.LogRequests,
			"checkForUpdates": opts.CheckForUpdates,
		},
	}
}

func publishConfiguration(target *config.VM, path string) {
	info, message := configurationSnapshot(target, path)
	configInfo = info
	window.SendMessageToWebView("config", message)
}

func applyVisualOptions(target *config.VM, rf rules.RulesFile) error {
	if target == nil {
		return nil
	}
	overrides := make(map[string]interface{})
	if rf.Options != nil {
		if rf.Options.KeepRunning != nil {
			overrides["keepRunning"] = *rf.Options.KeepRunning
		}
		if rf.Options.HideIcon != nil {
			overrides["hideIcon"] = *rf.Options.HideIcon
		}
		if rf.Options.LogRequests != nil {
			overrides["logRequests"] = *rf.Options.LogRequests
		}
		if rf.Options.CheckForUpdates != nil {
			overrides["checkForUpdates"] = *rf.Options.CheckForUpdates
		}
	}
	return target.ApplyOptionOverrides(overrides)
}
