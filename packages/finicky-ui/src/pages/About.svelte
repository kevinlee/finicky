<script lang="ts">
  import Icon from "../components/StudioIcon.svelte";
  import waystileIcon from "../../../../design/waystile-icon.svg";
  import type { UpdateInfo } from "../types";

  let {
    version,
    updateInfo = null,
    checkingForUpdates = false,
    publishingUpdate = false,
    installingUpdate = false,
    onCheckUpdates = () => {},
    onPublishUpdate = () => {},
    onInstallUpdate = () => {},
  }: {
    version: string;
    updateInfo?: UpdateInfo | null;
    checkingForUpdates?: boolean;
    publishingUpdate?: boolean;
    installingUpdate?: boolean;
    onCheckUpdates?: () => void;
    onPublishUpdate?: () => void;
    onInstallUpdate?: () => void;
  } = $props();

  const effectiveVersion = $derived(version || updateInfo?.currentVersion || "0.1.0");
  const effectiveCommit = $derived(
    updateInfo?.currentCommit && updateInfo.currentCommit !== "dev" ? updateInfo.currentCommit : "",
  );
</script>

<section class="about-page" aria-labelledby="about-product-heading">
  <div class="about-product panel">
    <img class="about-mark" src={waystileIcon} alt="" />
    <div>
      <h2 id="about-product-heading">Waystile</h2>
      <p class="about-version">
        Version {effectiveVersion}{effectiveCommit ? ` (${effectiveCommit})` : ""} · Made for macOS
      </p>
      <p class="about-description">Every link in the right place. Route links to your browsers and profiles, choose a destination when you need to, and share routing settings and updates between your Macs.</p>
      <div class="about-actions">
        {#if updateInfo?.hasUpdate && updateInfo?.canInstallAutomatically}
          <button
            class="button small primary"
            disabled={installingUpdate || updateInfo.archivePendingDownload}
            onclick={onInstallUpdate}
          >
            {installingUpdate ? "Updating…" : `Update to ${updateInfo.version}${updateInfo.latestCommit ? ` (${updateInfo.latestCommit})` : ""} & Relaunch`}
          </button>
        {/if}
        <button class="button small secondary" disabled={checkingForUpdates || installingUpdate} onclick={onCheckUpdates}>
          {checkingForUpdates ? "Checking…" : "Check for updates"}
        </button>
        <button class="button small secondary" disabled={publishingUpdate || installingUpdate} onclick={onPublishUpdate}>
          {publishingUpdate ? "Publishing…" : "Publish build to iCloud"}
        </button>
        <a href="https://github.com/kevincorvallis/waystile" target="_blank" rel="noopener noreferrer" class="button small secondary">View project on GitHub <Icon name="external" size={14} /></a>
      </div>
    </div>
  </div>

  <div class="feature-section">
    <div class="section-heading"><div><h2>Built on open source</h2><p>The foundations that make this app possible.</p></div></div>
    <div class="panel about-credits">
      <div><h3>Finicky</h3><p>Original app by <a href="https://github.com/johnste" target="_blank" rel="noopener noreferrer">John Sterling</a> and the <a href="https://github.com/johnste/finicky/graphs/contributors" target="_blank" rel="noopener noreferrer">Finicky contributors</a>.</p></div>
      <div><h3>Waystile icon</h3><p>An original route mark created for Waystile.</p></div>
      <div><h3>Interface foundations</h3><p><a href="https://lucide.dev" target="_blank" rel="noopener noreferrer">Lucide icons</a> and <a href="https://www.radix-ui.com/colors" target="_blank" rel="noopener noreferrer">Radix Colors</a>.</p></div>
      <div><h3>Support the foundation</h3><p><a href="https://github.com/sponsors/johnste" target="_blank" rel="noopener noreferrer">Sponsor the original Finicky project <Icon name="external" size={12} /></a></p></div>
    </div>
  </div>
</section>

<style>
  .about-page { max-width: 760px; }
  .about-product { display: flex; align-items: flex-start; gap: 20px; padding: 24px; }
  .about-mark { width: 64px; height: 64px; flex-shrink: 0; }
  .about-product h2 { font-size: 22px; font-weight: 650; letter-spacing: -0.4px; }
  .about-version { font-size: 12px; color: var(--text-secondary); margin-top: 4px; }
  .about-description { margin-top: 16px; color: var(--text-secondary); font-size: 14px; line-height: 1.6; max-width: 520px; }
  .about-actions { margin-top: 20px; display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .about-credits { padding: 0; }
  .about-credits > div { padding: 20px; }
  .about-credits > div + div { border-top: 1px solid var(--border-color); }
  .about-credits h3 { font-size: 14px; font-weight: 600; }
  .about-credits p { font-size: 14px; margin-top: 4px; color: var(--text-secondary); }
  .about-credits a { text-underline-offset: 3px; }
  .about-credits a:hover { text-decoration: underline; }
  .about-credits p a :global(svg) { display: inline-block; vertical-align: middle; margin-left: 4px; }
  @media (max-width: 560px) { .about-product { flex-direction: column; } }
</style>
