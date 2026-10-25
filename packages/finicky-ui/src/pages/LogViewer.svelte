<script lang="ts">
  import Icon from "../components/StudioIcon.svelte";
  import type { LogEntry } from "../types";
  import LogContent from "../components/LogContent.svelte";
  import { toast } from "../lib/toast";

  let {
    messageBuffer = [],
    onClearLogs,
  }: {
    messageBuffer: LogEntry[];
    onClearLogs: () => void;
  } = $props();

  let showDebug = $state(localStorage.getItem("showDebugLogs") === "true");
  const visibleCount = $derived(messageBuffer.filter((entry) => showDebug || entry.level.toLowerCase() !== "debug").length);

  async function copyLogs() {
    const logEntries = messageBuffer
      .filter((entry) => showDebug || entry.level.toLowerCase() !== "debug")
      .map((entry) => {
        const time = new Date(entry.time).toISOString();
        const baseMessage = `[${time}] [${entry.level.padEnd(5)}] ${entry.msg}`;
        const extraFields = Object.entries(entry)
          .filter(([key]: [string, any]) => !["level", "msg", "time", "error"].includes(key))
          .map(([key, value]: [string, any]) => `${key}: ${value}`)
          .join(" | ");
        const parts = [baseMessage];
        if (extraFields) parts.push(extraFields);
        if (entry.error) parts.push(`Error: ${entry.error}`);
        return parts.join(" | ");
      })
      .join("\n");

    try {
      await navigator.clipboard.writeText(logEntries);
      toast.success("Logs copied to clipboard");
    } catch (err) {
      console.error("Failed to copy logs:", err);
      toast.error("Failed to copy logs", err instanceof Error ? err.message : (err?.toString() ?? "unknown error"));
    }
  }
</script>

<section aria-labelledby="logs-heading">
  <div class="section-heading">
    <div><h2 id="logs-heading">Activity log</h2><p>Recent app activity for diagnosing routing and integration issues.</p></div>
    <div class="inline-actions">
      <button class="button small secondary" disabled={!messageBuffer.length} onclick={onClearLogs}><Icon name="trash" size={14} />Clear logs</button>
      <button class="button small secondary" disabled={!visibleCount} onclick={copyLogs}><Icon name="code" size={14} />Copy logs</button>
    </div>
  </div>
  <div class="log-toolbar">
    <span>{visibleCount} {visibleCount === 1 ? "entry" : "entries"}</span>
    <label class="check-label debug-toggle"><input type="checkbox" bind:checked={showDebug} onchange={() => localStorage.setItem("showDebugLogs", String(showDebug))} />Include debug messages</label>
  </div>
  {#if visibleCount}
    <div class="panel log-panel"><LogContent {messageBuffer} {showDebug} /></div>
  {:else}
    <div class="empty-state"><Icon name="logs" size={28} /><h3>No activity to show</h3><p>App events will appear here as you open links and change settings.</p></div>
  {/if}
</section>

<style>
  .log-toolbar { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; margin: 16px 0 12px; font-size: 12px; color: var(--text-secondary); }
  .debug-toggle { display: flex; align-items: center; gap: 8px; min-height: 32px; cursor: pointer; user-select: none; }
  .log-panel { padding: 20px; min-height: 240px; }
</style>
