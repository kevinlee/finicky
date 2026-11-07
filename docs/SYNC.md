# Settings-only iCloud sync

Settings sync is optional and starts off. Its revision engine only exchanges
portable settings. Link routing, local history, the learning dataset, browser
data, and JavaScript configuration files are never uploaded. Cloud work runs in
the background, outside the routing path. Offline or incomplete sync leaves the
currently saved local configuration active.

## Shared and local settings

The shared schema includes saved profile IDs and their names, colors, shortcuts,
hidden/private flags; ordered routing rules and source-app matches; default and
alternative targets; chooser, link-cleanup, short-link, native-app and menu
preferences; and Keep Running/Hide Icon.

Concrete browser profiles are represented by the saved profile's stable logical
ID. Browser, local profile name, and profile directory are machine-local
`Binding` values. Generic browser targets use canonical bundle identifiers with
no local path or profile. A concrete rule/default/alternative target must match
exactly one saved profile before it can be published. Duplicate logical IDs or
ambiguous targets stop publication instead of choosing a profile implicitly.

Request logging, history collection, learning collection, and update-checking
preferences stay local. Incoming settings preserve these local values. No stored
URLs, learning labels, cookies, browser sessions, passwords, or history files are
included. Rules may themselves contain site names and URL patterns the user chose
to sync; settings sync is not a promise that rule text contains no sensitive data.
Device provenance is a random sync ID, not the Mac's hostname or account name.

Transform scripts are excluded entirely. A configuration containing a global or
per-rule transform script cannot be published automatically. Incoming revisions
have a strict schema without executable-script fields. The application also
pauses sync when its JavaScript configuration or temporary Focus routing is
active; these app states must not be mistaken for shared settings changes.

## First use and profile mappings

The first device, when the cloud history is empty, publishes its current portable
configuration and retains its already selected local profile bindings. A joining
device must explicitly map every shared profile ID to its own browser/profile.
No directory names are guessed or copied from another Mac. Missing mappings keep
existing routing active, and no partial configuration is proposed.

Bindings validate canonical browser identities, bounded profile names, single
directory basenames, browser support for directory targets, and requested private
or new-window modes. The application's mapping UI should offer the locally
discovered browsers/profiles; the core does not create profiles, inspect their
contents, or treat syntax validation as proof an account is signed in.

A display rename retains the same logical profile ID. Ordinary edits to a saved
favorite's local browser/directory update its local binding when the explicit sync
mapping still matches the last applied binding. Conversely, an explicit sync
mapping change can propose that new local target. If both independently changed
to different targets, the engine reports a conflict instead of reverting either.
This comparison uses the last actually applied concrete bindings, never a guess
from a joining Mac's similarly named profiles.

The mapping UI calls `SetBindingsFor(current, bindings)`, supplying the exact
current configuration observed for that explicit action. This establishes the
ordering even if an ordinary favorite edit has not yet been polled, and honors
an explicit choice to return to an earlier directory. `Status` stays read-only.
Existing local portable edits are retained while applying a binding choice; when
the remote head is unchanged they can be published first, followed by the local
mapping proposal. Changes on both devices still require a recoverable conflict
choice. Local mapping conflicts retain profile rows and bindings even if iCloud
is unavailable, so the user can correct the mapping directly.

## Revision protocol and conflicts

The cloud folder contains `revisions/<random-id>.json`. Each immutable revision
has a schema version, random revision and device IDs, parent revision IDs, a
display timestamp, and portable settings. There is no mutable head file.
Revision IDs must match filenames; malformed, unsupported, duplicate-looking,
oversized, cyclic, or unknown-field revisions stop processing. An already applied
revision whose settings change in place is also rejected.

Heads are computed from graph ancestry. Missing parents remain pending until
they arrive, including when iCloud delivers child files first. Previously applied
history disappearing does not reset the engine or republish a new root. Multiple
heads are a conflict. Timestamps never select a winner, even when one device's
clock is newer or incorrect.

For a single complete head, an unchanged local portable baseline permits an
incoming proposal. A local edit with an unchanged remote head publishes a child
revision. Changes on both sides require an explicit choice unless the resulting
portable content is already identical. Publishing an explicit local winner joins
all currently known heads without deleting any losing revision.

An explicit remote choice must name a current head. A sole head may be selected
only after the user asks to use shared settings; multiple heads require a visible
choice. For multiple heads, `ResolveRemote` prepares a merge locally. Only after
the application successfully compare-and-saves the proposed configuration does
`Acknowledge` publish that merge, parenting all the selected snapshot's heads.
If the heads changed in the meantime, acknowledgment reports the unresolved
conflict rather than publishing a stale resolution. The local explicit choice
may already have been saved; the application's backup and conflict status allow
the user to inspect and resolve that situation.

## Coordinated storage and hydration

On macOS, cloud revision listing, reading, directory creation, and immutable
publication use `NSFileCoordinator`. Revision writes use a private temporary file,
sync it, and move it into place without replacing an existing revision. Existing
revision files are never overwritten or deleted by the engine.

An iCloud placeholder or not-downloaded item yields a pending state. The engine
requests downloads only for its settings directory or correctly named revision
placeholders, at most eight requests per directory scan, with a 30-second cooldown
per URL. Other iCloud content is not downloaded. Unknown placeholder names,
duplicate-looking files, and iCloud unresolved-conflict metadata block automatic
processing. Real download completion depends on iCloud and connectivity; a pending
status is not reported as successful synchronization.

The caller supplies a dedicated existing iCloud parent. Enable may create only
the app's immediate folder and `revisions` subfolder, not missing parent trees.
If the initial Enable is offline or pending, subsequent background steps resume
that already-authorized setup when the parent becomes available; no toggle is
needed. Discovering an existing history takes the explicit-mapping join path.
A local marker records history discovered during setup or background processing,
even before its first application, preventing later disappearance from triggering
a new root revision. Once that history was recorded or applied, polling and
re-enabling never recreate its missing directories or silently reset it.
Disabled operations and `LocalStatus` do not read or create cloud directories.
The local state file uses a dedicated mode-0700 directory and mode-0600 atomic
replacement. It stores opt-in state, device ID, local bindings and their applied
baseline, the last applied portable revision, and any pending proposal.

Each revision is limited to 1 MiB, with at most 100 profiles and 500 rules. The
history is limited to 512 revisions; directory scans are bounded and publication
stops at the limit. These are operational bounds. This version has no automatic
garbage collection or history deletion. A user-controlled archival/reset procedure
must coordinate all devices before starting a fresh history. Deleting arbitrary
parent files is not a supported cleanup method.

## Integration contract and validation

`New(localDir, cloudDir)` creates an engine without accessing cloud storage.
`Enable`, `Disable`, `Status`, `LocalStatus`, `SetBindingsFor`, and `Step` manage its
state. `Step` returns an optional `Update{Revision, Rules}`; it never applies live
routing. The application serializes sync operations, compare-and-saves against
the exact configuration it checked, maintains its backup, updates runtime state,
then calls `Acknowledge`. A failed comparison must not be acknowledged. Ordinary
acknowledgment is local-only; remote conflict acknowledgment may publish its
prepared merge and therefore must also run away from the routing thread.
`SetBindings(bindings)` remains available for initial joining or callers that
have no intervening local favorite edits; interactive mapping changes use
`SetBindingsFor(current, bindings)` to capture their explicit observation cutoff.

`Publish(current, true)` is explicit keep-this-Mac resolution;
`ResolveRemote(current, revision)` is explicit use-selected-shared resolution.
Neither operation is a timestamp-based automatic conflict policy.

Package validation uses two engines with temporary local/cloud folders and the
real macOS coordination wrapper: `go test -race ./settingssync`. Tests cover
propagation, explicit mapping, local collection preferences, concurrent heads,
dirty-local conflicts, acknowledgment ordering, ordinary profile edits, delayed
parents, placeholders, malformed data, immutable writes, and scan bounds. Local
placeholder simulation cannot establish actual second-Mac iCloud propagation or
download completion; that requires a real second device and remains a separate
deployment check.
