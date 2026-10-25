<script lang="ts">
  import { toast } from "../lib/toast";
  import Icon from "./StudioIcon.svelte";
</script>

<div class="toast-container" aria-label="Notifications">
  {#each $toast as notification (notification.id)}
    <div class="toast toast-{notification.type}" role={notification.type === "error" ? "alert" : "status"} aria-atomic="true">
      <div class="toast-content">
        <span class="toast-icon"><Icon name={notification.type === "success" ? "check" : "info"} size={18} /></span>
        <span class="toast-message">{notification.message}</span>
        <button class="toast-dismiss" type="button" aria-label={`Dismiss: ${notification.message}`} onclick={() => toast.remove(notification.id)}><Icon name="close" size={16} /></button>
      </div>
      {#if notification.extra}
        <details class="toast-details"><summary>Details</summary><pre>{notification.extra}</pre></details>
      {/if}
    </div>
  {/each}
</div>

<style>
  .toast-container { position: fixed; bottom: 80px; right: 24px; z-index: 9999; display: flex; flex-direction: column; gap: 8px; pointer-events: none; max-height: min(60vh, 520px); overflow-y: auto; padding: 4px; }
  .toast { padding: 12px 12px 12px 16px; border-radius: 10px; background: var(--card-bg); border: 1px solid var(--card-border); box-shadow: var(--shadow); width: min(400px, calc(100vw - 48px)); pointer-events: auto; color: var(--text-primary); }
  .toast-content { display: flex; align-items: flex-start; gap: 12px; }
  .toast-icon { display: grid; place-items: center; min-height: 32px; color: var(--accent-color); }
  .toast-message { flex: 1; font-size: 14px; line-height: 1.45; padding-top: 6px; overflow-wrap: anywhere; }
  .toast-dismiss { display: grid; place-items: center; border: 0; background: transparent; color: var(--text-secondary); min-height: 32px; min-width: 32px; border-radius: 6px; padding: 0; }
  .toast-dismiss:hover { background: var(--button-hover); color: var(--text-primary); }
  .toast-details { margin: 4px 0 0 30px; font-size: 12px; }
  .toast-details summary { cursor: pointer; min-height: 32px; padding: 6px 0; color: var(--text-secondary); }
  .toast-details pre { background: var(--inset-bg); color: var(--text-primary); padding: 12px; margin: 4px 0 0; border-radius: 6px; font-size: 12px; line-height: 1.5; overflow: auto; max-height: 160px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .toast-success .toast-icon { color: var(--success, var(--log-success)); }
  .toast-error { border-color: var(--danger, var(--log-error)); }
  .toast-error .toast-icon { color: var(--danger, var(--log-error)); }
  .toast-warning .toast-icon { color: var(--warning, var(--log-warning)); }
</style>
