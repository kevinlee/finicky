# Distribution

Waystile is an early beta distributed directly for macOS 13 and later, with a universal Intel / Apple silicon binary. The public source is at https://github.com/kevincorvallis/waystile. Only assets explicitly attached to a release are public downloads; local and CI builds are development artifacts.

## Reproduce a build

Install Xcode command-line tools, Go 1.24+, and Node.js 22+. Install the locked npm dependencies in `packages/config-api` and `packages/finicky-ui`, then run `BUILD_UNIVERSAL=1 ./scripts/build-fork.sh`. The resulting `apps/finicky/build/Waystile.app` is ad-hoc signed for development. No certificate or Apple credentials are included in the repository.

A distributor with an existing Developer ID identity can run:

```sh
SIGNING_IDENTITY='Developer ID Application: YOUR IDENTITY' \
  ./scripts/release.sh sign '/absolute/path/Waystile.app' '/private/release-dir'
NOTARY_KEYCHAIN_PROFILE='YOUR_EXISTING_PROFILE' \
  ./scripts/release.sh notarize '/absolute/path/Waystile.app' '/private/release-dir'
```

The signing step uses hardened runtime and Apple's timestamp service. The notarization step uploads the app to Apple, requires an Accepted result, staples the app, checks Gatekeeper, then creates the public ZIP and SHA-256 checksum. If interrupted, inspect the saved submission result before retrying; do not blindly create duplicate submissions. Notarization logs remain private. The script never installs the app or creates credentials.

Use the `sign-companion` phase instead of `sign` for Waystile Actions. The optional native companion contains extensions that must be signed inside out with their declared sandbox entitlements, matching signing team, and no test-only `get-task-allow` entitlement. Build/test products must use separate clean output directories. Use a separate private release directory for each app’s notarization. See `apps/integrations/README.md` for integration setup and limits. Chrome and Firefox add-ons are separate; an unsigned Firefox build is for development, not a claim of Mozilla store approval.

## App Store status

This release is not submitted to the Mac App Store. The router currently needs unrestricted local profile discovery and optional executable JavaScript configuration. A Store version needs a separate sandbox design, access model, packaging of integrations, and update path before submission. Thanos is a separate local CLI/skill, not an embedded Waystile service.

Apple documents the distribution requirements in [Distributing apps outside the Mac App Store](https://developer.apple.com/macos/distribution/) and the [App Review Guidelines](https://developer.apple.com/app-store/review/guidelines/#hardware-compatibility). Developer ID notarization is not App Store review or a guarantee of feature correctness.
