# Build and runtime verification

Updated October 25, 2026. The installed main app is signed universal build `1e13dd4`, with the UI foundation described in [UI.md](UI.md), the iCloud settings sync described in [SYNC.md](SYNC.md) and the local shadow-learning baseline described in [LEARNING.md](LEARNING.md). Sync and learning are enabled on this Mac at the user's request. Finicky Actions build 5 is unchanged. This remains a development build; the results below distinguish current checks from earlier native-integration acceptance.

## October 25 UI foundation update

- Claude Opus 5.5 (`claude-opus-5-5-medium`) and Gemini 4 Argon (`gemini-4-argon-high`) completed independent Cursor UI reviews and a Claude reconciliation. All three stages report success. This was a text-only review of source and written visual observations; neither model received image pixels or operated the app. The local report is retained at `~/.local/state/cross-review/ui-production-20261025/review/report.md`.
- The settings window now uses bundled Radix Colors 3.0.0 and Lucide Svelte 1.61.0, shared semantic tokens, readable type, compact routing summaries, and consistent controls. Native selects and dialogs remain. Complete upstream license notices are in the app bundle.
- Final Svelte/TypeScript checks passed with zero errors and the existing inactive `Rules.svelte` warning. All **51** settings model tests passed. Production UI and universal native builds passed. The two-architecture binary and its Apple Development signature were verified, including the installed copy.
- Browser preview checks covered light/dark appearance, the 760 × 560 minimum layout, routing, profiles, preferences, learning, sync, history, integrations, diagnostics, and About. Rule reordering retained focus and announced position; the named rule dialog focused its first field and restored focus after Escape. Preferences Apply, mapping navigation protection, mapping Save/Discard, and Chrome-to-Firefox manual mapping save were exercised with synthetic data. The sync conflict fixture was inspected visually. The temporary review harness was removed before the release build.
- Installed-app checks verified the new Routing, Sync, Learning, and Test a link screens. The native tester resolved the default example link to Personal / Chrome Default without opening it. The Work apps rule and all three saved destinations were visibly intact. Sync showed **On** and **Up to date**, with the correct local mappings and no global Apply footer; learning showed **On**, observe-only behavior, and an empty store.
- The real rules file, sync state, learning store, and sole cloud revision matched their pre-update SHA-256 hashes. Read-only LaunchServices checks still resolve HTTP, HTTPS, and `finicky-plus` to `/Applications/Finicky Plus.app`. No extension permission changes or new cloud revisions were introduced by this QA.
- Updated `iCloud Drive/Finicky Plus Setup/Finicky Plus Setup.zip` with the signed universal UI build. ZIP integrity and the staged app signature passed; the cloud copy matched the local archive. Apple's resource metadata reports the ZIP and guide uploaded. The archive is 32,022,849 bytes; SHA-256 is `8d7cbe24a0e66d55b4f169dcc69884f3ab2137560b2b2b0457a6c2b00a52377a`.

Installed screenshots, state hashes, the previous app and package, and refreshed setup staging are retained locally under `apps/finicky/build/qa-ui-20261025/`. The final routing screenshot is `installed-routing.png`; compact dark preview and installed sync screenshots are also retained. The other Mac was not available for acceptance testing. A complete VoiceOver walkthrough, the oldest supported WebKit version, and the unchanged companion integrations were not retested for this UI update. This is still an Apple Development build, not a notarized public release.

## October 25 iCloud sync build

- The full native race suite passed after the final sync recovery changes, including 26 settings-sync tests, ten application sync tests, seven configuration-fallback tests, and conditional-save coverage. The browser package race tests passed after the later native boolean correction. The final universal build compiled for both Intel and Apple silicon; the Intel build exposed and verified fixes to two Objective-C boolean boundaries.
- Settings checks and all 48 tests passed, with only the existing `Rules.svelte` initial-value warning. Config API type checking and all 87 tests passed. UI coverage includes explicit profile mappings, revision selection, independent sync state, browser-identity normalization, and preservation of unapplied routing edits.
- The installed app reports commit `a41d163`; its Apple Development signature passes strict, deep verification, and its executable contains `x86_64` and `arm64`. Read-only LaunchServices checks resolve HTTP, HTTPS, and `finicky-plus` to `/Applications/Finicky Plus.app`. This verifies development signing, not notarization or execution on a second Mac.
- The existing JavaScript rule was migrated to the visual `Work apps` rule, retaining Work routing for Slack, ChatGPT, Claude, and Cursor, and the Personal fallback. The original script was preserved as `~/.finicky.js.before-icloud-sync-20261025`. Routing probes confirmed those destinations and tracking cleanup. Startup without a JavaScript file now uses the visual settings; invalid or unreadable configuration still reports an error.
- Sync was enabled in the installed native settings window. Its clean state showed **On**, **Up to date**, and explicit Work / Personal / Academic mappings to this Mac's Chrome folders. Apply remained disabled, and the enabled state survived a restart. The final screenshot is retained locally at `apps/finicky/build/qa-sync-20261025/sync-enabled.png`.
- The first immutable settings revision was written to `iCloud Drive/Finicky Plus Sync/revisions/`. Its contents were inspected: logical profile IDs and the Work rule are portable; Chrome folder names and learning/history preferences are absent. Apple's file resource metadata reports the revision uploaded. This is evidence of upload from this Mac, not receipt or successful routing on the other Mac.
- An isolated second-device simulation read that real shared revision, required explicit mappings, and materialized the expected settings using different Chrome folder names. It wrote only temporary local state, added no shared revision, and was disabled after the check. The actual second Mac remains untested.
- Learning remains enabled with an empty local store; sync QA created no learning observations. Chrome extension 0.4.0 and the signed native companion are unchanged. Existing extension permissions were not expanded. Earlier native-integration acceptance below was not rerun for this release.

Rollback app/configuration copies and the setup-package staging files are retained locally under `apps/finicky/build/qa-sync-20261025/`. The other Mac requires one-time installation and explicit profile mapping; see [SECOND_MAC.md](SECOND_MAC.md). The setup package includes the universal router and Chrome extension, while native companion integrations and per-app permissions require their own setup on that Mac.

## October 25 learning build

- Full native `go test -race -count=1 ./...` passed, including 19 learning-package tests and six application-integration groups. The main-package race suite was rerun after the final hidden-duplicate exclusion. Temporal cutoffs, future-label invariance, corrections, profile lifecycle deletion, storage retention, privacy exclusions, and stable identities are covered.
- Config API type checks and all 87 tests passed. Settings checks and all 36 tests passed; the only Svelte warning is the pre-existing initial-value warning in `Rules.svelte`. Both production web builds and the arm64 application build passed. All ten browser-extension tests passed.
- The installed app's Apple Development signature verifies with `codesign --verify --deep --strict`. Its reported binary commit is `d50b393`. Read-only LaunchServices checks resolve HTTP, HTTPS, and `finicky-plus` to `/Applications/Finicky Plus.app`; a routing probe preserved the Personal fallback and tracking cleanup.
- Chrome extension 0.4.0 was reloaded and its version/enabled state verified in Personal, Work, and Academic. No extension permissions were expanded. Existing open webpages may need reloading to receive the changed Option-click content script.
- Live Chrome toolbar → native chooser → explicit Personal selection created exactly one sanitized `example.com` request and a neutral confirmation. The Learning panel displayed the choice and correctly abstained for insufficient history. Correcting it to Work created version two with explicit-correction provenance without reopening the page.
- A second live chooser action selected a private Personal destination and opened a Chrome Incognito window. It added no learning request or feedback. No Incognito extension access was enabled; source-private handling has automated coverage, not a separate live private-source extension test.
- The learning store contained neither the original URL nor the QA query; its file and directory modes were 0600 and 0700. The exact QA request and its corrections were removed through the store deletion API while the router was stopped. The final store is empty and learning remains on. Other settings are semantically unchanged; normal native serialization omitted false values and emitted null for the inherited service list.
- Native UI checks caught and fixed false settings-conflict and dirty-state banners caused by metadata omissions, null fields, and JSON property order in save messages. The isolated preview preserved unapplied routing edits during a learning toggle. The final installed build passed clean-state off/on toggles with Apply remaining disabled and no conflict banner.
- A synthetic 2,000-request/2,000-confirmation, 2.63 MB store measured a median 46 ms for observation plus confirmation after browser launch (11 trials), and 661 ms for evaluation (five trials). Evaluation runs on settings refresh, not the link-opening path. These are local measurements, not a latency guarantee or routing-quality result.

Predictions never choose or preselect a destination. There is no GNN or automatic learned routing in this build, and no real-user accuracy claim. Browser privacy is reported by the integration, not authenticated by the custom URL scheme. Rollback copies of the previous app, extension, and settings are retained locally under `apps/finicky/build/qa-learning-20261025/`.

## October 24 native-integration baseline

The prior QA used the `d6ae002` Go binary. Its app bundle received the missing Services context dictionaries and was re-signed. Finicky Actions build 5 included Share presentation metadata, a bounded activation predicate, and rich/plain-text loader corrections. The companion and its three embedded extensions verify under the same Apple signing team. These earlier integration results were not all repeated for the learning update.

## Automated checks

- Config API: type checking and all 87 Vitest tests passed in the QA baseline, including pre-match and per-rule transformations, prompt routing, private targets, and profile directories.
- Settings UI: Svelte/type checking and all 23 Node tests passed after the QA corrections. Coverage includes settings normalization, private favorites, alternate destinations, and routing-test labels. The final production build also passed.
- Native router: the full `go test -race -count=1 ./...` run passed on final build `d6ae002`. Coverage includes preserving required picker launch modes, alternate-modifier privacy, and transactional Focus restoration/cleanup.
- Clipboard URL extraction: `go test -race -count=1 ./urlrequest` passed after reproducing and fixing the loss of balanced trailing parentheses and brackets. Cases include Wikipedia-style paths, query delimiters, IPv6 hosts, prose wrappers, deduplication, and unchanged direct-URL parsing.
- Browser add-on: the Node test passed for encoded toolbar/context-menu requests and unsupported URL schemes.
- Native companion: signed build 5 and all 16 XCTest tests passed. Coverage includes encoded protocol values, web-only opening, opening options, Share presentation metadata, actual attachment-free `contentText` matching, bounded activation, attributed link destinations, UTF-8/UTF-16 and RTF decoding, representation fallback, reading order/deduplication, and cancellation. These build/unit checks are separate from the live acceptance results below.
- Companion build script: an empty-array failure under `nounset` in the system Bash 3.2 was fixed; the full Release build passed afterward.
- Signed companion build: the first incremental attempt retained a stale embedded ad hoc Safari extension and failed verification. A clean build succeeded. The build script now cleans before building, and stronger verification checks the embedded signatures and matching signing team; all 16 verifier fixtures pass.
- Native sharing correction: Objective-C syntax checks passed for both the main-app integration and ARC picker. Both corrected sharing entry points also passed the live presentation checks below.

## Installed-app acceptance

The following checks used the installed app and real Chrome profiles on this Mac:

| Check | Result |
| --- | --- |
| Default routing registration | `http`, `https`, and `finicky-plus` were reverified to resolve to `/Applications/Finicky Plus.app` after installing `d6ae002`. A later read-only NSWorkspace check confirmed the custom scheme, HTTPS, and bundle identifier target the installed app. The old Finicky app was moved to Trash. |
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
| Settings restoration | After the continuation, the original settings file was restored byte for byte and no Focus snapshot remained. |
| Browser cleanup | Chrome QA tabs were closed and the original Academic notebook was restored. The later HTTP control tab was closed; Safari's QA page was closed and its original Start Page restored. |
| Shortcuts cleanup | Both temporary QA shortcuts were deleted, restoring the original shortcut count. |
| Focus protocol restoration | An explicit Focus activation request selected Work/private and persisted the previous Personal default. After restarting the main app, a clear request restored regular Personal and removed the snapshot. This tests the router protocol, not OS delivery of Focus changes. |
| Focus filter configuration | System Settings → Work Focus → Add Filter listed Finicky Actions' Browser Profile filter, and its configuration opened. The flow was cancelled without editing the existing Focus. |
| Shortcuts built-in HTTP opening | The built-in Open URLs action with an HTTP(S) test link immediately opened Chrome Personal. This does not verify custom-scheme dispatch or the companion's App Intents. |
| Signed Shortcuts Settings action | Open Finicky Plus Settings rendered the real settings window with the companion already running and also started the companion successfully when it was closed. |
| Signed Shortcuts Open URLs action | Open URLs with Finicky Plus sent a synthetic `example.com` link to the picker, with its tracking parameter removed. Cancelling the picker succeeded. The temporary QA shortcut was deleted, restoring the original library count. |
| Signed Shortcuts default-profile action | Set Default Browser Profile saved Google Chrome / `Profile 2` with private mode enabled. A second invocation using `Default` with private mode off restored effective Personal/nonprivate routing. After checking those values, the original settings file was restored byte for byte because saving normalizes omitted false fields. The QA shortcut was deleted and the original library count restored. |
| Signed Safari activation | After the automated activation click was blocked, the user enabled Finicky Safari manually. Only `example.com` received Allow for One Day; no all-sites or private-browsing access was granted. The extension remains enabled with the user's approval. |
| Signed Safari toolbar | The toolbar action opened the picker with the current page's correct cleaned URL. Cancelling left the page in place. |
| Signed Safari clicked link | Right-clicking Learn more → Open Link with Finicky Plus opened the picker for exactly `https://iana.org/help/example-domains`. Cancelling caused no navigation. |
| Signed inbound Safari page Share | Companion build 5 passed the actual Safari page Share flow, opening the main picker with the correct cleaned page URL and no error. This passed before the catalogue refresh needed for selected-text discovery. |
| Signed rich-text Share | With build 5 installed and the stale user-session sharing catalogue refreshed, TextEdit's real RTF-selection Share menu listed Finicky. Three labelled links with two unique destinations produced exactly `rich-one`, then `rich-two`, with tracking removed and duplicates omitted. Both pickers were cancelled, with no extra picker. |
| Signed plain-text Share | Safari's native Share menu offered Finicky for a local HTML selection containing three plain URLs, including one duplicate. It produced exactly `text-one`, then `text-two`, with tracking removed and duplicates omitted. Both pickers were cancelled, with no extra picker. |
| Services discovery and dispatch | The existing Open URLs with Finicky Plus and Choose Browser with Finicky Plus entries both appeared in TextEdit and Safari after the metadata correction. Safari → Services → Choose Browser with Finicky Plus processed the same three-URL selection into exactly two ordered, cleaned pickers. Both were cancelled, with no extra picker. The non-prompting Open URLs service was not separately invoked. |

These results establish the three configured Chrome routes and extension entry points. They do not establish browser behavior for every supported derivative, profile login state, or every native-service adapter.

## QA corrections and coverage

QA reproduced a privacy regression in the previous installed binary: a private request could lose its required mode when a destination was selected in the picker. The source now carries required modes into the picker and the selected destination, with regression tests. Final installed build `d6ae002` passed the Incognito recheck recorded above.

The QA pass also corrected custom alternate-destination selection, private-favorite matching, routing-test profile labels, stale opening-option overlays, transactional Focus-default restoration, and balanced clipboard URL extraction. Their automated checks do not substitute for live acceptance of each flow. Clipboard extraction has automated coverage only.

The previous picker Share action dismissed its anchor window before presenting AppKit's sharing menu. The corrected picker keeps its panel visible and presents sharing from the button's mouse-down event; the standalone share request provides a retained panel with a Share button. Native syntax checks and live presentation/cancellation passed. Delivery to external sharing services from these outbound controls was not tested.

Selected-text sharing had separate source and discovery defects. The final activation predicate accepts `contentText` when the `attachments` key is absent, accepts supported text/RTF/web-URL representations, and bounds item and attachment counts. The loader preserves attributed link destinations and reading order, supports rich and plain text, falls back from unusable representations, and deduplicates URLs. A stale user-session `sharingd` catalogue still excluded the installed extension even when local matching passed. One graceful restart of only the verified current-user sharing daemon refreshed that catalogue; the same probe then included Finicky for plain text, RTF, and URLs. TextEdit rich-text and Safari plain-text Share flows subsequently passed. No database or settings reset was performed. The predicate and loader corrections address their own identified gaps; neither alone explains the stale-catalogue failure.

The main app's two existing Services omitted `NSRequiredContext`, which Apple requires for automatic menu presentation. Empty context dictionaries were added without creating duplicate services or changing the handlers. Both installed entries appear in TextEdit and Safari, and the Choose Browser service passed the Safari multiple-URL dispatch check above. See [Apple's Services properties](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/SysServices/Articles/properties.html).

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
| Native Shortcuts actions | All five actions appear in the catalog. With Apple Development signing, Settings, Open URLs, and Set Default Browser Profile passed the live checks above. Open Clipboard and Copy Clean URL still have no live native-action acceptance result. The earlier ad hoc build failed Settings with error 1200 and missing Team ID diagnostics; that Settings failure no longer reproduces with the signed build. |
| Safari extension | Enabled manually by the user. Toolbar/current-page and context-menu/clicked-link actions passed with only `example.com` Allow for One Day access; both pickers were cancelled without navigation. No all-sites, private-browsing, or unsigned-extension permission was enabled. |
| Inbound Share extension | Signed build 5 passed Safari page-URL sharing, TextEdit RTF selection with labelled links, and Safari plain-text selection. Multiple-link cases preserved order, removed tracking/duplicates, and cancelled without extra pickers after the stale user-session catalogue was refreshed. This is distinct from the main app's outbound Share menu. |
| Selected-text Services | Both existing entries appear in TextEdit and Safari. Choose Browser passed Safari multiple-URL dispatch with order, cleanup, deduplication, and cancellation. Open URLs was not separately invoked. |
| Focus filter | Discovery/configuration UI and the router's activate → restart → clear restoration protocol passed. No actual Focus was activated or changed during QA; OS delivery remains unverified. |
| Handoff | Cross-device delivery has not been verified. |
| Firefox extension and other browsers | Source/build coverage only; the live extension checks above used Chrome. |
| Chromium PWAs and native-service adapters | Fixture/unit coverage; no live installed PWA, authenticated service handoff, or meeting join was verified. |

A successful companion action confirms dispatch to macOS, not an acknowledgement from the destination browser. The signed Settings and Open URLs tests also observed the resulting settings window and picker. Before signing, the proposed built-in custom-scheme fallback failed: direct `finicky-plus://settings` input and typed URL → Open URLs both stalled until cancelled. No matching error 1200 was observed for those attempts, and they have not been retested after signing; no working fallback is claimed. The built-in HTTP(S) control and signed native actions are separate results.

An Apple Development identity was created through Xcode without revoking either existing certificate. The main app, companion, and all three embedded extensions now verify with the same Apple signing team. Development signing does not establish notarization or successful execution of untested integrations. The fork has no automatic update feed. See [Finicky Actions setup](../apps/integrations/README.md) for the runtime findings and remaining acceptance checks.

See [the capability matrix](PARITY.md) for feature coverage and service-specific limits.
