# Development build verification

This branch contains a local development build of Finicky Plus. It is not an App Store release or a certified replacement for Velja.

## Automated checks

- Config API: type checking and Vitest, including pre-match and per-rule transformations, prompt routing, private targets, and profile directories.
- Settings UI: Svelte/type checking, production build, and Node tests covering settings normalization, profiles, native-service selection, scripts, private defaults, history reopening, and prompt results.
- Native router: `go test ./... -race`, including stable Chromium profiles, modern Firefox profile groups, native handoff fallback, history concurrency/capping, URL transformations/timeouts, clipboard request parsing, redirect boundaries, and logging.
- Browser add-on: Node tests for encoded toolbar/context-menu requests and unsupported URL schemes.
- Native companion: universal macOS build, signature verification, and XCTest coverage for encoded protocol values, shared URL extraction, web-only opening, and private/new-window/background options.

## Native preview acceptance

The separate preview identity was launched with illustrative Personal, Work, and Academic destinations and an isolated rules file. Browser launching is simulated in this mode.

- Tracking cleanup was visible in the picker URL.
- The right-click destination menu exposed opening, background, new window, private, and remembered-website actions.
- Remembering Work created the expected domain rule in the isolated file and the settings immediately displayed it.
- The settings rendered in the native WebKit window with profiles, preferences, history, and integrations navigation.
- Pressing the Work shortcut dismissed the picker and produced a simulated Chrome launch with `--profile-directory=Profile 2` and the cleaned URL.

The preview uncovered a Cocoa startup issue that reinterpreted positional URL arguments as local document paths. The native file-event handler now ignores already-processed command-line arguments. Repeating the native preview test confirmed a single picker and a single simulated browser launch.

## Runtime limits

Actual browser account login state and behavior across every browser/version were not exercised. Chromium profile metadata on this Mac requires user-granted folder access; explicit profile-directory targets are available without reading account metadata. A folder chooser was implemented, but access was not automatically granted during verification.

The companion app and extension bundles build successfully. Shortcuts registration/execution, Safari and Share extension activation, Focus activation/deactivation, and cross-device Handoff still need acceptance checks after installation. No extensions were enabled and the system default browser was not changed during development. Shortcuts actions report successful dispatch to macOS, not an acknowledgement from the destination browser.

There is no developer signing identity on this build machine. Ad hoc signatures are suitable for local development; they do not establish notarization or guarantee extension registration on another Mac. The fork has no automatic update feed.

See [the capability matrix](PARITY.md) for feature coverage and service-specific limits.
