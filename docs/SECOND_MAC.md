# Set up your other Mac

Use the same Apple Account with iCloud Drive enabled. This setup shares Finicky Plus routing settings. Chrome accounts, browser data, learning records and link history stay on each Mac.

## Install and connect

1. In Finder, open **iCloud Drive → Finicky Plus Setup** and let the setup archive finish downloading. Unzip it in a local folder.
2. Quit any existing Finicky router on that Mac. Keep its app and settings as a backup, then copy **Finicky Plus.app** into Applications and open it. The universal app supports Apple silicon and Intel Macs running macOS 13 or later. It is an Apple Development build, not a notarized public release; a macOS launch restriction may require local setup before it can run.
3. Open Finicky Plus settings and choose **Sync**. Turn on **Sync settings with iCloud**. Finicky reads the shared settings after iCloud delivers them.
4. Map **Work**, **Personal**, and **Academic** to their destinations on this Mac, then choose **Save profile mappings**. Use existing saved favorites or enter Google Chrome and the correct profile directory manually. No directory is inferred from a profile's name.
5. Wait until the status says **Up to date**, then use **Test a link** to check the routes. Set Finicky Plus as this Mac's default browser when the routes are correct.

To identify a Chrome profile directory, switch Chrome to that profile, open `chrome://version`, and find **Profile Path**. Enter only the final folder name, such as `Default` or `Profile 4`, into Finicky's mapping field. Repeat for each profile. [Chromium's profile-path documentation](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/user_data_dir.md) describes this check. The directory numbers on your two Macs can differ.

If a JavaScript configuration is active, sync pauses. Preserve the original configuration and move its rules into visual settings first; executable JavaScript is not shared. A temporary Focus override also pauses sync until it ends.

## Browser integration

The setup archive also contains **Chrome Extension**. Move that folder to a permanent local location before loading it through Chrome's Extensions page. Install it separately in each Chrome profile where you want the toolbar and Option-click integration. Extension permissions and enabled state do not sync through Finicky.

The Finicky Actions companion, Safari extension, Shortcuts and Focus permissions need separate setup on the other Mac. This archive does not transfer their permissions or claim those integrations have been tested there.

## Changes and conflicts

Saved routing rules, profile labels and supported preferences propagate while both apps run and iCloud delivers their files. Mappings remain specific to each Mac. Learning, history and logging preferences remain local.

If both Macs change settings before receiving one another's changes, Finicky asks which version to keep. Choose **Keep this Mac's settings**, or select a shared version and choose **Use shared settings**. This chooses an entire settings version; it does not combine individual rules. A backup is kept before local settings are replaced. Immutable shared versions also preserve the previous shared contents.

**Up to date** describes the revisions currently available on that Mac. It does not certify delivery to the other Mac. If the status says it is waiting for iCloud, let Finder download the **Finicky Plus Sync** folder and use **Sync now**. Routing continues using the current local settings when iCloud is unavailable.

Turning sync off leaves the current routing settings in place. Do not delete individual files from **Finicky Plus Sync** to fix a conflict; the revision history is needed to establish which changes are related.
