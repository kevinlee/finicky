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

These are implemented behaviors, not a claim that every integration executes on the tested Mac. Native Shortcuts execution currently fails, Safari activation is blocked, inbound Share is missing from the host menu, and OS delivery of Focus changes is unverified. See the runtime results below. The extensions do not intercept ordinary navigation. Safari's content script only reads the link that the user right-clicked. The Share extension reads only items explicitly shared with it.

## Build

Requires Xcode with the macOS 13 SDK or newer and [XcodeGen](https://github.com/yonaskolb/XcodeGen). The companion's deployment target is macOS 13.

From the repository root:

```sh
scripts/build-integrations.sh
```

The script generates the Xcode project, builds the companion and all three embedded extensions, verifies their signatures, and copies the result to `dist/integrations/Finicky Actions.app`. It does not install the app or enable extensions. The full Release build passed after correcting empty-array expansion under `nounset` on the system Bash 3.2.

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

The previously proposed **URL** → **Open URLs** custom-scheme fallback did not work on this Mac. A built-in Open URLs action with an HTTP(S) link immediately opened Chrome Personal, but passing `finicky-plus://settings` directly stalled. A separate typed URL → Open URLs sequence also remained running for more than a minute before cancellation. These attempts did not show the same error 1200 as the native App Intent; the custom-scheme failure has no verified root cause. Read-only NSWorkspace checks still resolved the custom scheme, HTTPS, and the router's bundle identifier to the installed Finicky Plus app. Both temporary QA shortcuts and the HTTP control tab were removed afterward. Do not treat ordinary HTTP(S) opening as evidence that custom-scheme shortcuts or native actions work.

### Share and Focus runtime results

On the continuation of the October 24 QA pass, companion build 2 from `c16feeb` was installed and signature-verified; Finicky Plus remained on `d6ae002`. The installed Share extension contains activation dictionary version 2. Development app copies were unregistered again so the installed companion remained the intended registration.

Version 2 fixes a concrete activation gap: the default version 1 can hide an extension when a host offers supported URL/text and unsupported representations together. [Apple documents this matching difference](https://developer.apple.com/library/archive/documentation/General/Reference/InfoPlistKeyReference/Articles/AppExtensionKeys.html). Six native tests pass, including the source-plist regression check. Despite the corrected installed plist, **Open with Finicky Plus** was still absent from Safari's actual Share menu. Earlier read-only AppKit enumeration also omitted it for URL, plain text, URL-plus-title, and attributed text. The cause is unresolved. In particular, the missing Team ID observed for Shortcuts is not proof of a Share signing failure; Apple's [extension testing guidance](https://developer.apple.com/library/archive/documentation/General/Conceptual/ExtensibilityPG/ExtensionCreation.html) permits matching ad hoc signatures for test targets.

System Settings listed **Finicky Actions → Browser Profile** when adding a filter to Work Focus, and its configuration UI opened. The flow was cancelled without editing that existing Focus. Separately, an explicit protocol activation selected Work/private and persisted the previous Personal default. After restarting Finicky Plus, explicit clear restored regular Personal and removed the snapshot. The original main-app settings were restored byte for byte and the snapshot was confirmed absent after the continuation. This verifies discovery and the router's restoration protocol, **not OS delivery** when a Focus activates or deactivates.

## Enable on your Mac

1. Install and open Finicky Plus once so macOS registers its URL scheme.
2. Move `Finicky Actions.app` to Applications and open it once. Keep the companion installed; its embedded extensions and Shortcuts metadata are registered through this bundle.
3. In Shortcuts, search for **Finicky Actions** to add actions. Use the browser's exact profile name or Chromium directory when specifying a profile.
4. With a properly signed build, open Safari Settings → Extensions and enable **Finicky Safari**, then grant website access for its contextual link action. Activation is blocked for the current local ad hoc build; unsigned-extension protection was not disabled during QA.
5. In macOS Extensions settings, enable **Open with Finicky Plus**. The exact location varies by macOS version. It is enabled on the tested Mac but still absent from Safari's Share menu, so enabling alone is not acceptance.
6. In System Settings → Focus, add the **Browser Profile** app filter and enter the browser and optional profile. The separate App Intents extension lets the system deliver changes while the companion is closed.

Focus filter delivery and extension registration are controlled by macOS. A successful build does not confirm execution. Neither native Shortcuts actions nor the custom-scheme shortcut path are a verified Focus fallback on this Mac.

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

The main app owns the previous-default snapshot for Focus restoration. If a companion action completes successfully, it confirms dispatch to macOS, not that the selected browser finished opening or clipboard processing completed. Successful native action execution has not been established on the tested Mac.

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

The six unit tests cover query-value isolation, the profile-directory protocol key, URL extraction/deduplication, all-or-nothing web-URL validation, per-link opening options, and the Share activation policy. Remaining runtime acceptance checks are: open a Safari page and clicked link; share text with multiple links; successfully run each native action from Shortcuts; activate and clear an actual Focus and verify OS delivery and the restored profile. Browser launching and clipboard behavior require the main app running. A successful explicit Focus protocol test does not replace the final OS-delivery check.

Implementation follows Apple's [Safari context-menu](https://developer.apple.com/documentation/safariservices/adjusting-settings-for-contextual-menu-items), [toolbar](https://developer.apple.com/documentation/safariservices/adjusting-settings-for-a-toolbar-item), and [Focus filter](https://developer.apple.com/documentation/appintents/defining-your-app-s-focus-filter) APIs. The toolbar artwork is an original vector drawing included in this directory.
