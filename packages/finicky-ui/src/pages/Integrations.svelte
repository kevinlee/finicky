<script lang="ts">
  import { onMount } from "svelte";
  import Icon from "../components/StudioIcon.svelte";
  import { integrationURL } from "../lib/featuresModel";
  import { toast } from "../lib/toast";
  import type { IntegrationInfo } from "../types";
  export let info: IntegrationInfo | null = null;
  export let exportedJSON = "";
  export let importing = false;
  export let dirty = false;
  export let requestingBrowserAccess = false;
  export let onGrantBrowserAccess: () => void;
  export let onLoad: () => void;
  export let onExport: () => void;
  export let onImport: (json: string) => void;
  let importJSON = "";
  let importError = "";
  let importSummary = "";
  let showImport = false;
  let exportField: HTMLTextAreaElement;
  $: scheme = info?.scheme?.replace(/:\/\/$/, "") || "finicky-plus";
  $: exampleOpen = integrationURL(scheme, "open", {
    url: "https://example.com",
    prompt: "true",
  });
  $: exampleProfile = integrationURL(scheme, "open", {
    url: "https://example.com",
    browser: "com.google.Chrome",
    profileDirectory: "Profile 2",
  });
  $: exampleClipboard = integrationURL(scheme, "clipboard", { prompt: "true" });
  function inspectImport(value: string) {
    importJSON = value;
    importError = "";
    importSummary = "";
    if (!value.trim()) return;
    try {
      const parsed = JSON.parse(value);
      if (!parsed || typeof parsed !== "object" || !Array.isArray(parsed.rules))
        throw new Error(
          "Choose a Waystile settings JSON file with a rules array.",
        );
      importSummary = `${parsed.rules.length} rules and ${Array.isArray(parsed.profiles) ? parsed.profiles.length : 0} profiles ready to import.`;
    } catch (error) {
      importError =
        error instanceof Error ? error.message : "This JSON could not be read.";
    }
  }
  async function readFile(event: Event) {
    const file = (event.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    if (file.size > 5_000_000) {
      importError = "Choose a settings file smaller than 5 MB.";
      return;
    }
    try {
      inspectImport(await file.text());
    } catch {
      importError = "The selected file could not be read.";
    }
  }
  async function copyExport() {
    try {
      if (navigator.clipboard?.writeText)
        await navigator.clipboard.writeText(exportedJSON);
      else {
        exportField.focus();
        exportField.select();
        if (!document.execCommand("copy")) throw new Error();
      }
      toast.show("Settings JSON copied", "success");
    } catch {
      exportField?.focus();
      exportField?.select();
      toast.show("Press ⌘C to copy the selected JSON", "info");
    }
  }
  onMount(onLoad);
</script>

<section
  class="feature-section first-section"
  aria-labelledby="browser-access-heading"
>
  <div class="section-heading">
    <div>
      <h2 id="browser-access-heading">Browser profile access</h2>
      <p>Discover the browser profiles installed on this Mac.</p>
    </div>
  </div>
  <div class="panel integration-card">
    <span class="integration-icon"><Icon name="profiles" size={22} /></span>
    <div>
      <h3>Refresh browser profile discovery</h3>
      <p>
        Missing a profile? Choose its browser folder in the Mac folder picker.
        The available browsers and profiles will refresh after access is granted.
      </p>
      <button
        class="button secondary access-button"
        disabled={requestingBrowserAccess}
        onclick={onGrantBrowserAccess}
        >{requestingBrowserAccess
          ? "Waiting for folder access…"
          : "Grant browser profile access…"}</button
      >
    </div>
  </div>
</section>
<section class="feature-section" aria-labelledby="automation-heading">
  <div class="section-heading">
    <div>
      <h2 id="automation-heading">Shortcuts & automation</h2>
      <p>Route links from Shortcuts, scripts, and other apps.</p>
    </div>
  </div>
  <div class="panel integration-card">
    <span class="integration-icon"><Icon name="route" size={22} /></span>
    <div>
      <h3>Use a URL in Shortcuts</h3>
      <p>
        Add a URL action with one of the links below, followed by Open URLs.
        Replace the example destination with your input, using URL Encode for
        dynamic values.
      </p>
      <p>
        The optional Waystile Actions companion adds native Shortcuts actions.
        Open it once after installing, then search for Waystile in the Shortcuts
        action library.
      </p>
    </div>
  </div>
  <details class="integration-details panel automation-examples">
    <summary>URL examples for Shortcuts & scripts</summary>
    <div class="integration-examples">
    <div>
      <h3>Choose a browser for a link</h3>
      <code>{exampleOpen}</code>
    </div>
    <div>
      <h3>Open in a specific Chrome profile</h3>
      <code>{exampleProfile}</code>
    </div>
    <div>
      <h3>Choose where to open the clipboard</h3>
      <code>{exampleClipboard}</code>
    </div>
    </div>
  </details>
  <details class="integration-details">
    <summary>Advanced automation options</summary>
    <p>
      Open-link URLs also accept <code>private=true</code>,
      <code>background=true</code>, and <code>newWindow=true</code>. Browser
      support varies.
    </p>
    <p>
      Use <code
        >{integrationURL(scheme, "set-default", { browser: "Safari" })}</code
      >
      to change the saved default, or
      <code>{integrationURL(scheme, "settings")}</code> to open settings. A JavaScript
      configuration may override the saved default.
    </p>
    {#if info}<dl>
        <dt>Application</dt>
        <dd>{info.appPath}</dd>
        <dt>Bundle ID</dt>
        <dd>{info.bundleId}</dd>
      </dl>{/if}
  </details>
</section>

<section class="feature-section" aria-labelledby="browser-integration-heading">
  <div class="section-heading">
    <div>
      <h2 id="browser-integration-heading">Browser & share integrations</h2>
      <p>Choose a profile from a browser tab, link menu, or the Mac share menu.</p>
    </div>
  </div>
  <div class="panel integration-card">
    <span class="integration-icon"><Icon name="globe" size={22} /></span>
    <div>
      <h3>Browser extensions</h3>
      <p>
        The companion Chromium and Firefox extensions provide toolbar and
        link-menu actions. Safari uses the extension bundled with Waystile
        Actions; enable it in Safari Settings → Extensions after installing the
        companion app.
      </p>
      <p>
        For the Mac share menu, enable the Waystile share extension in System
        Settings → General → Login Items & Extensions → Sharing. Extension
        availability depends on which companion components are installed.
      </p>
      <details class="integration-details"><summary>Use an installed browser web app</summary><p>Enter its existing <code>.app</code> path in Profiles → Browser or custom app. Add the exact profile directory when the app is installed in several browser profiles. Web-app wrappers do not support private browsing.</p></details>
    </div>
  </div>
  <div class="info-panel">
    <Icon name="info" size={18} />
    <div>
      <h3>External links need the default-browser setting</h3>
      <p>
        Select Waystile in System Settings → Desktop & Dock → Default web
        browser. Links clicked inside an existing browser tab need a browser
        extension or an explicit Waystile integration URL.
      </p>
    </div>
  </div>
</section>

<section class="feature-section" aria-labelledby="settings-transfer-heading">
  <div class="section-heading">
    <div>
      <h2 id="settings-transfer-heading">Import & export</h2>
      <p>Back up your routing rules, profiles, and preferences as a settings file.</p>
    </div>
  </div>
  <div class="panel feature-fields">
    <p class="transfer-note">
      JavaScript configuration files are managed separately. Export your current
      settings before replacing them.
    </p>
    {#if dirty}<p class="inline-note">
        Apply or discard your pending changes before importing or exporting
        settings.
      </p>{/if}
    <div class="inline-actions">
      <button
        class="button secondary"
        disabled={dirty || importing}
        onclick={onExport}
        ><Icon name="external" size={14} />Export settings JSON</button
      ><button
        class="button secondary"
        disabled={dirty || importing}
        onclick={() => (showImport = !showImport)}
        ><Icon name="code" size={14} />Import settings JSON</button
      >
    </div>
    {#if exportedJSON}<div class="field export-field">
        <label for="export-json">Exported settings</label><textarea
          id="export-json"
          readonly
          value={exportedJSON}
          bind:this={exportField}
          rows="8"
          spellcheck="false"></textarea><button
          class="button small secondary"
          onclick={copyExport}>Copy JSON</button
        >
      </div>{/if}{#if showImport}<div class="field import-field">
        <label for="import-file">Choose a settings file</label><input
          id="import-file"
          type="file"
          accept=".json,application/json"
          disabled={dirty || importing}
          onchange={readFile}
        />
      </div>
      <div class="field">
        <label for="import-json">Or paste settings JSON</label><textarea
          id="import-json"
          value={importJSON}
          oninput={(event) => inspectImport(event.currentTarget.value)}
          rows="8"
          spellcheck="false"
          disabled={dirty || importing}
          aria-invalid={Boolean(importError)}
          placeholder={JSON.stringify({ defaultBrowser: "Safari", rules: [] })}
        ></textarea>{#if importError}<p class="error-text" role="alert">
            {importError}
          </p>{/if}{#if importSummary}<p class="field-hint">
            {importSummary}
          </p>{/if}
      </div>
      <p class="transfer-note">
        Import replaces your saved JSON settings immediately. Your current
        JavaScript configuration, if any, keeps its priority.
      </p>
      <button
        class="button primary"
        disabled={dirty || importing || !importSummary || Boolean(importError)}
        onclick={() => onImport(importJSON)}
        >{importing ? "Importing…" : "Replace saved settings"}</button
      >{/if}
  </div>
</section>

<style>
  .automation-examples { margin-top: 16px; padding: 16px 20px; }
  .automation-examples > summary { min-height: 32px; padding: 4px 0; cursor: pointer; color: var(--text-primary); font-size: 14px; font-weight: 600; }
  .automation-examples .integration-examples { margin-top: 12px; }
</style>
