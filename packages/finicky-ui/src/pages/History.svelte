<script lang="ts">
  import { onMount } from "svelte";
  import { historyOpenURL } from "../lib/historyModel";
  import Icon from "../components/StudioIcon.svelte";
  import type { HistoryEntry } from "../types";
  export let entries: HistoryEntry[] = [];
  export let loaded = false;
  export let enabled = false;
  export let clearing = false;
  export let onRefresh: () => void;
  export let onClear: () => void;
  export let onOpen: (url: string) => void;
  let search = "";
  let confirming = false;
  $: filtered = entries.filter((entry) =>
    [
      entry.url,
      entry.originalUrl,
      entry.browser,
      entry.profile,
      entry.profileDirectory,
      entry.sourceApp,
    ].some((value) => value?.toLowerCase().includes(search.toLowerCase())),
  );
  function time(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.valueOf())
      ? value
      : date.toLocaleString(undefined, {
          dateStyle: "medium",
          timeStyle: "short",
        });
  }
  function title(value: string) {
    try {
      return new URL(value).hostname;
    } catch {
      return value;
    }
  }
  onMount(onRefresh);
</script>

<div class="section-heading">
  <div>
    <div class="history-heading-title"><h2>Recent links</h2><span class="status-badge">{enabled ? "On this Mac" : "History off"}</span></div>
    <p>
      {enabled
        ? "Up to 200 recent links, stored on this Mac. Private browsing links are excluded."
        : "History is off. Enable it in Preferences to record future links."}
    </p>
  </div>
  <div class="inline-actions">
    <button class="button small secondary" onclick={onRefresh}>Refresh</button
    ><button
      class="button small secondary"
      disabled={!entries.length || clearing}
      onclick={() => (confirming = true)}
      ><Icon name="trash" size={13} />Clear history</button
    >
  </div>
</div>
{#if confirming}<div class="notice-banner history-confirm" role="alert">
    <span>Clear all saved link history? This cannot be undone.</span><button
      class="button small secondary"
      disabled={clearing}
      onclick={() => (confirming = false)}>Cancel</button
    ><button
      class="button small danger"
      disabled={clearing}
      onclick={() => {
        onClear();
        confirming = false;
      }}>Clear all</button
    >
  </div>{/if}
<div class="field history-search">
  <label class="sr-only" for="history-filter">Search recent links</label><input
    id="history-filter"
    type="search"
    placeholder="Search links, profiles, or source apps…"
    bind:value={search}
  />
  {#if loaded && entries.length}<p class="field-hint history-result-count" role="status">{search ? `${filtered.length} of ${entries.length} links` : `${entries.length} saved ${entries.length === 1 ? "link" : "links"}`}</p>{/if}
</div>
{#if !loaded}<div class="empty-state" role="status">
    <Icon name="history" size={26} />
    <h3>Loading recent links…</h3>
  </div>
{:else if !filtered.length}<div class="empty-state">
    <Icon name="history" size={26} />
    <h3>{search ? "No matching links" : enabled ? "No links recorded yet" : "History is off"}</h3>
    <p>
      {search
        ? "Try another website, browser, or profile name."
        : "Routed links will appear here when history is enabled."}
    </p>
  </div>
{:else}<ol class="history-list">
    {#each filtered as entry (entry.id)}<li class="history-card">
        <div class="history-card-top">
          <h3>{title(entry.url)}</h3>
          <time datetime={entry.timestamp}>{time(entry.timestamp)}</time>
        </div>
        <p class="history-url">{entry.url}</p>
        {#if entry.originalUrl && entry.originalUrl !== entry.url}<details
            class="history-original"
          >
            <summary>Original link</summary>
            <p>{entry.originalUrl}</p>
          </details>{/if}
        <div class="history-card-bottom">
          <div>
            <span class="history-destination"
              >{entry.browser || "No destination"}{entry.profile ||
              entry.profileDirectory
                ? ` · ${entry.profile || entry.profileDirectory}`
                : ""}</span
            >{#if entry.sourceApp}<span class="history-source"
                >From {entry.sourceApp}</span
              >{/if}{#if typeof entry.matchedRule === "number" && entry.matchedRule >= 0}<span
                class="history-source"
                >Routing rule {entry.matchedRule + 1}</span
              >{/if}{#if entry.error}<p class="error-text">
                {entry.error}
              </p>{/if}
          </div>
          <button
            class="button small secondary"
            disabled={!historyOpenURL(entry)}
            onclick={() => {
              const url = historyOpenURL(entry);
              if (url) onOpen(url);
            }}
            aria-label={`Choose where to open ${historyOpenURL(entry) || entry.url}`}
            >Open with…<Icon name="external" size={12} /></button
          >
        </div>
      </li>{/each}
  </ol>{/if}

<style>
  .history-heading-title { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
  .history-result-count { margin-top: 8px; }
</style>
