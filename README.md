# Waystile

**Every link in the right place.**

Waystile is a macOS link router for people who move between work, personal, and academic browsing. Set it as your default browser, then use visual rules to send links to the right browser or profile. When a link needs a choice, the native picker lets you choose its destination.

Waystile is built on [Finicky](https://github.com/johnste/finicky), with a visual settings app, browser-profile controls, optional iCloud settings sync, and native integrations. This is the **0.1.0 beta**. Browser coverage and system integrations vary; it is not a claim of complete parity with another product.

[Project page](https://klee.page/projects/waystile/) · [Releases](https://github.com/kevincorvallis/waystile/releases) · [Report an issue](https://github.com/kevincorvallis/waystile/issues)

## What it does

- Route links by website, source app, or a combination of rules.
- Save browser and profile destinations with your own labels and colors.
- Choose a destination from a native picker, with keyboard shortcuts and supported private-window options.
- Remove common tracking parameters, unwrap recognized redirect links, and optionally expand short URLs.
- Test a link before opening it, inspect optional local history, and import or export visual settings.
- Share visual rules and profile labels through iCloud Drive, with a separate destination mapping on each Mac.
- Use optional Chromium, Firefox, Safari, Share, Shortcuts, and Focus integrations.

Chrome-family and Firefox profile support depends on the browser's available metadata and launch behavior. Safari and some other browsers do not expose equivalent profile targeting. Unsupported private-window requests are rejected rather than opened as regular windows. See the [capability matrix](docs/PARITY.md) for detailed limits and historical test evidence.

## Install

Requires macOS 13 or later. Universal builds support Apple silicon and Intel Macs.

Get Waystile from this project's [releases](https://github.com/kevincorvallis/waystile/releases), not an upstream Finicky download or Homebrew cask. Each release states whether it contains a signed, notarized app or source only. The app is not currently distributed through the Mac App Store. Development and ad hoc builds are not notarized public releases.

For a release that supplies `Waystile.app`:

1. Quit your existing Waystile or Finicky Plus router. Keep the old app and settings as a backup.
2. Move `Waystile.app` into Applications and open it.
3. Add your browser profiles, choose a default destination, and use **Test a link** to check your rules.
4. Select **Waystile** as the default web browser in macOS System Settings when you are ready to route external links.

If you are upgrading from Finicky Plus, the app keeps its existing identity and settings locations. Do not run both copies at once. Waystile does not alter or migrate browser account data.

A legacy `~/.finicky.js` configuration is still read. Its JavaScript handlers take priority over visual rules. If you want to use visual settings exclusively, preserve a backup and migrate those rules first. The `--no-config` flag can disable JavaScript loading for a test or preview.

## Browser and system integrations

The browser add-on sends a link or current page to Waystile from its toolbar, context menu, or Option-click gesture. Ordinary navigation stays in the browser unless you invoke an integration. Chromium extensions can be loaded unpacked from their extracted folder; Firefox public distribution requires Mozilla signing. No Chrome Web Store or Firefox Add-ons listing is claimed for this beta.

The optional **Waystile Actions** companion provides native Shortcuts actions, a Safari extension, a Share extension, and a Focus filter. It needs separate installation and OS permissions. A successful build does not mean macOS has registered or enabled every integration. See the [companion guide](apps/integrations/README.md).

## Privacy and local learning

Routing rules are evaluated locally. Browser profiles and accounts stay in their browsers. Waystile reads profile metadata to discover destinations; it does not copy browser history, cookies, or passwords between profiles.

Link history is optional and local, retaining up to 200 entries. Private launches are omitted from that history. Diagnostics are separate: optional request/file logging may contain URLs, including private-mode requests, so review logs before sharing them. Optional short-link expansion contacts the linked services to resolve redirects.

The **Learning** panel is an opt-in local experiment. It records sanitized domains and source-app names from explicit profile choices, excludes private and unknown-source requests, and supports correction and deletion. Predictions do not change routing. No model service receives those observations through this feature. See the [learning contract](docs/LEARNING.md).

The **Sync** panel uses your iCloud Drive to share visual routing settings. Each Mac maps shared profile labels to its own local destinations. Browser data, learning observations, link history, and logging preferences remain local. Concurrent edits require an explicit choice of an entire settings version; there is no automatic rule-by-rule merge. See [multi-Mac setup](docs/SECOND_MAC.md) and the [sync contract](docs/SYNC.md).

## Build from source

Requires macOS 13+, Xcode command-line tools, Go 1.24 or later, Node.js 22 or later, and npm. Full Xcode and XcodeGen are additionally required for the native companion.

```sh
git clone https://github.com/kevincorvallis/waystile.git
cd waystile
npm --prefix packages/config-api ci
npm --prefix packages/finicky-ui ci
BUILD_UNIVERSAL=1 ./scripts/build-fork.sh
```

The router is written to `apps/finicky/build/Waystile.app`. The internal executable remains `Contents/MacOS/Finicky` for compatibility. Browser extension archives are written to `apps/browser-addon/build/chrome.zip` and `firefox.zip`.

Build scripts do not install the app or change the default browser. Local router builds use ad hoc signatures. See the release script and release notes for distribution signing requirements. There is no automatic fork-update channel configured by default.

To build the optional native companion too, use `./scripts/build-fork.sh --integrations` and follow the [companion signing instructions](apps/integrations/README.md).

For an isolated preview that simulates browser launches and uses separate settings:

```sh
./scripts/build-preview.sh
./scripts/run-preview.sh
```

To resolve a URL without opening a browser:

```sh
'apps/finicky/build/Waystile.app/Contents/MacOS/Finicky' \
  --no-config --rules path/to/rules.json --test-url 'https://example.com'
```

## Compatibility and storage

The public name is Waystile. These earlier identifiers deliberately remain stable so existing installations, shortcuts, and shared settings keep working:

- Bundle identity: `com.kevinlee.finicky`.
- Integration scheme: `finicky-plus://`.
- Visual settings, history, and learning: `~/Library/Application Support/Finicky Plus/`.
- Optional file logs: `~/Library/Logs/Finicky Plus/`.
- iCloud routing settings: `iCloud Drive/Finicky Plus Sync/`.
- JavaScript configuration: `~/.finicky.js` or an explicitly selected path.

These are compatibility paths, not separate applications to install. Visual settings export does not include executable JavaScript or browser data.

## Validation and limitations

Automated tests cover rule and destination validation, browser launch arguments, router-loop prevention, URL cleanup, profile discovery fixtures, settings sync, and local learning. UI checks cover draft saving, focus handling, and mapping errors. Live testing has covered selected Chrome profile routes and native integrations on development builds, not every browser, OS version, or integration combination.

Current limitations include incomplete live coverage for Firefox variants, actual OS delivery of Focus changes, Handoff between devices, and some companion actions. App Store distribution, full-app sandbox conversion, and a public automatic updater are not part of this beta. The public [verification summary](docs/VERIFICATION.md) distinguishes release checks from earlier development-build evidence.

## Credits and license

Waystile is derived from **Finicky**, created by [John Sterling](https://github.com/johnste) and its [contributors](https://github.com/johnste/finicky/graphs/contributors). The original [MIT license](LICENSE) is retained and included in the app. Waystile's route-mark icon is original artwork; its vector source and native icon generator are included in `design/` and `scripts/create-waystile-icons.swift`.

The settings interface uses [Lucide icons](https://lucide.dev) and [Radix Colors](https://www.radix-ui.com/colors). Their license notices are included in the app's resources. Waystile is an independent project; upstream Finicky releases and browser-extension listings are separate products.
