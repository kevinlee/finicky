# Set up Waystile on another Mac

Both Macs need the same Apple Account with iCloud Drive enabled. Waystile shares visual routing settings; browser accounts, browser data, learning observations, and link history stay on each Mac.

## Install and connect

1. Install the same supported Waystile release on both Macs. Quit an existing Waystile or Finicky Plus copy before replacing it, and preserve the old app and settings as a backup. See the main [installation guide](../README.md#install) for release and signing requirements.
2. Open Waystile settings and choose **Sync**. Turn on **Sync settings with iCloud**.
3. Map each shared profile label to its actual browser destination on this Mac, then choose **Save profile mappings**. Use an existing saved destination or enter the browser and profile directory manually. A directory is never inferred from a profile's label.
4. Wait for **Up to date**, then use **Test a link** to check the routes. Set Waystile as this Mac's default browser when those routes are correct.

To identify a Chrome profile directory, switch Chrome to that profile, open `chrome://version`, and find **Profile Path**. Enter only the final folder name, such as `Default` or `Profile 4`, in the mapping field. Repeat for each profile. [Chromium documents this profile-path check](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/user_data_dir.md). Directory numbers on two Macs can differ.

If a JavaScript configuration is active, sync pauses. Preserve the original configuration and migrate its rules into visual settings first; executable JavaScript is not shared. A temporary Focus override also pauses sync until it ends.

## Browser and system integration

Install the browser add-on separately in each browser profile where you want its toolbar or Option-click integration. Keep an unpacked extension in a permanent local folder. Extension permissions and enabled state do not sync through Waystile.

The Waystile Actions companion, Safari extension, Shortcuts, and Focus permissions also need separate setup. Copying the router or its settings does not transfer those permissions. See the [companion guide](../apps/integrations/README.md).

## Changes and conflicts

Saved rules, profile labels, and supported preferences propagate while the apps run and iCloud delivers their files. Destination mappings remain specific to each Mac. Learning, history, and logging preferences remain local.

If both Macs change settings before receiving one another's changes, Waystile asks which version to keep. Choose **Keep this Mac's settings**, or select a shared version and choose **Use shared settings**. This chooses an entire settings version; it does not combine individual rules. A backup is kept before local settings are replaced. Immutable shared versions preserve previous shared contents too.

**Up to date** describes revisions currently available on that Mac. It does not certify delivery to another Mac. If the status says it is waiting for iCloud, let Finder download the **Finicky Plus Sync** folder and use **Sync now**. This folder name is deliberately retained for compatibility with earlier installations. Routing continues using current local settings when iCloud is unavailable.

Turning sync off leaves the current routing settings in place. Do not delete individual files from **Finicky Plus Sync** to resolve a conflict; that revision history establishes which changes are related.
