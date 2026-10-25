# Waystile verification

This summary records checks performed while preparing the 0.1.0 beta. It describes the tested behavior and its limits; it is not an independent security audit or a claim of App Store approval.

## Router and settings window

The Go test suite passed with the race detector enabled. Focused native navigation-policy tests cover the bundled document origin, spoofed origins, credentials and ports in URLs, subframes, initial loading, redirects, and permitted external HTTP(S) links. Native bridge access is restricted to the bundled settings document in its main frame.

An isolated native preview loaded its browser choices, saved a changed fallback choice to a temporary preview configuration, and opened an About-page link through the system URL handler. The settings window remained on the bundled document. This check did not change an existing installation's routing configuration.

The preview bundle passed strict ad-hoc signature verification. The original Finicky MIT license and UI dependency notices were checked against their source files in the bundle. The settings bridge no longer logs complete message bodies or tested URLs unconditionally.

## Native companion

Waystile Actions and its Safari, Share, and Focus extensions built successfully for both Intel and Apple silicon. All four bundles passed strict signature verification with hardened runtime enabled. Their distribution-candidate entitlements contain only App Sandbox; no test-only debugging or temporary filesystem exceptions are present. The main companion includes the original Finicky license.

The companion's 16 XCTest tests passed with no failures. They cover command encoding, supported link schemes, batch validation, profile-option encoding, ordered deduplication, attributed-text links, text encodings, RTF links, cancellation, Share activation limits, and the packaged Share display name and icon.

Tests and distribution builds use separate output directories so test-injected entitlements cannot be accidentally reused for the public companion.

## Release safeguards

The release script was checked with an absent signing identity and with the router signing mode applied to the companion. Both attempts failed before modifying any app file or producing a release archive. Public packaging requires universal binaries, Developer ID signatures, hardened runtime, accepted notarization, a stapled ticket, and a successful Gatekeeper assessment.

## Limits

These checks were performed on Apple silicon. Intel slices were verified in the bundles but were not executed on Intel hardware. Unit tests and an ad-hoc signature do not establish that Shortcuts, Safari, Share, or Focus can execute under a final distribution identity. Those integrations require acceptance checks after Developer ID signing. Release-specific notarization and download checks should be recorded with the published release.

The router is not sandboxed. Optional JavaScript configuration runs with the user's privileges and should be treated as executable code. This beta has not been submitted to the Mac App Store.
