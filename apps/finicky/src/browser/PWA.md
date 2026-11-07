## Installed Chromium web apps

A favorite or rule can use an absolute or `~/` path to an installed Chromium web
app's `.app` bundle. The launcher reads its Info.plist with Foundation (XML and binary
formats) and uses the installed app ID, parent browser and profile to open the
requested web URL in that app. It sends no additional ordinary-tab URL.

Profile-specific wrappers use their recorded profile automatically. A modern
wrapper shared across profiles must have an explicit profile directory selected
in the favorite or rule; it cannot safely infer which account the user intends.
An explicit profile that conflicts with a profile-specific wrapper is rejected.

The parent must be a Chromium browser in `browsers.json`. Private mode is rejected
because installed web apps do not provide an incognito app mode. Background and
new-window requests and non-conflicting custom flags are passed to the parent.
Custom arguments that replace the app/profile, request private/guest mode, or add
other positional URLs are rejected. The browser ultimately controls window reuse
and whether a requested URL is within the web app's scope.

Nonstandard browser user-data directories are rejected rather than opening a
different profile. Automatic installed-web-app discovery, Safari web-app
integration, and creating new web apps are outside this adapter.

Implementation follows Chromium's documented [bundle metadata keys](https://chromium.googlesource.com/chromium/src/+/lkgr/chrome/common/mac/app_mode_common.mm),
[app-data path and multi-profile wrapper layout](https://github.com/chromium/chromium/blob/main/chrome/browser/web_applications/os_integration/mac/web_app_shortcut_creator.mm),
and [app launch switches](https://github.com/chromium/chromium/blob/main/chrome/common/chrome_switches.h).
Tests cover metadata formats, launcher dispatch, profile safety, mode/argument
preservation and avoiding a duplicate browser tab. Live opening inside every
Chromium variant still needs browser-specific integration testing.
