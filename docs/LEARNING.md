# Local shadow learning

Waystile can collect a small local dataset of explicit browser-profile choices
when the user turns learning on. Learning starts disabled. This implementation
does not change routing, reorder the chooser, or preselect its suggested answer.
The shadow suggestion is recorded for inspection and evaluation after the
decision. Rules and the user's chooser selection remain authoritative.

## Collection and privacy contract

The application supplies a captured arrival time, a later observation time, the
source application's bundle identifier, a positively known source privacy state,
and the stable identities of the available saved destinations. Each accepted
request gets an immutable random ID. The caller records an observation only after
the final launch is known to be nonprivate. A private launch must not create a
request, confirmation, or correction.

The learning package also enforces explicit opt-in for each observation. Private
and unknown source privacy are excluded. In particular, an external URL from a
browser is not presumed public merely because its private-mode state is missing.
Extension privacy state is browser-reported metadata. A public field in a custom
URL scheme is not authenticated attestation of the caller or browser mode; the
integration must conservatively exclude sources it cannot establish as public.
Private destinations are excluded from the candidate set and cannot receive a
label. Destination identities should use browser bundle plus stable profile
directory basename, or a stable opaque encoding, not a display name or full path.
Profile renames should not create new identities.

Only the lowercased DNS hostname is retained from HTTP(S) URLs. URL paths, titles,
queries, fragments, credentials, and the full URL are not persisted. URLs with
credentials are rejected altogether. IP addresses, single-label hosts, localhost,
and `.local`, `.internal`, `.home`, `.lan`, and `.onion` destinations are excluded.
This conservative filter is not a universal classifier for sensitive websites;
hostnames can themselves be sensitive. No browsing history or page contents are
collected by this package and no learning data is sent to a server.

## Labels and time

An observation alone is unlabeled. Automatic rules, fallback launches, lack of a
correction, and the model's own prediction never count as confirmation.

`Confirm` records an explicit, neutral chooser selection after a successful launch.
`Correct` records an explicit correction linked to that same request. It requires
an existing confirmation, retains earlier versions, and has a strictly later
availability timestamp. Each label stores its provenance and the time it became
available. Calling code must maintain these semantics; the package cannot infer
whether a UI selection was influenced by a suggestion.

For a request received at time T, training uses only other requests received and
observed strictly before T, with feedback already available strictly before T.
It selects the latest feedback version available then. Same-time events are
excluded, as are the current request's label and all future labels/corrections.
Delayed observations are therefore unavailable to earlier requests even when
their original arrival time was earlier.

## Baseline and abstention

The deterministic baseline first uses the majority of eligible confirmed labels
for the same hostname and source bundle. With no such examples it falls back to
the hostname alone. Ties use the most recently available feedback and then stable
destination identity. Each output reports the method, winning support count,
matching-label count, and up to five supporting feedback references. These counts
are evidence summaries, not calibrated probabilities or confidence thresholds.

The baseline abstains for absent history, unseen hostnames, and candidate profiles
that have never received an eligible explicit confirmation. New profiles thus
start without assumptions. It ranks historical destinations before checking
availability: if the historical winner is absent from the current request's
candidate set it abstains rather than silently promoting a different profile.
The caller supplies only relevant saved destinations, not every installed browser.

Each observation stores the actual shadow prediction captured at arrival time.
`Predict` is a separate replay operation using the immutable candidate snapshot;
it cannot use a cutoff after that request's arrival. A new live decision needs a
new request with the current candidates. Replay must not be presented as the
original captured result.

## Evaluation and limits of the evidence

Evaluation replays requests in historical time using the strict availability
rules above. Main outcomes use the first neutral chooser label available by the
evaluation cutoff. Later corrections are counted separately and do not silently
rewrite evaluation truth. Earlier corrections can affect training for later
requests only after they became available.

The report gives the number of observed requests, explicitly labeled requests,
predictions, correct predictions, errors, abstentions, and supporting examples.
Coverage uses labeled requests as its denominator; error rate uses predictions
and is absent when there were none. No labels yields `data_not_ready`. Other
reports remain descriptive and labeled-only. They are selected, temporally
correlated samples; they do not estimate accuracy across automatically opened or
excluded links. Corrected choices may also reveal that the initial outcome was
mistaken, which is why correction counts are shown rather than hidden.

There is no automatic-routing threshold or GNN in this implementation. A future
experiment would need a frozen, time-correct comparison with this baseline,
explicit coverage and error costs, sufficient held-out support, and uncertainty
estimates that account for correlated requests. Agreement between reviewers is
not evidence that a GNN improves routing.

## Storage and deletion

The default store is `Finicky Plus/learning/observations.json` under the user's
application-support/config directory. Its dedicated directory is mode 0700 and
its JSON file is mode 0600. Writes use a same-directory private temporary file,
sync it, and atomically replace the previous file. A process-local path lock
serializes store instances; the desktop application is the single writer.
Corrupt, oversized, unknown-schema, and symlink store files fail closed rather
than being silently overwritten. Explicit Clear can replace corrupt regular data.

Storage is capped at 2,000 requests, 8,000 feedback versions, and 8 MiB. When
request or feedback counts exceed their bounds, the oldest request and its whole
feedback history are removed until within bounds. These are operational storage
limits, not learned quality thresholds. An overlarge encoded write fails without
replacing the existing file. Retention eviction clears existing captured
predictions because their counts or examples may derive from evicted labels. A
new observation captures its shadow output after trimming, against only the
remaining evidence. Older captures are not reconstructed or presented as originals.

Delete-request removes its full feedback history. Delete-destination removes any
request ever labeled with that destination and removes the destination from
remaining candidate snapshots; this prevents an earlier label from reviving
after a correction is deleted. Either deletion also clears captured predictions,
which may contain derived references to removed evidence. Clear removes all
records. There is no retained model, training cache, or hidden derived state:
subsequent predictions rebuild from the remaining records. The application should
use deletion on removed profiles and offer a clear-data control when disabling
learning. If a user wants a fresh start, Clear resets all learned behavior.

`RetainDestinations(ids []string) (changed bool, err error)` atomically reconciles
storage with currently eligible saved destination identities at startup and after
configuration changes. Removed, hidden, private, or retargeted destinations are
excluded from that set. Their labeled requests and all feedback versions are
deleted, their identities are removed from other request candidate snapshots,
and derived captures are cleared. An empty candidate set removes all records.
Display-name changes preserve the stable browser/profile identity and therefore
preserve learning. Reconciliation returns false and does not rewrite the file
when nothing changed; true means the changed state was successfully saved. The
caller can use this result to invalidate pending observations through its lifecycle
epoch. The store itself does not inspect application configuration or profile names.

Package validation: `cd apps/finicky/src && go test -race ./learning`.
