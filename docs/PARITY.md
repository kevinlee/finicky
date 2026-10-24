# Finicky Plus capability and verification matrix

Checked against the [Velja product documentation](https://sindresorhus.com/velja) on October 24, 2026. This is a capability comparison, not a claim of complete parity. “Implemented” means the source contains the behavior; it does not certify every installed browser, macOS version, or extension-registration path.

## Routing and destinations

| Capability | Velja baseline | Finicky Plus implementation and limits |
| --- | --- | --- |
| Website rules | Supported | Existing wildcard, regular-expression, and JavaScript matching; visual rules have enable/disable, ordering, and named destinations. |
| Source application rules | Supported | Bundle-ID conditions are available in visual rules and JavaScript. Source detection depends on what the launching app exposes. |
| Automatic browser discovery | Supported | Launch Services supplies installed browser names; link routers are excluded to prevent loops. |
| Browser profiles | Supported | Chromium discovery retains stable folder IDs, including duplicate display names. Explicit `profileDirectory` bypasses display-name lookup. Unresolved named profiles fail instead of silently opening another account. |
| Firefox / Zen profiles | Modern profiles supported | Reads both traditional `profiles.ini` entries and modern profile groups referenced by `StoreID`. Group databases are opened read-only; discovered profiles retain stable paths and launch with `--profile`. Synthetic SQLite fixtures verify renames, duplicate names, missing directories, and legacy fallback. Live profile switching is not yet verified. |
| Default / alternate destination | Supported | Browser/profile defaults and configurable held-modifier alternate destinations; saved profiles also appear in the menu. |
| Private browsing | Supported | Chromium and Firefox-family argument generation, including Edge's InPrivate flag. Unsupported browsers return an error. Actual behavior across every derivative/version remains unverified. |
| New windows | Supported | Chromium and Firefox-family launch flags. Firefox private launches use its private-window operation rather than stacking competing window flags. |
| Native application handoff | Supported for selected services | **Partial:** independent service toggles, installed-app checks, and browser fallback for Zoom, Teams, Spotify, Figma, Linear, ClickUp, Google Meet in Chrome, Slack URLs containing explicit workspace IDs, and Mastodon post/profile links on `mastodon.social` or `mastodon.online` in Ivory. Explicit read-only Obsidian URIs work after a JavaScript rewrite. No built-in Notion adapter or arbitrary Slack workspace inference. |
| Chromium PWAs | Supported with setup | An installed Chromium `.app` wrapper selected by absolute or `~/` path opens through its parent browser using the recorded app ID and profile. Shared wrappers require an explicit profile directory. Conflicting profile/private flags and nonstandard data roots fail explicitly. Fixture-tested; live opening remains unverified. No dedicated PWA discovery screen. |

Implementation: [browser launcher](../apps/finicky/src/browser/launcher.go), [profile discovery](../apps/finicky/src/browser/profiles.go), [native adapters](../apps/finicky/src/linkfeatures/native.go), [visual rule conversion](../apps/finicky/src/rules/rules.go), [routing bridge](../apps/finicky/src/features.go).

## Picker and link handling

| Capability | Velja baseline | Finicky Plus implementation and limits |
| --- | --- | --- |
| Browser prompt | Supported | Native queued panel with a scrollable destination list and application icons; never/always/unmatched settings, explicit prompt rules, and modifier trigger. |
| Keyboard selection | Supported | Arrow/Tab navigation, Return/Space, Escape, numbers, and custom letters. |
| Remember website | Supported | Picker checkbox and Command-click create a domain rule after destination validation. |
| Background opening | Supported | Picker control, Control-click, and native context-menu actions request background launch; a browser may still activate itself. |
| Destination context menu | Supported | Native right-click actions open normally, in background, in a new/private window, or remember the website. Unsupported mode requests return an error. |
| Copy / share | Supported | Picker terminal actions and encoded URL requests. Sharing uses the native macOS share picker. |
| Saved destinations | Supported | Profile names/colors, visibility, and custom prompt shortcuts; underlying browser storage is not changed. |
| Tracking cleanup | Supported | **Partial:** `utm_*` plus a conservative list of common tracking keys. Unknown query fields, ordering, and encoding are retained. This is not an exhaustive tracker-removal database. |
| Redirect unwrapping | Supported | **Partial:** recognized Google, Meta, Outlook Safe Links, Slack redirector, YouTube, LinkedIn, and DuckDuckGo wrappers. Does not authenticate to protected redirects or execute redirect-page JavaScript. |
| Short URLs | Supported | Existing domain list, optional expansion, and user-added domains; bounded redirects and request timeouts. Domain checks respect hostname boundaries. |
| Global URL transforms | Supported | Visual settings contain a JavaScript editor, and existing configuration `rewrite` callbacks remain available. Transformations run after enabled cleanup and before matching. |
| Per-rule transforms | Supported | Visual rules contain a JavaScript transformation editor. Existing handler `url` transforms remain available; these run after the matching rule is selected. |
| Link history | Supported | Optional local list, search, original URL display, reopen through picker, and clear. Newest 200 records retained. Private launches are omitted from this history. |
| Logs / routing test | Supported | Existing diagnostics and test view, with browser/profile output. Disk logs are separately opt-in and may contain URL/request data, including private-mode requests. Turning file logging off closes its file immediately. |

Implementation: [native picker](../apps/finicky/src/picker), [cleanup pipeline](../apps/finicky/src/linkfeatures/prepare.go), [shortener resolver](../apps/finicky/src/shorturl/resolver.go), [history store](../apps/finicky/src/linkfeatures/history.go), [logger](../apps/finicky/src/logger/logger.go), [settings UI](../packages/finicky-ui/src/components/FeaturePreferences.svelte).

## macOS integration

| Capability | Velja baseline | Finicky Plus implementation and limits |
| --- | --- | --- |
| Clipboard / multiple URLs | Supported | Main-app menu, selected-text Services, and companion actions dispatch distinct URLs through the router. |
| Menu launch / default selection | Supported | Saved destinations can launch their browser/profile or set the router's default, including private-mode defaults. |
| Menu icon | Supported | Finicky, selected default-browser icon, globe, and arrow choices. |
| Browser profile access | Supported | User-triggered native folder picker grants access and saves read-only security-scoped bookmarks for startup restoration. The access-grant flow has not been exercised on this Mac. |
| URL scheme / scripting | Supported | `finicky-plus://` actions support opening, prompting, copying, sharing, clipboard, settings, and default changes. The command line and JavaScript configuration remain available. |
| Shortcuts actions | Supported | Finicky Actions companion provides five App Intents. Universal build, deep signature check, and metadata for all five actions are verified. The companion window was launched and visually checked; action registration and execution still need runtime acceptance testing. |
| Focus filter | Supported | Companion extension sends active/clear requests; main app persists/restores the previous default. Build-verified, but Focus activation/deactivation has not been exercised on this Mac. |
| Browser extensions | Supported | Existing Chromium/Firefox extension source plus new Safari companion extension. Safari toolbar/current-page and clicked-link actions are implemented; enabling and runtime testing remain necessary. |
| Share extension | Supported | Companion extension handles explicitly shared URLs or text, with deduplication. Built and signature-verified; host Share-menu registration is setup-dependent. |
| Handoff | Supported | Existing browsing-activity handler routes received web activities. No new cross-device runtime verification. |
| Import / export | Supported | JSON settings import/export in the visual interface; imports use validation and the normal Apply path. Rules saves retain an atomic backup. JavaScript configuration is a separate file. |

See [Finicky Actions setup and verification](../apps/integrations/README.md). The companion is separate from the router. Its local build uses ad hoc signing unless a signing identity is supplied; that is not notarization or proof that the operating system has enabled the extensions. Ordinary browser navigation stays inside that browser unless an extension action or explicit redirect sends the URL to Finicky Plus.

## Remaining differences

- Browser-specific runtime coverage needs expansion. Modern Firefox/Zen groups are implemented and fixture-tested, not live-tested. Browser account-avatar extraction is absent. Brave Origin Beta metadata has not been verified; supported identifiers and data paths are listed in [browsers.json](../apps/finicky/src/browser/browsers.json).
- Native adapters remain a bounded set, with per-service enable/disable choices. Notion is not built in. Mastodon coverage is limited to two known servers and Ivory. Custom rules can target an installed app or rewrite a URL for other services. Slack workspace inference and Webex meeting conversion are also unsupported by the documented Velja baseline, so these are shared limitations rather than unique parity gaps.
- No dedicated PWA discovery screen or specialized Safari web-app adapter. Chromium wrapper launching is implemented with documented limits; selecting arbitrary installed apps and visual URL transformations are available.
- No automatic translation of Obsidian Publish HTTPS links into a local vault: the URL does not provide the required vault identity. Top-level input currently accepts web/file URLs; the Obsidian native adapter is useful for an explicit JavaScript rewrite.
- Music-link normalization is not implemented. The [Songlink/Odesli API notice](https://linktree.notion.site/API-d0ebe08a5e304a55928405eb682f6741) says its public API retired July 31, 2026. The retired endpoint must not be presented as a working integration; public landing pages are a separate service.
- Safari/Arc profile targeting and automatic Firefox container selection are shared limitations with Velja, rather than unique missing features. Dia profile targeting is also absent here. Safari private mode is not built in to either app; Velja documents an external helper. This launcher rejects unsupported private-mode requests.
- There is no App Store release, notarized distribution, automatic fork-update channel, or full-app sandbox conversion in this change.

## Storage and validation

The fork isolates visual settings and history in `~/Library/Application Support/Finicky Plus/` and disk logs in `~/Library/Logs/Finicky Plus/`. Upstream Finicky's corresponding directories are left intact. The existing JavaScript configuration remains independently selectable and can retain higher-priority routing logic. Import/export covers the JSON visual model, not the contents of that JavaScript file.

Automated checks cover modern Firefox group databases, per-service native filtering, launch argument ordering, private/new-window flags, rejected router loops, stable/ambiguous profile handling, URL cleanup preservation, redirect host boundaries, response-body closure, installed-app fallback, history ordering/capping/concurrent writes, feature serialization, and logger enable/disable/concurrency. The browser, rules, shorturl, linkfeatures, and logger package suites pass with Go's race detector.

Those checks do not launch every browser or verify actual meeting joins, profile login state, native-service authentication, extension activation, or cross-device Handoff. Runtime acceptance results belong in the release notes when exercised, rather than being inferred from a successful compile.

## Implementation evidence

- Firefox group storage and launch behavior follow Mozilla's [profile datastore](https://github.com/mozilla-firefox/firefox/blob/main/toolkit/profile/ProfilesDatastoreService.sys.mjs) and [selectable-profile service](https://github.com/mozilla-firefox/firefox/blob/main/browser/components/profiles/SelectableProfileService.sys.mjs). The implementation reads only group IDs referenced by the browser's own `profiles.ini`.
- Browser variants use [Chromium's macOS flavor definitions](https://chromium.googlesource.com/chromium/src/+/main/docs/mac/flavors_of_chrome.md), [Apple's maintained browser metadata](https://github.com/apple/password-manager-resources/blob/main/quirks/web-browser-extension-distribution-information.json), [Brave Origin branding](https://github.com/brave/brave-core/blob/master/app/theme/brave_origin/BRANDING) and [profile-directory construction](https://github.com/brave/brave-core/blob/master/build/config.gni), and Thorium's [branding](https://github.com/Alex313031/thorium/blob/main/src/chrome/app/theme/chromium/BRANDING) and [macOS data path](https://github.com/Alex313031/thorium/blob/main/src/chrome/common/chrome_paths_mac.mm). Read-only discovery of a browser does not prove its launch flags have been exercised on that version.
- Slack conversion follows the [official deep-link ID requirements](https://api.slack.com/reference/deep-linking) and [workspace URL structure](https://slack.com/help/articles/221769328-Locate-your-Slack-URL-or-ID). Links lacking required IDs stay in the browser.
- Ivory conversion follows Tapbots' [URL-scheme documentation](https://tapbots.com/support/ivory/tips/urlschemes); its [product page](https://tapbots.com/ivory/) describes the Mac app's shared feature set. Server recognition is intentionally limited, with no background probing of arbitrary sites.
- Other adapter references include [Linear desktop links](https://linear.app/docs/get-the-app), [Obsidian URI operations](https://help.obsidian.md/Extending+Obsidian/Obsidian+URI), [Finicky's application recipes](https://github.com/johnste/finicky/wiki/Configuration-ideas), and Velja's documented ClickUp conversion. [Notion's help](https://www.notion.com/help/notion-for-desktop) confirms a desktop-link preference but does not establish a tested adapter in this fork.

The companion's native tests cover query encoding, profile-directory keys, URL extraction/deduplication, rejection of non-web URLs before dispatch, and per-link private/new-window/background options. Metadata extraction verifies five App Intent types plus the Focus type. Browser-access and picker packages pass race tests; actual profile grants and Safari/Share/Focus activation remain unexercised. PWA tests verify XML/binary metadata, parent-browser dispatch, profile safety, mode/custom-argument preservation, and avoiding a duplicate ordinary tab. See [PWA support](../apps/finicky/src/browser/PWA.md) for primary Chromium sources and the exact supported wrapper layout; no real installed PWA was opened during validation.
