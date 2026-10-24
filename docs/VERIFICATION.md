# Build and runtime verification

Updated October 24, 2026. Finicky Plus build `d6ae002` and Finicky Actions are installed locally. The installed main-app binary matches the final build, and its code signature verifies. This is a development build, not an App Store release or a certified replacement for Velja. Automated checks, live acceptance, and unverified features are recorded separately below.

## Automated checks

- Config API: type checking and all 87 Vitest tests passed in the QA baseline, including pre-match and per-rule transformations, prompt routing, private targets, and profile directories.
- Settings UI: Svelte/type checking and all 23 Node tests passed after the QA corrections. Coverage includes settings normalization, private favorites, alternate destinations, and routing-test labels. The final production build also passed.
- Native router: the full `go test -race -count=1 ./...` run passed on final build `d6ae002`. Coverage includes preserving required picker launch modes, alternate-modifier privacy, and transactional Focus restoration/cleanup.
- Clipboard URL extraction: `go test -race -count=1 ./urlrequest` passed after reproducing and fixing the loss of balanced trailing parentheses and brackets. Cases include Wikipedia-style paths, query delimiters, IPv6 hosts, prose wrappers, deduplication, and unchanged direct-URL parsing.
- Browser add-on: the Node test passed for encoded toolbar/context-menu requests and unsupported URL schemes.
- Native companion: the universal macOS build, deep signature verification, and five XCTest tests passed. Tests cover encoded protocol values, shared URL extraction, web-only opening, and private/new-window/background options. These are build/unit results, not successful App Intents execution.
- Native sharing correction: Objective-C syntax checks passed for both the main-app integration and ARC picker. Both corrected sharing entry points also passed the live presentation checks below.

## Installed-app acceptance

The following checks used the installed app and real Chrome profiles on this Mac:

| Check | Result |
| --- | --- |
| Default routing registration | `http`, `https`, and `finicky-plus` were reverified to resolve to `/Applications/Finicky Plus.app` after installing `d6ae002`. The old Finicky app was moved to Trash. |
| Installed routing smoke test | The installed app's `--test-url` output confirmed commit `d6ae002`, the Personal fallback, and removal of `utm` tracking parameters. |
| Chrome add-on installation | The local add-on is installed in Personal, Work, and Academic. |
| Academic → Work | Option-Shift-click followed by the Work shortcut (`W`) opened the link in Work. |
| Work → Personal | The toolbar action followed by the Personal shortcut (`P`) opened the page in Personal. |
| Personal → Academic | The context-menu action followed by the Academic shortcut (`A`) opened the link in Academic. |
| Tracking cleanup | Tracking parameters were stripped in the cross-profile checks. |
| Queued picker cancellation | Two queued requests could each be cancelled without leaving an extra picker or launching a destination. |
| Required private/new-window modes | Final installed build `d6ae002` showed the required options checked and disabled. Selecting Personal (`P`) opened the final smoke-test URL in a real Chrome Incognito window; the test window was then closed. |
| Picker Share | Installed build `b33ae05` opened the system sharing popover while retaining the destination picker. Cancelling sharing returned to that same picker. |
| Standalone Share | Installed build `b33ae05` showed its Share window; pressing Share opened the system popover. Cancellation and closing the window worked. No external sharing service was invoked. |
| Custom alternate destination | Changing the alternate from Work to Custom kept the custom controls open. Discard restored the saved setting. |
| Remember website | Remembering Work created a temporary website rule visible in settings. A subsequent external URL opened in Work. Existing JavaScript Work handlers could also match, so this does not isolate which rule performed the routing. The temporary rule was deleted and the change applied. |
| Settings restoration | After QA, the original settings file was restored byte for byte. |
| Browser cleanup | All tabs opened for QA were closed, and the original Academic notebook was restored. |

These results establish the three configured Chrome routes and extension entry points. They do not establish browser behavior for every supported derivative, profile login state, or every native-service adapter.

## QA corrections and coverage

QA reproduced a privacy regression in the previous installed binary: a private request could lose its required mode when a destination was selected in the picker. The source now carries required modes into the picker and the selected destination, with regression tests. Final installed build `d6ae002` passed the Incognito recheck recorded above.

The QA pass also corrected custom alternate-destination selection, private-favorite matching, routing-test profile labels, stale opening-option overlays, transactional Focus-default restoration, and balanced clipboard URL extraction. Their automated checks do not substitute for live acceptance of each flow. Clipboard extraction has automated coverage only.

The previous picker Share action dismissed its anchor window before presenting AppKit's sharing menu. The corrected picker keeps its panel visible and presents sharing from the button's mouse-down event; the standalone share request provides a retained panel with a Share button. Native syntax checks and live presentation/cancellation passed. Actual delivery through a sharing service was not tested.

Follow-up review found three further cases: required privacy must survive a held alternate-destination modifier, a persisted Focus clear must clean up its restoration snapshot, and the settings UI must refresh effective opening options. The corrections are regression-tested and included in installed build `d6ae002`. Not every edge case was separately exercised live; the earlier installed-build results establish only the flows recorded above.

## Earlier native preview acceptance

The separate preview identity was launched with illustrative Personal, Work, and Academic destinations and an isolated rules file. Browser launching is simulated in this mode.

- Tracking cleanup was visible in the picker URL.
- The right-click destination menu exposed opening, background, new window, private, and remembered-website actions.
- Remembering Work created the expected domain rule in the isolated file and the settings immediately displayed it.
- The settings rendered in the native WebKit window with profiles, preferences, history, and integrations navigation.
- Pressing the Work shortcut dismissed the picker and produced a simulated Chrome launch with `--profile-directory=Profile 2` and the cleaned URL.

The preview uncovered a Cocoa startup issue that reinterpreted positional URL arguments as local document paths. The native file-event handler now ignores already-processed command-line arguments. Repeating the native preview test confirmed a single picker and a single simulated browser launch.

## Runtime limits

Chromium profile metadata on this Mac requires user-granted folder access; the installed routes use explicit profile-directory targets without reading account metadata. A folder chooser was implemented, but the access-grant flow was not exercised.

| Integration | Current runtime status |
| --- | --- |
| Native Shortcuts actions | All five actions appeared in the Shortcuts catalog. Running the Settings action failed with `LNConnectionErrorDomain` error 1200. Missing Team ID/code-identity diagnostics persisted after duplicate app registrations were removed and the installed companion was restarted. A build with a real signing identity is needed to test that diagnosis; successful native action execution is not established. |
| Safari extension | Activation is blocked for the current ad hoc build. No unsigned-extension protection was disabled. |
| Inbound Share extension | Enabled in macOS, but a successful inbound share has not been verified. This is separate from the main app's outbound Share button. |
| Focus filter | OS delivery of activation/deactivation has not been verified. Router restoration logic has regression coverage. |
| Handoff | Cross-device delivery has not been verified. |
| Firefox extension and other browsers | Source/build coverage only; the live extension checks above used Chrome. |
| Chromium PWAs and native-service adapters | Fixture/unit coverage; no live installed PWA, authenticated service handoff, or meeting join was verified. |

Shortcuts actions report dispatch to macOS, not an acknowledgement from the destination browser. The documented custom-scheme fallback does not establish that the companion's App Intents or Focus extension work.

There is no developer signing identity on this build machine. The installed app signatures verify, but their ad hoc signatures do not establish a persistent developer identity, notarization, or successful native extension execution. The fork has no automatic update feed. See [Finicky Actions setup](../apps/integrations/README.md) for the signing limitation and the next acceptance checks.

See [the capability matrix](PARITY.md) for feature coverage and service-specific limits.
