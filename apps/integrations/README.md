# Waystile Actions

The optional Waystile Actions companion supplies native macOS integrations. The main Waystile router remains the URL handler; the companion and its sandboxed extensions send encoded `finicky-plus://` requests through `NSWorkspace`.

## Included

| Integration | Behavior |
| --- | --- |
| Open URLs with Waystile | Opens multiple web URLs with an optional destination, picker, and supported private/new-window/background options. |
| Set Default Browser Profile | Changes Waystile's default destination, not the macOS default-browser setting. |
| Open Clipboard | Sends clipboard links through the picker or normal routing. |
| Copy Clean URL | Asks the router to clean and copy a supplied URL. |
| Open Waystile Settings | Opens the router's settings window. |
| Browser Profile Focus filter | Applies a destination during a Focus and requests restoration when cleared. |
| Safari extension | Opens the current page from its toolbar or a clicked link from its context menu. |
| Share extension | Extracts web links from shared URLs, plain text, and rich text; preserves order and removes duplicates. |

The extensions do not intercept ordinary navigation. Safari's content script reads the link the user right-clicks. The Share extension reads the items explicitly shared with it. Each integration needs its own macOS permissions.

## Build and signing

Requires macOS, full Xcode with a macOS 13 or newer SDK, and [XcodeGen](https://github.com/yonaskolb/XcodeGen). The companion's deployment target is macOS 13.

From the repository root:

```sh
scripts/build-integrations.sh
```

The script creates a clean universal build for Apple silicon and Intel, enables hardened runtime, verifies the embedded signatures, and writes `dist/integrations/Waystile Actions.app`. It does not install the app, enable extensions, or register Shortcuts. Local builds use ad hoc signing by default, which does not establish OS acceptance of native integrations.

To use an existing Apple-issued identity:

```sh
FINICKY_SIGNING_IDENTITY='Apple Development: Your Name (TEAMID)' \
FINICKY_DEVELOPMENT_TEAM='TEAMID' \
scripts/build-integrations.sh
```

`FINICKY_INTEGRATIONS_OUTPUT` changes the output directory. `CONFIGURATION` defaults to `Release`. Set `FINICKY_REQUIRE_RUNTIME_SIGNING=1` to require a matching Apple signing identity and team during verification. Public Developer ID distribution also needs notarization; development signing alone is insufficient. See the release notes for the exact distributed build and its acceptance status.

The public name and output bundle are **Waystile Actions**. Internal Xcode target names, executable names, extension bundle paths, and identifiers remain stable for compatibility. In particular, the internal products retain the names `Finicky Actions`, `Finicky Safari`, `Open with Finicky Plus`, and `Finicky Focus`; these are not additional apps to install. The public companion is version 0.1.0, build 6.

## Enable on your Mac

1. Install and open Waystile once so macOS knows its URL handler.
2. Install and open **Waystile Actions**. The companion can open the router's settings and explains each integration.
3. In Shortcuts, search for **Waystile** and add the action you need. Run a harmless test link before using it in an automation.
4. Enable **Waystile Safari** in Safari Settings → Extensions. Grant access to the sites where you want to invoke its actions.
5. Enable **Open with Waystile** in macOS extension settings to use the Share menu. The settings location varies by macOS version.
6. For Focus behavior, add the Browser Profile filter to a Focus and choose its destination. Verify both activation and restoration before relying on the automation.

The router and companion have separate responsibilities. A companion action returning successfully confirms dispatch to macOS, not acknowledgement that a browser opened or a clipboard operation completed.

## Compatibility protocol

The `finicky-plus://` scheme is deliberately retained. Query values use `URLComponents`, and each URL is sent separately to match the router's single-link parser. Opening validates the entire input list before dispatching and rejects non-HTTP(S) links. Private, new-window, and background options default to false.

| Action | Query fields |
| --- | --- |
| `finicky-plus://open` | `url`, `prompt`, `private`, `newWindow`, `background`, optional `browser`, `profile`, `profileDirectory` |
| `finicky-plus://set-default` | `browser`, `private`, optional `profile`, `profileDirectory` |
| `finicky-plus://clipboard` | `prompt` |
| `finicky-plus://copy` | `url` |
| `finicky-plus://settings` | none |
| Focus activation | `set-default` with `focus=active` and a destination |
| Focus deactivation | `set-default` with `focus=clear` |

The router owns the previous-default snapshot for Focus restoration.

## Coverage and limits

Earlier Apple Development builds passed live checks for Settings, Open URLs, and Set Default Browser Profile Shortcuts actions; Safari toolbar and context-menu actions; Safari page sharing; and rich/plain-text sharing with link order, cleanup, deduplication, and cancellation. Those results do not establish acceptance of the newly branded Developer ID companion on another Mac.

Open Clipboard and Copy Clean URL still need live native-action checks. Focus configuration and explicit protocol restoration have been exercised, but actual OS delivery of Focus changes remains unverified. Handoff between Macs and arbitrary browser combinations also need separate acceptance.

The native tests cover URL encoding, destination parameters, web-only dispatch, opening options, bounded Share activation, rich-text link extraction, order/deduplication, fallback, and cancellation. Run them using the preserved internal scheme:

```sh
xcodebuild \
  -project apps/integrations/FinickyIntegrations.xcodeproj \
  -scheme FinickyActions \
  -configuration Debug \
  -destination 'platform=macOS' \
  -derivedDataPath apps/integrations/.build/DerivedData \
  CODE_SIGN_IDENTITY=- test
```

See [release verification](../../docs/VERIFICATION.md) for build-specific results. The Safari integration follows Apple's [context-menu](https://developer.apple.com/documentation/safariservices/adjusting-settings-for-contextual-menu-items) and [toolbar](https://developer.apple.com/documentation/safariservices/adjusting-settings-for-a-toolbar-item) APIs; the Focus integration uses Apple's [Focus filter API](https://developer.apple.com/documentation/appintents/defining-your-app-s-focus-filter).
