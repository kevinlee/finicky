<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "../components/StudioIcon.svelte";
  import { learningCorrection, learningEvaluation, learningPrediction, learningSourceName } from "../lib/learningModel";
  import type { LearningSnapshot } from "../types";

  export let snapshot: LearningSnapshot | null = null;
  export let busy = false;
  export let error = "";
  export let onRefresh: () => void;
  export let onEnabled: (enabled: boolean) => void;
  export let onClear: () => void;
  export let onCorrect: (requestId: string, destinationId: string) => void;
  export let onDelete: (requestId: string) => void;
  let confirmingClear = false;
  let editing = "";
  let selectedDestination = "";
  $: correction = snapshot && learningCorrection(snapshot, editing, selectedDestination);

  function time(value: string) {
    return new Date(value).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
  }
  function edit(id: string) {
    editing = id;
    selectedDestination = "";
  }
  onMount(onRefresh);
</script>

<section aria-labelledby="learning-heading">
  <div class="section-heading">
    <div>
      <h2 id="learning-heading">Learn from your choices</h2>
      <p>A local experiment that checks predictions without changing where links open.</p>
    </div>
    <button class="button small secondary" disabled={busy} onclick={onRefresh}>Refresh</button>
  </div>
  {#if error}<div class="notice-banner warning" role="alert"><Icon name="info" size={17} /><span>{error}</span></div>{/if}
  {#if confirmingClear && !snapshot}
    <div class="notice-banner history-confirm" role="alert">
      <span>Delete all local learning examples and learned counts? This cannot be undone. Your settings, routing rules, and history stay as they are.</span>
      <button class="button small secondary" disabled={busy} onclick={() => (confirmingClear = false)}>Cancel</button>
      <button class="button small primary" disabled={busy} onclick={() => { onClear(); confirmingClear = false; }}>Reset learning data</button>
    </div>
  {/if}
  {#if !snapshot}
    <div class="empty-state" role="status">
      <Icon name="learning" size={26} /><h3>{error ? "Learning is unavailable" : "Loading learning settings…"}</h3>
      {#if error}<p>If saved learning records cannot be read, you can reset them. This removes only local learning data.</p><button class="button small secondary learning-recovery" disabled={busy} onclick={() => (confirmingClear = true)}>{busy ? "Checking learning data…" : "Reset learning data"}</button>{/if}
    </div>
  {:else}
    <div class="panel preferences-panel learning-settings">
      <div class="preference-row">
        <div class="preference-description">
          <h3>Learn on this Mac</h3>
          <p>{snapshot.enabled ? "On · predictions stay in the background." : "Off · no new learning examples are collected."}</p>
        </div>
        <button
          class="switch"
          class:on={snapshot.enabled}
          type="button"
          role="switch"
          aria-checked={snapshot.enabled}
          aria-label="Learn on this Mac"
          disabled={busy}
          onclick={() => onEnabled(!snapshot!.enabled)}><span></span></button>
      </div>
      <div class="learning-explanation">
        <Icon name="lock" size={17} />
        <div>
          <p>Learning uses explicit chooser selections from eligible links that a browser reports as regular browsing. Private or unknown sources and private destinations are excluded.</p>
          <p>Only domains, source apps, and choices stay on this Mac. History is not training data. Predictions never choose or preselect a profile.</p>
          <p>Changes save immediately. Turning learning off keeps existing examples until you clear them.</p>
        </div>
      </div>
    </div>

    <section class="feature-section" aria-labelledby="learning-evaluation-heading">
      <div class="section-heading">
        <div><h2 id="learning-evaluation-heading">Checked against initial choices</h2><p>Each prediction uses only choices known before that request.</p></div>
      </div>
      <dl class="learning-stats">
        <div><dt>Confirmed choices</dt><dd>{snapshot.feedbackCount}</dd></div>
        <div><dt>Predictions made</dt><dd>{snapshot.evaluation.predicted}</dd></div>
        <div><dt>Matched initial choices</dt><dd>{snapshot.evaluation.correct}</dd></div>
        <div><dt>Insufficient data</dt><dd>{snapshot.evaluation.abstained}</dd></div>
      </dl>
      <p class="learning-evaluation">{learningEvaluation(snapshot)}</p>
      <p class="learning-quiet">Unconfirmed requests do not count as successes or failures. These results describe confirmed choices, not all links.</p>
      <p class="learning-quiet">Corrections update future learning. Historical results keep the original chooser selection.</p>
      {#if snapshot.limitations?.length}<details class="learning-limitations"><summary>About this experiment</summary><ul>{#each snapshot.limitations as limitation}<li>{limitation}</li>{/each}</ul></details>{/if}
    </section>

    <section class="feature-section" aria-labelledby="learning-examples-heading">
      <div class="section-heading">
        <div><h2 id="learning-examples-heading">Recent examples</h2><p>Review what was learned, correct a choice, or remove an example.</p></div>
        <button class="button small secondary" disabled={busy || (!snapshot.observations.length && !snapshot.feedbackCount)} onclick={() => (confirmingClear = true)}><Icon name="trash" size={13} />Clear learning data</button>
      </div>
      {#if confirmingClear}
        <div class="notice-banner history-confirm" role="alert">
          <span>Delete all local learning examples and learned counts? This cannot be undone. Your settings, routing rules, and history stay as they are.</span>
          <button class="button small secondary" disabled={busy} onclick={() => (confirmingClear = false)}>Cancel</button>
          <button class="button small primary" disabled={busy} onclick={() => { onClear(); confirmingClear = false; }}>Delete learning data</button>
        </div>
      {/if}
      {#if !snapshot.observations.length}
        <div class="empty-state"><Icon name="learning" size={26} /><h3>No learning examples yet</h3><p>{snapshot.enabled ? "Use the browser integration to open an eligible link, then choose its destination in the picker." : "Turn on learning to collect future eligible choices."}</p></div>
      {:else}
        <ol class="history-list learning-list">
          {#each snapshot.observations as observation (observation.id)}
            <li class="history-card">
              <div class="history-card-top"><h3>{observation.domain}</h3><time datetime={observation.observedAt}>{time(observation.observedAt)}</time></div>
              {#if observation.sourceApp}<p class="history-source">From {learningSourceName(observation.sourceApp)}</p>{/if}
              <dl class="learning-choice-pair">
                <div><dt>Confirmed choice</dt><dd>{observation.confirmedDestinationId ? observation.confirmedDestinationName || "Unavailable destination" : "No confirmed choice"}{#if ["correction", "explicit_correction"].includes(observation.feedbackKind || "")}<span class="learning-corrected">Corrected</span>{/if}</dd></div>
                <div><dt>Background prediction</dt><dd>{learningPrediction(observation)}</dd></div>
              </dl>
              {#if editing === observation.id}
                <form class="learning-correction" onsubmit={(event) => {
                  event.preventDefault();
                  if (correction) { onCorrect(correction.requestId, correction.destinationId); editing = ""; }
                }}>
                  <div class="field"><label for={`learning-destination-${observation.id}`}>Which destination did you intend?</label>
                    <select id={`learning-destination-${observation.id}`} bind:value={selectedDestination} disabled={busy}>
                      <option value="" disabled>Choose a destination…</option>
                      {#each snapshot.destinations as destination}<option value={destination.id}>{destination.name}</option>{/each}
                    </select>
                    <p class="learning-quiet">This updates learning only. It does not reopen the link.</p>
                  </div>
                  <div class="inline-actions"><button class="button small primary" type="submit" disabled={busy || !correction}>Save correction</button><button class="button small secondary" type="button" disabled={busy} onclick={() => (editing = "")}>Cancel</button></div>
                </form>
              {:else}
                <div class="inline-actions learning-example-actions">
                  {#if observation.confirmedDestinationId}<button class="button small secondary" disabled={busy || !snapshot.destinations.length} onclick={() => edit(observation.id)}><Icon name="edit" size={12} />Correct choice</button>{/if}
                  <button class="button small secondary" disabled={busy} onclick={() => onDelete(observation.id)} aria-label={`Delete learning example for ${observation.domain}`}><Icon name="trash" size={12} />Delete example</button>
                </div>
              {/if}
            </li>
          {/each}
        </ol>
      {/if}
    </section>
  {/if}
</section>

<style>
  .learning-settings { margin-top: 20px; }
  .learning-recovery { margin-top: 16px; }
  .learning-explanation { display: flex; gap: 12px; padding: 18px 22px; border-top: 1px solid var(--border-color); color: var(--text-secondary); font-size: 12px; }
  .learning-explanation p + p { margin-top: 8px; }
  .learning-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin: 18px 0 12px; }
  .learning-stats > div { padding: 18px; border: 1px solid var(--card-border); border-radius: 12px; background: var(--card-bg); }
  dt { font-size: 11px; color: var(--text-secondary); }
  dd { margin: 5px 0 0; }
  .learning-stats dd { font-size: 26px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .learning-evaluation { font-size: 12px; }
  .learning-quiet { margin-top: 6px; font-size: 11px; color: var(--text-secondary); }
  .learning-limitations { margin-top: 14px; font-size: 11px; color: var(--text-secondary); }
  .learning-limitations summary { cursor: pointer; }
  .learning-limitations li + li { margin-top: 6px; }
  .learning-list { margin-top: 20px; }
  .learning-choice-pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; margin: 18px 0; }
  .learning-choice-pair dd { font-size: 13px; overflow-wrap: anywhere; }
  .learning-corrected { display: inline-block; margin-left: 7px; font-size: 10px; color: var(--text-secondary); }
  .learning-example-actions { justify-content: flex-end; }
  .learning-correction { padding-top: 16px; border-top: 1px solid var(--border-color); }
  .learning-correction .field { margin-bottom: 14px; }
  @media (max-width: 850px) { .learning-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
  @media (max-width: 560px) { .learning-choice-pair { grid-template-columns: 1fr; } .learning-example-actions { justify-content: flex-start; } }
</style>
