# Browser profile folder access

`Restore() error` resolves previously granted folder bookmarks without showing UI. Call it before browser profile discovery at startup. If one bookmark is invalid, expired, or revoked, other valid grants still restore and the function returns an explanation that can be shown or logged.

`Request() <-chan error` presents a native folder chooser on the main thread. The user chooses the browser data folder, for example `~/Library/Application Support/Google/Chrome`. The buffered channel receives one result: `nil` means the read grant was activated and its bookmark saved; cancellation or failure returns an explanatory error. A second simultaneous request reports that the chooser is already open.

Bookmarks live under the `FinickyPlusBrowserProfileBookmarks` key in the running app's standard UserDefaults domain, preserving the fork's bundle-identity isolation. They are read-only security-scoped bookmarks. Stale bookmarks are refreshed, each active folder scope is acquired at most once, and successful scopes are balanced when the app terminates. Read scopes remain active while the app runs because discovery and URL routing may need profile metadata at any time.

This package does not change Privacy & Security settings, choose a folder on the user's behalf, or grant access without the user's folder selection. It does not scan new arbitrary folders for browser identities; it grants access so the existing discovery code can read its known browser locations.

A future sandboxed router build must include the standard user-selected read-only and app-scoped bookmark entitlements. The current router is not sandboxed; when a scope is unnecessary, verified existing read access is accepted. A change in signing identity can invalidate saved grants and require the user to choose folders again.
