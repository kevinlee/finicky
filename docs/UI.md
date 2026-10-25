# Settings UI foundation

Waystile's settings window uses a shared visual and interaction foundation while retaining the existing native routing and settings bridge. The public product name is Waystile; legacy protocol and storage names remain stable for compatibility.

## Libraries and local assets

- **Radix Colors 3.0.0** supplies Slate, Indigo, Green, Amber, and Red scales in both light and dark variants. `packages/finicky-ui/src/tokens.css` maps them to roles such as surface, primary text, secondary text, border, accent, success, warning, and danger. Components use those roles instead of defining their own palettes. This follows the scale's intended separation between backgrounds, controls, borders, and text. See [installation](https://www.radix-ui.com/colors/docs/overview/installation) and [scale roles](https://www.radix-ui.com/colors/docs/palette-composition/understanding-the-scale).
- **Lucide for Svelte 1.61.0** supplies consistently drawn icons through `StudioIcon.svelte`. The adapter imports named components and preserves the existing `name` and `size` interface. The package supports tree-shaking so unused icon components can be omitted from the production output. See [Lucide for Svelte](https://lucide.dev/guide/svelte).
- Both packages are pinned in `package.json` and the npm lockfile. Their CSS and icon components are bundled with the application. The settings UI does not fetch these libraries, fonts, or icons from a CDN at runtime.
- Standard HTML buttons, selects, inputs, and the native HTML dialog remain the interaction primitives. This update does not introduce Bits UI, Tailwind, or a styled component framework.
- Waystile's original route-mark icon lives in `design/waystile-icon.svg`. The native generator in `scripts/create-waystile-icons.swift` renders matching app, menu-bar, and browser-extension assets. Run `scripts/create-icons.sh` to regenerate them; the settings header and About page bundle the SVG directly.

The complete upstream license texts are included in `apps/finicky/assets/UI-THIRD-PARTY-NOTICES.txt`, including Lucide's Feather-derived icon notice. The build places this file and Finicky's original MIT license in the app bundle's `Contents/Resources` directory. `scripts/build-fork.sh` creates a fresh `Waystile.app` bundle so obsolete resources are not retained from an earlier build.

## Visual rules

The system font stack is used for interface text; monospace is reserved for technical values. The shared scale uses a 14px base, 12–13px secondary text, and limited 11px metadata. Headings establish hierarchy without repeated branding or decorative header eyebrows. These are project design choices, not a claim about an Apple-mandated minimum size.

Spacing follows a four-pixel base with shared gutter and radius tokens. The desktop sidebar groups daily routing tasks separately from settings, while Diagnostics and About have their own labeled navigation landmark. Every destination retains a visible label. The sidebar can scroll at the smallest supported window height.

Routing shows a compact profile summary above the ordered rules; the Profiles page retains the fuller editing cards. Rule rows keep the source, destination, and actions together. Off rules have a text badge as well as a switch state. Secondary pages share the same surfaces, text roles, form styles, and status treatments.

`theme.ts` follows system appearance, including changes while the window is open. Light and dark modes use the corresponding Radix scales and native `color-scheme` setting. Focus outlines remain visible, and reduced-motion preferences suppress nonessential animation.

## Saving and feedback

Routing rules, profile shortcuts, and preferences are drafts until **Apply changes** succeeds. The footer appears on those editing pages and remains visible on any page when a global draft is unapplied, saving, invalid, or affected by an external change. A clean Sync, Learning, History, or Integrations page does not show a misleading Apply footer. Test a link evaluates applied native settings rather than the unapplied draft.

Sync controls and learning actions keep their existing immediate-save contracts. Sync profile mappings have their own Save and Discard actions. Sidebar and home navigation are blocked with an explanation while those mapping edits are unsaved. Editing a manual browser name preserves the entered profile fields; saving selects the appropriate field for the final browser.

Errors and warnings remain visible until dismissed unless a caller explicitly supplies a duration. Success and informational notifications are transient. Notifications have a dismiss button; technical details are available in a disclosure rather than dominating the message. Errors use assertive alerts, while ordinary status messages use polite announcements.

## Keyboard and accessible structure

- **Command-S** invokes the same guarded Apply action as the button. It does not submit while a dialog is open, during a conflicting operation, or when settings are invalid or unchanged.
- Dialogs are named by their visible heading. Editing dialogs focus the first relevant field; destructive confirmations initially focus Cancel. Escape closes the dialog, and focus returns to its trigger or the page heading if the trigger no longer exists.
- Rule rows have stable UI-only identities that are never serialized into settings. Reordering restores focus to a move control on the same rule, including at the first and last positions, and announces its new position politely.
- Rule switches use a stable label, with `aria-checked` carrying state. Navigation exposes the current page. Mapping inputs associate visible validation messages with the relevant field.
- Rule edit and delete confirmations check that their original target still exists. Profile deletion checks the original saved profile. If an external update changes the target, the operation stops with an explanation rather than applying to a different item.

These improvements are not a WCAG or macOS Human Interface Guidelines certification. A complete VoiceOver walkthrough and testing on the oldest supported macOS/WebKit version remain separate acceptance work.

## Cross-model review and verification

Independent **Claude Opus 5.5** and **Gemini 4 Argon High** UI reviews completed through Cursor, followed by a Claude reconciliation. The exact requested selections were `claude-opus-5-5-medium` and `gemini-4-argon-high`. Cursor reported `Claude Opus 5.5 300K Medium` and `Gemini 4 Argon High`; those are observed CLI labels, not independent verification of the providers' backend identities or context-window sizes.

The reviewers received the same frozen text brief, source excerpts, and written UI observations. They did not receive screenshot pixels, operate the application, or independently measure contrast. Their agreement identified useful priorities—save semantics, keyboard focus, typography, contrast, and hierarchy—but did not itself establish correctness. Findings were checked against the complete source before implementation. The suggested manual-mapping data-loss issue was confirmed and addressed with focused model tests.

After the final shell corrections, Svelte/TypeScript checks reported no errors and all 51 settings model tests passed. The existing warning in the inactive legacy `Rules.svelte` component remains. Those tests do not replace rendered UI, keyboard, screen-reader, or installed-app checks. Release-specific runtime evidence and remaining limitations belong in [VERIFICATION.md](VERIFICATION.md).

For future work, extend the semantic tokens and shared controls before adding selector-specific overrides. Use synthetic preview fixtures for destructive or conflicting states, preserve the existing settings serialization contract, and keep real user settings and iCloud revisions out of visual QA.
