# Waystile capabilities and coverage

Waystile combines a native macOS link router with visual browser-profile settings. This matrix describes implemented behavior and known limits. It does not claim complete feature parity with another app or that every supported destination has been exercised live.

## Routing and profiles

| Capability | Current behavior | Coverage and limits |
| --- | --- | --- |
| Website and source-app rules | Ordered visual rules, wildcard matching, defaults, and link testing. | Automated rule/model tests and selected live Chrome routes. Existing JavaScript handlers retain priority. |
| Saved destinations | Browser/profile labels, colors, picker visibility, and shortcuts. | Settings and launch validation tested. Browser account creation, sign-in, deletion, and renaming stay in the browser. |
| Chromium profiles | Named metadata or an explicit profile directory. | Selected Chrome routes exercised live. Metadata coverage for other Chromium variants is not equivalent to live launch coverage. |
| Firefox profiles | Legacy profiles and supported modern profile-group metadata. | Automated discovery and argument fixtures. Firefox variants and the Firefox extension still need live acceptance. |
| Safari and other browser profiles | Ordinary installed-app routing. | Safari, Arc, and other browsers without compatible launch interfaces do not gain profile targeting through this app. |
| Private/new-window modes | Supported browser-specific launch flags and required-mode propagation through the picker. | Chrome Incognito exercised live on a development build. Unsupported modes are rejected. Safari private mode is not built in. |
| Native apps and web apps | A bounded set of link adapters, custom targets, and supported Chromium PWA wrappers. | Adapter and wrapper fixtures; no claim that all native services or real installed PWAs have been opened live. |
| Router-loop prevention | Rejects Waystile and legacy router names, app paths, and bundle identities as destinations. | Automated launch-validation checks. |

## Link handling and interface

| Capability | Current behavior | Coverage and limits |
| --- | --- | --- |
| Native picker | Queued requests, keyboard shortcuts, profile choices, background/private/new-window options, and remember-website action. | Selected live picker, cancellation, remembered-rule, and private-mode flows on development builds. |
| URL cleanup | Common tracking parameters and recognized redirect wrappers; optional short-link expansion. | Preservation, host-boundary, timeout, and redirect tests. No exhaustive tracker-removal database or authenticated redirect-page execution. |
| JavaScript transforms | Existing configuration callbacks plus global and per-rule visual editors. | Config API tests. Executable JavaScript is not shared through iCloud sync. |
| History and diagnostics | Optional local history, reopen, clear, logs, and routing test. | History order/cap/concurrency and UI model tests. Private launches are omitted from history, but optional request/file logs can still contain private-mode requests. |
| Settings UI | Shared light/dark styles, keyboard focus handling, draft Apply, and explicit validation. | Earlier live checks covered the minimum window size, dialogs, rule reordering, and mapping Save/Discard. No complete VoiceOver or oldest-WebKit certification. |
| Import/export | Visual settings JSON with validation and normal Apply behavior. | Model/serialization tests. Browser data and JavaScript files are not included. |

## macOS and browser integrations

| Integration | Current behavior | Coverage and limits |
| --- | --- | --- |
| Chromium extension | Toolbar, link/page context menu, and Option-click routing. | Three Chrome-profile routes exercised live on the previous extension build. New branding does not substitute for release acceptance. |
| Firefox extension | Equivalent protocol-based browser actions. | Source/build tests; public Firefox distribution requires Mozilla signing. |
| Shortcuts | Five native actions in the optional Waystile Actions companion. | Settings, Open URLs, and Set Default Browser Profile passed live Apple Development-build checks. Open Clipboard and Copy Clean URL still need live checks. |
| Safari extension | Current-page toolbar and clicked-link context-menu actions. | Both passed live on an earlier signed development build with limited site access. New Developer ID companion acceptance remains release-specific. |
| Inbound Share extension | Web URLs and plain/rich text, preserving link order and removing duplicates. | Earlier signed builds passed Safari page sharing, Safari text selection, and TextEdit rich-text selection, including cancellation. New companion acceptance remains separate. |
| macOS Services | Open or choose browsers for selected links. | Choose Browser passed selected multiple-link dispatch. The non-prompting Open URLs service was not separately invoked. |
| Outbound sharing | AppKit sharing popover from the picker or a dedicated entry point. | Presentation and cancellation passed. Delivery through an external sharing service was not tested. |
| Focus filter | Destination override with persisted previous-default restoration. | Filter configuration and explicit activate → restart → clear protocol tested. Real OS delivery of Focus changes remains unverified. |
| Handoff | Routes received browsing activities. | No new two-device runtime acceptance. |

Native integrations require the [separate companion](../apps/integrations/README.md), appropriate signing, OS registration, and user-granted permissions. Ordinary browser navigation remains in its browser unless an integration action sends a link to Waystile.

## Learning, sync, and distribution

| Capability | Current behavior | Coverage and limits |
| --- | --- | --- |
| Local learning | Opt-in observations of explicit profile choices, sanitized domains and source apps, correction/deletion controls, and background predictions. | Privacy, temporal-cutoff, identity, retention, and lifecycle tests; selected live observations on a development build. Predictions never route or preselect links. There is no GNN or demonstrated user-accuracy claim. |
| iCloud settings sync | Immutable settings versions and explicit destination mappings per Mac. | Automated concurrent/offline/recovery cases and an isolated second-device simulation. These are not real two-Mac delivery acceptance. Conflicts require an explicit whole-version choice. |
| Storage compatibility | Earlier bundle identity, scheme, configuration, history, learning, logging, cache, and cloud paths are retained. | Renaming does not migrate browser data or require a new settings store. |
| Distribution | Public source beta and release-specific signed artifacts when available. | No Mac App Store listing or configured automatic update channel. Full-app sandbox conversion is outside this release. Signing/notarization and native runtime acceptance are separate checks. |

For implementation detail, see [learning](LEARNING.md), [sync](SYNC.md), [UI foundations](UI.md), [multi-Mac setup](SECOND_MAC.md), and [build-specific verification](VERIFICATION.md). Earlier development-build results are useful evidence for those exact flows; they do not certify the current release on every supported Mac.
