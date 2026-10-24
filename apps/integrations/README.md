# Finicky Actions

This macOS companion adds native system integrations to Finicky Plus. The Go routing app remains the URL handler; the companion and its sandboxed extensions send encoded `finicky-plus://` requests using `NSWorkspace`, without shell commands.

## Included

| Integration | Behavior |
| --- | --- |
| Open URLs with Finicky Plus | Accepts multiple web URLs, optional browser/profile/directory, picker toggle, and private/new-window/background options. |
| Set Default Browser Profile | Changes the default destination and optional private mode inside Finicky Plus. |
| Open Clipboard | Sends clipboard links through the picker or normal routing. |
| Copy Clean URL | Asks the main app to clean and copy the supplied URL. |
| Settings action | Opens the main Finicky Plus settings window. |
| Browser Profile Focus filter | Applies a browser/profile and optional private mode while the Focus is active and requests restoration when cleared. |
| Safari app extension | Toolbar button opens the current page in the picker; a context-menu item opens the clicked link. |
| Share extension | Extracts web URLs from URL and text attachments, removes duplicates, and opens each in the picker. |

The extensions do not intercept ordinary navigation. Safari's content script only reads the link that the user right-clicked. The Share extension reads only items explicitly shared with it.

## Build

Requires Xcode with the macOS 13 SDK or newer and [XcodeGen](https://github.com/yonaskolb/XcodeGen). The companion's deployment target is macOS 13.

From the repository root:

```sh
scripts/build-integrations.sh
```

The script generates the Xcode project, builds the companion and all three embedded extensions, verifies their signatures, and copies the result to `dist/integrations/Finicky Actions.app`. It does not install the app or enable extensions.

The default is local ad hoc signing. To use an existing development or distribution identity:

```sh
FINICKY_SIGNING_IDENTITY='Apple Development: Your Name (TEAMID)' \
FINICKY_DEVELOPMENT_TEAM='TEAMID' \
scripts/build-integrations.sh
```

`FINICKY_INTEGRATIONS_OUTPUT` changes the output directory. `CONFIGURATION` defaults to `Release`.

Set `FINICKY_REQUIRE_RUNTIME_SIGNING=1` when preparing a build for native integration acceptance. The build then requires an existing Apple-issued signing identity and team, and checks that the companion and embedded extensions share that team. It never creates certificates, changes Keychain trust, or invents signing entitlements.

You can inspect an existing bundle without launching or changing it:

```sh
bash scripts/check-integration-signing.sh --require-team '/Applications/Finicky Actions.app'
```

### Native Shortcuts communication failures

On the October 24, 2026 deployment, the catalog correctly listed all five actions, but running the Settings action failed while macOS established its connection to the companion, before action execution. Scoped system logs reported `LNConnectionErrorDomain` 1200 (“AppleEvents communication failed”) and an unavailable signing Team ID. The installed ad hoc signature had no Team ID, and this Mac had no valid code-signing identity. The failure persisted after all four development copies were unregistered, the installed app and extensions were registered again, and the installed companion was restarted. The retry resolved the correct installed process and still reported the missing Team ID. Passing `codesign --verify` and discovering action metadata therefore did **not** establish that native actions could execute.

Before attributing a failure to signing, keep only the installed app registered with Launch Services and restart that companion after replacing its binary. Development builds in `dist` and DerivedData can otherwise compete under the same bundle identifier. Do not delete Shortcuts databases or add Apple's private entitlements as a workaround. Once a real Apple Development or Developer ID identity is available, rebuild the companion and all its extensions together and repeat the actual action test. This is the remaining diagnosis to validate; a signed build has not yet been tested on this Mac. Apple's [code-identity guidance](https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements) explains why an ad hoc seal is not a persistent developer identity.

As a local fallback, a normal shortcut can use the built-in **URL** and **Open URLs** actions with the `finicky-plus://` commands below. That invokes the router's tested URL handler directly and does not depend on the companion's App Intents connection. It does not make the companion's native actions or Focus extension verified.

## Enable on your Mac

1. Install and open Finicky Plus once so macOS registers its URL scheme.
2. Move `Finicky Actions.app` to Applications and open it once. Keep the companion installed; its embedded extensions and Shortcuts metadata are registered through this bundle.
3. In Shortcuts, search for **Finicky Actions** to add actions. Use the browser's exact profile name or Chromium directory when specifying a profile.
4. In Safari, open Settings → Extensions and enable **Finicky Safari**. Grant website access for the websites where you want its contextual link action. A local ad hoc build may require Safari's developer option to allow unsigned extensions; a properly signed build avoids that development-only workflow.
5. In a Share menu, enable **Open with Finicky Plus** if it is hidden. The exact Extensions settings location varies by macOS version.
6. In System Settings → Focus, add the **Browser Profile** app filter and enter the browser and optional profile. The separate App Intents extension lets the system deliver changes while the companion is closed.

Focus filter delivery and extension registration are controlled by macOS. A successful build does not confirm that Safari, Shortcuts, Share, and Focus have been enabled or exercised on the machine. If a macOS release fails to deliver Focus filter changes, use Shortcuts automations with the Set Default Browser Profile action as a fallback.

## Protocol

Query values are constructed through `URLComponents`; embedded URLs are never interpolated as raw query strings. Each URL is sent separately to match the main application's single-link request parser. Opening validates the entire input list first and rejects non-HTTP(S) links before dispatching anything. Private, new-window, and background options default to false.

| Action | Query fields |
| --- | --- |
| `finicky-plus://open` | `url`, `prompt`, `private`, `newWindow`, `background`, optional `browser`, `profile`, `profileDirectory` |
| `finicky-plus://set-default` | `browser`, `private`, optional `profile`, `profileDirectory` |
| `finicky-plus://clipboard` | `prompt` |
| `finicky-plus://copy` | `url` |
| `finicky-plus://settings` | none |
| Focus activation | `set-default` with `focus=active` and a destination |
| Focus deactivation | `set-default` with `focus=clear` |

The main app owns the previous-default snapshot for Focus restoration. Calling a shortcut reports successful dispatch to macOS; it is not an acknowledgement that the selected browser finished opening or that clipboard processing has completed.

## Verify

```sh
xcodebuild \
  -project apps/integrations/FinickyIntegrations.xcodeproj \
  -scheme FinickyActions \
  -configuration Debug \
  -destination 'platform=macOS' \
  -derivedDataPath apps/integrations/.build/DerivedData \
  CODE_SIGN_IDENTITY=- test
```

The unit tests cover query-value isolation, the profile-directory protocol key, URL extraction/deduplication, all-or-nothing web-URL validation, and per-link opening options. Runtime acceptance checks are: open a Safari page and clicked link; share text with multiple links; run each action from Shortcuts; activate and clear a Focus and verify the restored profile. Browser launching and clipboard behavior require the main app running.

Implementation follows Apple's [Safari context-menu](https://developer.apple.com/documentation/safariservices/adjusting-settings-for-contextual-menu-items), [toolbar](https://developer.apple.com/documentation/safariservices/adjusting-settings-for-a-toolbar-item), and [Focus filter](https://developer.apple.com/documentation/appintents/defining-your-app-s-focus-filter) APIs. The toolbar artwork is an original vector drawing included in this directory.
