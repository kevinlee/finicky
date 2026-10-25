<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import Icon from "../components/StudioIcon.svelte";
  import { bindingForFavorite, equalBindings, isChromiumBrowser, reconcileSyncMappings, syncBindingForSave, syncMappingErrors, syncResolution, syncStatusLabels, updateSyncBindingDraft } from "../lib/syncModel";
  import type { SyncBinding, SyncSnapshot } from "../types";

  export let snapshot: SyncSnapshot | null = null;
  export let busy = false;
  export let blocked = false;
  export let error = "";
  export let jsConfig = false;
  export let onRefresh: () => void;
  export let onEnabled: (enabled: boolean) => void;
  export let onSync: () => void;
  export let onSaveBindings: (bindings: Record<string, SyncBinding>) => void;
  export let onResolve: (choice: "local" | "remote", revision?: string) => void;
  export let onDraftChange: (dirty: boolean) => void = () => {};
  let mappingDraft: Record<string, SyncBinding> = {};
  let mappingBaseline: Record<string, SyncBinding> = {};
  let selections: Record<string, string> = {};
  let previousSnapshot: SyncSnapshot | null = null;
  let mappingConflict = false;
  let showMappingErrors = false;
  let pendingResolution: "local" | "remote" | "" = "";
  let selectedRevision = "";
  $: mappingDirty = !equalBindings(mappingDraft, mappingBaseline);
  $: onDraftChange(mappingDirty);
  $: selectionValues = resolveSelections(snapshot, mappingDraft, selections);
  $: bindingsToSave = Object.fromEntries(Object.entries(mappingDraft).map(([id, binding]) => [id, syncBindingForSave(binding, selectionValues[id] === "manual")]));
  $: mappingErrors = snapshot ? syncMappingErrors(snapshot, bindingsToSave) : {};
  $: visibleErrors = Object.fromEntries(Object.entries(mappingErrors).filter(([id, message]) => Boolean(message) && (showMappingErrors || snapshot?.missingProfileIds.includes(id))));
  $: revisionNames = Object.fromEntries((snapshot?.heads || []).map((id) => [id, revisionName(id, snapshot)]));
  $: unavailableReason = snapshot?.blockedReason || (jsConfig ? "This Mac uses a JavaScript configuration. Move those routing rules into visual settings before enabling sync." : "");
  $: mutationBlocked = busy || blocked;
  $: remoteChoice = snapshot ? syncResolution(snapshot, "remote", selectedRevision) : null;
  $: if (snapshot && snapshot !== previousSnapshot) {
    previousSnapshot = snapshot;
    const reconciled = reconcileSyncMappings(mappingDraft, mappingBaseline, snapshot.bindings);
    if (reconciled.draft !== mappingDraft) {
      resetMappings(snapshot);
    } else mappingConflict = reconciled.conflict;
    if (snapshot.state !== "conflict") { pendingResolution = ""; selectedRevision = ""; }
    else if (selectedRevision && !snapshot.heads.includes(selectedRevision)) { selectedRevision = ""; pendingResolution = ""; }
  }
  function resetMappings(current: SyncSnapshot) {
    mappingDraft = structuredClone(current.bindings);
    mappingBaseline = structuredClone(current.bindings);
    selections = {};
    mappingConflict = false;
    showMappingErrors = false;
  }
  function resolveSelections(current: SyncSnapshot | null, draft: Record<string, SyncBinding>, explicit: Record<string, string>): Record<string, string> {
    return Object.fromEntries((current?.profiles || []).map(({ id }) => {
      if (explicit[id] !== undefined) return [id, explicit[id]];
      const binding = draft[id];
      if (!binding) return [id, ""];
      const favorite = current?.localProfiles.find((entry) => equalBindings({ chosen: bindingForFavorite(entry) }, { chosen: binding }));
      return [id, favorite ? `favorite:${favorite.id}` : "manual"];
    }));
  }
  function choose(id: string, value: string) {
    selections = { ...selections, [id]: value };
    const next = { ...mappingDraft };
    if (!value) delete next[id];
    else if (value === "manual") next[id] = next[id] || { browser: "" };
    else {
      const favorite = snapshot?.localProfiles.find((profile) => `favorite:${profile.id}` === value);
      if (favorite) next[id] = bindingForFavorite(favorite);
    }
    mappingDraft = next;
  }
  function updateBinding(id: string, patch: Partial<SyncBinding>) {
    mappingDraft = { ...mappingDraft, [id]: updateSyncBindingDraft(mappingDraft[id], patch) };
  }
  function saveMappings(event: SubmitEvent) {
    event.preventDefault();
    showMappingErrors = true;
    if (!snapshot || mutationBlocked || unavailableReason || mappingConflict) return;
    const firstInvalid = Object.keys(mappingErrors)[0];
    if (firstInvalid) {
      const field = selectionValues[firstInvalid] === "manual"
        ? (mappingDraft[firstInvalid]?.browser.trim() ? "directory" : "browser")
        : "local";
      document.getElementById(`sync-${field}-${firstInvalid}`)?.focus();
      return;
    }
    const bindings = Object.fromEntries(snapshot.profiles.map((profile) => [profile.id, bindingsToSave[profile.id]]));
    // Explicit Save commits the selected browser's fields to the draft too.
    // Keep the old baseline until the native app acknowledges persistence.
    mappingDraft = structuredClone(bindings);
    onSaveBindings(bindings);
  }
  function displayTime(value: string) {
    const date = new Date(value);
    return Number.isFinite(date.valueOf()) ? date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) : "Not available";
  }
  function revisionName(id: string, current: SyncSnapshot | null): string {
    const detail = current?.headDetails?.find((head) => head.id === id);
    const device = detail?.deviceName || (detail?.deviceId ? `Mac ${detail.deviceId.slice(0, 8)}` : "Shared version");
    return `${device}${detail?.createdAt ? ` · ${displayTime(detail.createdAt)}` : ""} · ${id.slice(0, 8)}`;
  }
  onMount(onRefresh);
  onDestroy(() => onDraftChange(false));
</script>

<section aria-labelledby="sync-heading">
  <div class="section-heading">
    <div><h2 id="sync-heading">Sync status</h2></div>
    <button class="button small secondary" disabled={busy} onclick={onRefresh}>Refresh</button>
  </div>
  {#if error}<div class="notice-banner warning" role="alert"><Icon name="info" size={17} /><span>{error}</span></div>{/if}
  {#if blocked}<div class="notice-banner" role="status"><Icon name="edit" size={17} /><span>Apply or discard your settings changes before changing sync settings. Wait for any save or import to finish.</span></div>{/if}
  {#if unavailableReason}<div class="notice-banner warning"><Icon name="info" size={17} /><span>{unavailableReason}</span></div>{/if}
  {#if !snapshot}
    <div class="empty-state" role="status"><Icon name="sync" size={26} /><h3>{error ? "Sync information is unavailable" : "Loading sync settings…"}</h3></div>
  {:else}
    <div class="panel preferences-panel sync-settings">
      <div class="preference-row">
        <div><h3>Sync with iCloud Drive</h3><p>{snapshot.enabled ? "On · this Mac shares routing settings." : "Off · settings stay on this Mac."}</p></div>
        <button class="switch" class:on={snapshot.enabled} role="switch" aria-checked={snapshot.enabled} aria-label="Sync settings with iCloud" disabled={mutationBlocked || Boolean(unavailableReason && !snapshot.enabled) || mappingDirty} onclick={() => onEnabled(!snapshot!.enabled)}><span></span></button>
      </div>
      <div class="sync-status">
        <div class="sync-status-heading">
          <span class="sync-state-icon" class:success={snapshot.state === "synced"} class:needs-attention={["conflict", "awaitingMapping", "error"].includes(snapshot.state)}><Icon name={snapshot.state === "synced" ? "check" : "sync"} size={20} /></span>
          <div><div role="status" aria-atomic="true"><h3>{syncStatusLabels[snapshot.state]}</h3>{#if snapshot.message}<p>{snapshot.message}</p>{/if}</div>{#if snapshot.lastSyncedAt}<p class="sync-muted">Last synced {displayTime(snapshot.lastSyncedAt)}</p>{/if}</div>
        </div>
        <button class="button small secondary" disabled={mutationBlocked || !snapshot.enabled || Boolean(unavailableReason) || mappingDirty || snapshot.state === "conflict" || snapshot.state === "awaitingMapping"} onclick={onSync}>{busy ? "Working…" : "Sync now"}</button>
      </div>
    </div>
    <p class="field-hint sync-save-hint">{mappingDirty ? "Save or discard profile mappings before changing sync or sharing settings." : "Sync changes save immediately. Turning sync off keeps this Mac’s settings."}</p>

    {#if snapshot.state === "conflict"}
      <section class="sync-conflict panel" aria-labelledby="sync-conflict-heading">
        <h3 id="sync-conflict-heading">Settings changed on more than one Mac</h3>
        <p>Choose which version should be shared. A backup is kept before settings are replaced.</p>
        <div class="sync-conflict-choices">
          <div><strong>Keep this Mac’s settings</strong><p>Replace the shared version with this Mac’s saved routing settings. Other Macs will receive this version.</p><button class="button small secondary" disabled={mutationBlocked || Boolean(unavailableReason) || mappingDirty} onclick={() => (pendingResolution = "local")}>Keep this Mac’s settings</button></div>
          <div><strong>Use shared settings</strong><p>Replace this Mac’s routing settings with the shared version you choose. Local browser mappings stay specific to this Mac.</p>{#if snapshot.heads.length > 1}<div class="field sync-revision-field"><label for="sync-conflict-revision">Shared version to use</label><select id="sync-conflict-revision" bind:value={selectedRevision} disabled={mutationBlocked || Boolean(unavailableReason) || mappingDirty}><option value="" disabled>Choose a shared version…</option>{#each snapshot.heads as revision}<option value={revision}>{revisionNames[revision]}</option>{/each}</select></div>{:else if snapshot.heads.length === 1}<p class="sync-muted">{revisionNames[snapshot.heads[0]]}</p>{:else}<p class="sync-muted">No shared version is available. Refresh to check again.</p>{/if}<button class="button small secondary" disabled={mutationBlocked || Boolean(unavailableReason) || mappingDirty || !remoteChoice} onclick={() => (pendingResolution = "remote")}>Use shared settings</button></div>
        </div>
        {#if pendingResolution}<div class="notice-banner sync-resolution" role="alert"><span>{pendingResolution === "local" ? "Replace the shared settings with this Mac’s saved settings?" : `Replace this Mac’s saved settings with ${remoteChoice?.revision ? revisionNames[remoteChoice.revision] : "the selected shared version"}?`}</span><button class="button small secondary" disabled={mutationBlocked} onclick={() => (pendingResolution = "")}>Cancel</button><button class="button small primary" disabled={mutationBlocked || Boolean(unavailableReason) || mappingDirty || (pendingResolution === "remote" && !remoteChoice)} onclick={() => { const command = syncResolution(snapshot!, pendingResolution, selectedRevision); if (command) onResolve(command.choice, command.revision); }}>Confirm choice</button></div>{/if}
      </section>
    {/if}

    <section class="feature-section" aria-labelledby="sync-mapping-heading">
      <div class="section-heading"><div><h2 id="sync-mapping-heading">Profiles on this Mac</h2><p>Choose where each shared profile opens here. Set this up once on each Mac.</p></div></div>
      {#if mappingConflict}<div class="notice-banner warning" role="alert"><span>Saved profile mappings changed while you were editing. Reload them before saving.</span><button class="button small secondary" disabled={busy} onclick={() => resetMappings(snapshot!)}>Reload saved mappings</button></div>{/if}
      {#if !snapshot.profiles.length}<div class="empty-state"><Icon name="profiles" size={26} /><h3>No shared profiles yet</h3><p>Turn on sync to share this Mac’s saved profile setup or load the setup already in iCloud.</p></div>
      {:else}
        <form class="sync-mapping-form" onsubmit={saveMappings}>
          <div class="panel sync-mapping-list">
          {#each snapshot.profiles as profile (profile.id)}
            <div class="sync-mapping-row">
              <div class="sync-profile-name"><span class="sync-profile-mark"><Icon name={profile.private ? "lock" : "profiles"} size={18} /></span><div><h3>{profile.name}</h3><p>{profile.private ? "Private destination" : "Shared profile"}{profile.hidden ? " · hidden in picker" : ""}</p></div></div>
              <div class="sync-profile-controls">
                <div class="field"><label class="sr-only" for={`sync-local-${profile.id}`}>Local destination for {profile.name}</label><select id={`sync-local-${profile.id}`} aria-invalid={Boolean(visibleErrors[profile.id])} aria-describedby={visibleErrors[profile.id] ? `sync-error-${profile.id}` : undefined} value={selectionValues[profile.id]} disabled={mutationBlocked || Boolean(unavailableReason)} onchange={(event) => choose(profile.id, event.currentTarget.value)}><option value="">Choose a destination on this Mac…</option>{#each snapshot.localProfiles as local}<option value={`favorite:${local.id}`}>{local.name} · {local.browser}{local.profileDirectory ? ` · ${local.profileDirectory}` : local.profile ? ` · ${local.profile}` : ""}{local.private ? " · private" : ""}</option>{/each}<option value="manual">Enter a destination manually…</option></select></div>
                {#if selectionValues[profile.id] === "manual"}
                  <div class="field-row"><div class="field"><label for={`sync-browser-${profile.id}`}>Browser</label><input id={`sync-browser-${profile.id}`} aria-invalid={Boolean(visibleErrors[profile.id])} aria-describedby={visibleErrors[profile.id] ? `sync-error-${profile.id}` : undefined} list="sync-local-browsers" value={mappingDraft[profile.id]?.browser || ""} placeholder="Google Chrome" disabled={mutationBlocked || Boolean(unavailableReason)} oninput={(event) => updateBinding(profile.id, { browser: event.currentTarget.value })} /></div>
                    <div class="field"><label for={`sync-directory-${profile.id}`}>{isChromiumBrowser(mappingDraft[profile.id]?.browser || "") ? "Chrome profile directory" : "Profile name (optional)"}</label><input id={`sync-directory-${profile.id}`} aria-invalid={Boolean(visibleErrors[profile.id])} aria-describedby={visibleErrors[profile.id] ? `sync-error-${profile.id}` : undefined} value={isChromiumBrowser(mappingDraft[profile.id]?.browser || "") ? mappingDraft[profile.id]?.profileDirectory || "" : mappingDraft[profile.id]?.profile || ""} placeholder={isChromiumBrowser(mappingDraft[profile.id]?.browser || "") ? "Default or Profile 2" : "Profile name"} disabled={mutationBlocked || Boolean(unavailableReason)} oninput={(event) => updateBinding(profile.id, isChromiumBrowser(mappingDraft[profile.id]?.browser || "") ? { profileDirectory: event.currentTarget.value } : { profile: event.currentTarget.value })} /></div></div>
                {/if}
                {#if visibleErrors[profile.id]}<p id={`sync-error-${profile.id}`} class="error-text sync-field-error">{visibleErrors[profile.id]}</p>{/if}
              </div>
            </div>
          {/each}
          </div>
          <datalist id="sync-local-browsers">{#each [...new Set(["Google Chrome", "Safari", ...snapshot.localProfiles.map((profile) => profile.browser)])] as browser}<option value={browser}></option>{/each}</datalist>
          <div class="sync-mapping-footer"><p>{mappingDirty ? "You have unsaved profile mappings." : "Profile mappings stay on this Mac."}</p><div class="inline-actions"><button class="button small secondary" type="button" disabled={mutationBlocked || !mappingDirty} onclick={() => resetMappings(snapshot!)}>Discard mapping changes</button><button class="button small primary" type="submit" disabled={mutationBlocked || Boolean(unavailableReason) || !mappingDirty || mappingConflict}>Save profile mappings</button></div></div>
        </form>
      {/if}
    </section>
    <details class="sync-details panel">
      <summary><Icon name="lock" size={16} />What syncs & privacy</summary>
      <div class="sync-details-content">
        <p>Routing rules, saved profile labels, and supported link preferences sync through your iCloud Drive.</p>
        <p>Learning records, link history, browser accounts, and profile mappings stay on each Mac. Sync does not sign you into Chrome or transfer browsing data. Destinations always need an explicit mapping on each Mac.</p>
        {#if snapshot.cloudPath}<p>Shared settings folder: <code>{snapshot.cloudPath}</code></p>{/if}
        {#each snapshot.limitations || [] as limitation}<p>{limitation}</p>{/each}
      </div>
    </details>
  {/if}
</section>

<style>
  .sync-settings { margin-top: 20px; }
  .sync-status { padding: 20px; border-top: 1px solid var(--border-color); display: flex; justify-content: space-between; align-items: center; gap: 20px; }
  .sync-status-heading { display: flex; align-items: flex-start; gap: 12px; }
  .sync-state-icon, .sync-profile-mark { display: grid; place-items: center; flex-shrink: 0; width: 36px; height: 36px; border-radius: 8px; background: var(--inset-bg); color: var(--text-secondary); }
  .sync-state-icon.needs-attention { color: var(--warning, var(--log-warning)); background: var(--warning-bg, var(--inset-bg)); }
  .sync-state-icon.success { color: var(--success, var(--log-success)); background: var(--success-bg, var(--inset-bg)); }
  h3 { font-size: 14px; font-weight: 600; }
  .sync-status p, .sync-conflict p { font-size: 12px; color: var(--text-secondary); margin-top: 4px; }
  .sync-status .button { flex-shrink: 0; }
  .sync-muted { font-size: 12px; }
  .sync-save-hint { margin-top: 8px; }
  .sync-conflict { margin-top: 24px; padding: 20px; border-color: var(--log-warning); }
  .sync-conflict-choices { margin-top: 16px; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
  .sync-conflict-choices > div { border: 1px solid var(--card-border); border-radius: 8px; padding: 16px; }
  .sync-conflict-choices strong { font-size: 14px; }
  .sync-conflict-choices button { margin-top: 12px; }
  .sync-revision-field { margin: 12px 0 0; }
  .sync-resolution { flex-wrap: wrap; margin-bottom: 0; }
  .sync-resolution span { flex: 1; min-width: 180px; }
  .sync-mapping-form { display: grid; gap: 12px; margin-top: 16px; }
  .sync-mapping-list { overflow: hidden; padding: 0; }
  .sync-mapping-row { display: grid; grid-template-columns: minmax(160px, 1fr) minmax(0, 2fr); gap: 20px; padding: 16px 20px; align-items: center; }
  .sync-mapping-row + .sync-mapping-row { border-top: 1px solid var(--border-color); }
  .sync-profile-name { display: flex; align-items: center; gap: 12px; }
  .sync-profile-name p { font-size: 12px; color: var(--text-secondary); margin-top: 4px; }
  .sync-profile-controls .field { margin-bottom: 0; }
  .sync-profile-controls .field-row { margin-top: 12px; }
  .sync-profile-controls .field-row .field { flex: 1; min-width: 0; }
  .sync-field-error { margin-top: 8px; font-size: 12px; }
  .sync-mapping-footer { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 12px; }
  .sync-mapping-footer > p { flex: 1; min-width: 180px; color: var(--text-secondary); font-size: 12px; }
  .sync-details { margin-top: 24px; font-size: 12px; color: var(--text-secondary); }
  .sync-details summary { cursor: pointer; display: flex; gap: 8px; align-items: center; padding: 16px 20px; min-height: 40px; font-weight: 600; color: var(--text-primary); }
  .sync-details summary::after { content: "+"; margin-left: auto; font-size: 18px; font-weight: 400; }
  .sync-details[open] summary::after { content: "−"; }
  .sync-details-content { padding: 0 20px 20px; }
  .sync-details p { margin-top: 8px; overflow-wrap: anywhere; }
  @media (max-width: 760px) { .sync-mapping-row { grid-template-columns: 1fr; gap: 12px; } .sync-conflict-choices { grid-template-columns: 1fr; } .sync-status { align-items: flex-start; } }
</style>
