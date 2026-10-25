# Finicky Plus

A personal fork of [Finicky](https://github.com/johnste/finicky) with visual profile routing and a native browser picker, developed toward the capabilities documented by Velja. This is a development build, **not a claim of complete feature parity**. See the [capability matrix](docs/PARITY.md) for supported behavior and remaining differences.

The fork adds saved Work/Personal/Academic-style destinations, website and source-app rules, private windows, URL cleanup and transformation editors, recent history, profile access controls, and import/export. A companion app supplies native Shortcuts actions, Safari and Share extensions, and a Focus filter. Chromium and Firefox extensions are included separately.

The Learning panel offers an opt-in local experiment: it learns from explicit profile choices and checks predictions in the background. It stores sanitized domains and source apps, excludes private and unknown sources, and provides corrections and deletion controls. Predictions do not change routing. See the [learning contract and evaluation limits](docs/LEARNING.md).

The Sync panel shares visual routing settings through iCloud Drive. Each Mac maps shared profile labels to its own browser destinations; learning, history and browser data remain local. Concurrent changes require an explicit version choice. See [second-Mac setup](docs/SECOND_MAC.md) and the [sync contract](docs/SYNC.md).

## Build this fork

Requires macOS 13+, Xcode command-line tools, Go, Node.js, and npm. The native companion additionally needs full Xcode and XcodeGen.

```sh
npm --prefix packages/config-api ci
npm --prefix packages/finicky-ui ci
./scripts/build-fork.sh --integrations
```

Outputs:

- `apps/finicky/build/Finicky Plus.app` — the router and settings.
- `dist/integrations/Finicky Actions.app` — Shortcuts, Safari, Share, and Focus integrations.
- `apps/browser-addon/build/chrome.zip` and `firefox.zip` — browser extensions.

Build scripts do not install or replace applications. Local builds use ad hoc signatures, not notarized distribution. Enable native integrations using the [companion setup guide](apps/integrations/README.md); the Chromium extension can be loaded unpacked from its extracted archive. Firefox distribution requires Mozilla signing; temporary developer loading is available for local testing.

Use `BUILD_UNIVERSAL=1 ./scripts/build-fork.sh` to build the router for both Apple silicon and Intel Macs.

For a safe visual preview that simulates browser launches and uses separate settings:

```sh
./scripts/run-preview.sh
```

To resolve a URL without opening a browser:

```sh
'apps/finicky/build/Finicky Plus.app/Contents/MacOS/Finicky' \
  --no-config --rules path/to/rules.json --test-url 'https://example.com'
```

The fork uses the `finicky-plus://` scheme, `com.kevinlee.finicky` identity, and `~/Library/Application Support/Finicky Plus/` storage. An existing `~/.finicky.js` is still read unless `--no-config` is supplied. JavaScript handlers have priority over visual rules; an explicitly saved visual default controls unmatched links. No existing browser profile data is modified. See [build verification and limits](docs/VERIFICATION.md).

## Upstream project documentation

The installation instructions and releases below describe upstream Finicky, not this fork. Finicky's MIT license and attribution are retained.

<div align="center"><picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://github.com/user-attachments/assets/ab66e6cc-25d1-4f5f-9c98-c742ecb2261f">
  <img alt="Finicky Logo" height="110" src="https://github.com/user-attachments/assets/067d7619-a2be-49dd-8a4f-5e9a46fa632a">
</picture>
<br/><br/>
 <strong>Always open the right browser</strong><br>
    <br/>

</div>

Finicky is a macOS application that allows you to set up rules that decide which browser is opened for every url. With Finicky as your default browser, you can tell it to open Bluesky or Reddit in one browser, and LinkedIn or Google Meet in another.

- Route any URL to your preferred browser with powerful matching rules
- Automatically edit URLs before opening them (e.g., force HTTPS, remove tracking parameters)
- Write rules in JavaScript or TypeScript for complete control
- Create complex routing logic with regular expressions and custom functions
- Handle multiple browsers and apps with a single configuration
- Keep your workflow organized by separating work and personal browsing

[![GitHub prerelease](https://badgen.net/github/release/johnste/finicky?color=purple)](https://GitHub.com/johnste/finicky/releases/) ![MIT License](https://badgen.net/github/license/johnste/finicky) ![Finicky v4 release](https://badgen.net/github/milestones/johnste/finicky/6?color=pink)

## Table of Contents

- [Installation](#installation)
- [Basic configuration](#basic-configuration)
- [Configuration](#configuration)
- [Migrating from Finicky 3](#migrating-from-finicky-3)

## Installation

- Download from [releases](https://github.com/johnste/finicky/releases)
- Or install via homebrew: `brew install --cask finicky`
- Create a JavaScript or TypeScript configuration file at `~/.finicky.js`.  Have a look at the example configuration below, or in the `example-config` folder.
- Start Finicky (in Applications, or through Spotlight/Alfred/Raycast) and allow it to be set as the default browser. Starting Finicky manually opens the configuration/troubleshooting window. 

## Basic configuration

Here's a short example configuration that can help you get started

```js
// ~/.finicky.js
export default {
  defaultBrowser: "Google Chrome",
  rewrite: [
    {
      // Redirect all x.com urls to use xcancel.com
      match: "x.com/*",
      url: (url) => {
        url.host = "xcancel.com";
        return url;
      },
    },
  ],
  handlers: [
    {
      // Open all bsky.app urls in Firefox
      match: "bsky.app/*",
      browser: "Firefox",
    },
    {
      // Open google.com and *.google.com urls in Google Chrome
      match: [
        "google.com/*", // match google.com urls
        "*.google.com*", // also match google.com subdomains
      ],
      browser: "Google Chrome",
    },
  ],
};
```

See the [configuration](#configuration) for all the features Finicky supports.

## Configuration

Finicky has extensive support for matching, rewriting and starting browsers or other application that handle urls. See the wiki for the [full configuration documentation](<https://github.com/johnste/finicky/wiki/Configuration-(v4)>) explaining available, APIs and options as well as detail information on how to match on urls.

- The wiki has some good [configuration ideas](https://github.com/johnste/finicky/wiki/Configuration-ideas).
- Visit [discussions](https://github.com/johnste/finicky/discussions) to discuss supporting specific apps.

## Migrating from Finicky 3

Please see the [wiki page](https://github.com/johnste/finicky/wiki/Migrating-from-Finicky-3) for updating info and migrating your configuration

# Other

### Browser extensions

Finicky has browser extensions for Chrome and Firefox. They add an "open with Finicky" on links, and alt-clicking links opens them in Finicky directly.

- [Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/finicky/)
- [Chrome Web Store](https://chromewebstore.google.com/detail/finicky/kcnjhpdfmjcbohngnmobipdllkhnpdbk)

### Building Finicky from source

See [Building Finicky from source](https://github.com/johnste/finicky/wiki/Building-Finicky-from-source)

### Works well with

If you are looking for something that lets you pick the browser to activate in a graphical interface, check out [Browserosaurus](https://browserosaurus.com/) by Will Stone, an open source browser prompter for macOS. It works really well together with Finicky!

### Questions

Have any other questions or need help? Please feel free to reach out to me on [Bluesky](https://bsky.app/profile/mejkarsense.se) or post an issue here

## License

[MIT](https://raw.githubusercontent.com/johnste/finicky/master/LICENSE)

Icon designed by [@uetchy](https://github.com/uetchy)
