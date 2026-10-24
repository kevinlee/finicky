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

These are implemented behaviors, not a claim that every integration executes on the tested Mac. With Apple Development signing, the Settings, Open URLs, and Set Default Browser Profile Shortcuts actions, Safari toolbar/context-menu actions, and inbound Safari page-URL sharing now pass live checks. Sharing selected text containing URLs does not offer Finicky in TextEdit or Safari. Open Clipboard, Copy Clean URL, and OS delivery of Focus changes remain unverified. See the runtime results below. The extensions do not intercept ordinary navigation. Safari's content script only reads the link that the user right-clicked. The Share extension reads only items explicitly shared with it.

## Build

Requires Xcode with the macOS 13 SDK or newer and [XcodeGen](https://github.com/yonaskolb/XcodeGen). The companion's deployment target is macOS 13.

From the repository root:

```sh
scripts/build-integrations.sh
```

The script generates the Xcode project, performs a clean build of the companion and all three embedded extensions, verifies their signatures, and copies the result to `dist/integrations/Finicky Actions.app`. It does not install the app or enable extensions. The full Release build passed after correcting empty-array expansion under `nounset` on the system Bash 3.2. Cleaning is necessary when changing signing identities: an incremental signed build retained an ad hoc Safari extension during QA and failed verification; a clean build succeeded. The verifier checks the embedded signatures and matching signing team, with all 16 verifier fixtures passing. Signed Release build 3 and all seven native tests also pass.

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

### Signed Shortcuts runtime results

During the October 24, 2026 signed QA pass, an Apple Development identity was created through Xcode without revoking either existing certificate. The tested Finicky Plus `d6ae002` artifact was re-signed without rebuilding; companion build 2 was clean-rebuilt from `c16feeb`. Both were installed with matching Apple signing identities, and the companion and all three embedded extensions verify under the same team.

**Open Finicky Plus Settings** now renders the actual settings window both when the companion is running and when Shortcuts must start it. **Open URLs with Finicky Plus** sent a synthetic `example.com` URL into the picker with its tracking parameter removed; cancelling succeeded. **Set Default Browser Profile** saved Google Chrome / `Profile 2` with private mode on, then a second invocation with `Default` and private mode off restored effective Personal/nonprivate routing. Saving normalizes omitted false fields, so after verifying the effective values the original settings file was restored byte for byte. Temporary QA shortcuts were deleted and the original library count restored. Open Clipboard and Copy Clean URL have not yet been exercised as signed native actions.

The earlier ad hoc build listed all five actions but failed Settings before execution with `LNConnectionErrorDomain` 1200 and an unavailable Team ID. Registration cleanup and restart did not resolve it; the clean Apple Development-signed build did. An ad hoc signature's integrity check and action metadata therefore did not establish runtime success on this Mac. Apple's [code-identity guidance](https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements) explains why an ad hoc seal is not a persistent developer identity.

Keep only the intended installed app registered with Launch Services and restart the companion after replacing its binary. Development builds in `dist` and DerivedData can otherwise compete under the same bundle identifier. Do not delete Shortcuts databases or add Apple's private entitlements as a workaround.

The earlier built-in **URL** → **Open URLs** custom-scheme workaround failed before signing: direct `finicky-plus://settings` input and a typed URL → Open URLs sequence both stalled until cancelled, while an HTTP(S) control opened Chrome Personal. Those custom-scheme attempts did not show the native App Intent's error 1200 and have not been retested after signing, so no working custom-scheme fallback is claimed. The now-successful signed native actions are a separate result.

### Share and Focus runtime results

Finicky Plus remains the re-signed `d6ae002` artifact. Installed companion build 3 includes the `c16feeb` changes plus the Share display-name, icon, and regression-test corrections. It retains activation dictionary version 2 and matching Apple Development signatures. Fresh AppKit enumeration now includes Open with Finicky Plus with a title, icon, and eligible service result. After restarting Safari while preserving its original Start Page and closing only QA content, Safari's actual page Share menu also listed the extension. Selecting it reached the main picker with the correct cleaned page URL and no error.

Selected-text sharing has a separate live failure: Finicky is missing from TextEdit's native Share menu for both single-URL and multiple-URL text, with unsaved and saved documents and after restarting the host. Safari also omits it when sharing a text selection from a local synthetic HTML page. Fresh enumeration using the deprecated AppKit API accepts String and attributed-string inputs, but the modern host menus omit the extension. This is not a TextEdit-only limitation, and its cause is unresolved; no sandbox-filter explanation is established.

Version 2 fixes a concrete activation gap: the default version 1 can hide an extension when a host offers supported URL/text and unsupported representations together. [Apple documents this matching difference](https://developer.apple.com/library/archive/documentation/General/Reference/InfoPlistKeyReference/Articles/AppExtensionKeys.html). Build 2 still failed Share-menu discovery even after Apple Development signing; build 3's display-name/icon correction resolved the tested page-sharing flow. The seven native tests include activation-policy and presentation-metadata regression checks. Share discovery was a separate issue from the native Settings action connection failure.

The user manually enabled the signed Safari extension after an automated activation click was blocked. Only `example.com` received Allow for One Day; no all-sites or private-browsing access was granted. The toolbar action opened the picker with the current page's correct cleaned URL. Right-clicking Learn more → Open Link with Finicky Plus opened the picker for exactly `https://iana.org/help/example-domains`. Both pickers were cancelled without navigation. The extension remains enabled with the user's approval; no unsigned-extension protection was disabled.

System Settings listed **Finicky Actions → Browser Profile** when adding a filter to Work Focus, and its configuration UI opened. The flow was cancelled without editing that existing Focus. Separately, an explicit protocol activation selected Work/private and persisted the previous Personal default. After restarting Finicky Plus, explicit clear restored regular Personal and removed the snapshot. The original main-app settings were restored byte for byte and the snapshot was confirmed absent after the continuation. No actual Focus was activated or changed during QA. This verifies discovery and the router's restoration protocol, **not OS delivery** when a Focus activates or deactivates.

## Enable on your Mac

1. Install and open Finicky Plus once so macOS registers its URL scheme.
2. Move `Finicky Actions.app` to Applications and open it once. Keep the companion installed; its embedded extensions and Shortcuts metadata are registered through this bundle.
3. In Shortcuts, search for **Finicky Actions** to add actions. Use the browser's exact profile name or Chromium directory when specifying a profile.
4. With a properly signed build, open Safari Settings → Extensions and enable **Finicky Safari**, then grant access to the sites where you want its contextual link action. On the tested Mac, the user enabled it manually and the toolbar and clicked-link actions passed with only `example.com` Allow for One Day access.
5. In macOS Extensions settings, enable **Open with Finicky Plus**. The exact location varies by macOS version. Enabling alone is not acceptance; an actual inbound share must also succeed. Signed build 3 passed Safari page sharing on the tested Mac.
6. In System Settings → Focus, add the **Browser Profile** app filter and enter the browser and optional profile. The separate App Intents extension lets the system deliver changes while the companion is closed.

Focus filter delivery and extension registration are controlled by macOS. A successful build does not confirm execution. Set Default Browser Profile now works as a signed native action, but an actual Focus automation using it remains unverified. No working built-in custom-scheme shortcut fallback is claimed.

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

The main app owns the previous-default snapshot for Focus restoration. A successful companion action confirms dispatch to macOS, not that the selected browser finished opening or clipboard processing completed. The signed Settings and Open URLs live tests additionally observed the expected settings window and picker.

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

The seven unit tests cover query-value isolation, the profile-directory protocol key, URL extraction/deduplication, all-or-nothing web-URL validation, per-link opening options, Share activation policy, and presentation metadata. Remaining runtime acceptance checks are: resolve selected-text Share-menu omission and verify multiple-link text dispatch; run Open Clipboard and Copy Clean URL from Shortcuts; activate and clear an actual Focus and verify OS delivery and the restored profile; and verify cross-device Handoff. Browser launching and clipboard behavior require the main app running. A successful explicit Focus protocol test does not replace the final OS-delivery check.

Implementation follows Apple's [Safari context-menu](https://developer.apple.com/documentation/safariservices/adjusting-settings-for-contextual-menu-items), [toolbar](https://developer.apple.com/documentation/safariservices/adjusting-settings-for-a-toolbar-item), and [Focus filter](https://developer.apple.com/documentation/appintents/defining-your-app-s-focus-filter) APIs. The toolbar artwork is an original vector drawing included in this directory.
