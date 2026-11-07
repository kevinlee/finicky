import { writable } from "svelte/store";

export type ToastType = "success" | "error" | "info" | "warning";

export interface Toast {
  id: string;
  message: string;
  extra?: string;
  type: ToastType;
  duration?: number;
  timeoutId?: number;
  key?: number;
}

const MAX_TRANSIENT_TOASTS = 3;
const DEFAULT_DURATION = 5000;

export function createToastStore() {
  const { subscribe, update } = writable<Toast[]>([]);

  function show(
    message: string,
    type: ToastType = "info",
    extra?: string,
    duration?: number,
  ) {
    // Problems remain visible until dismissed. Callers can still choose a duration.
    const resolvedDuration = duration ?? (type === "error" || type === "warning" ? 0 : DEFAULT_DURATION);
    let id = "";
    update((toasts) => {
      const existing = toasts.find((entry) => entry.message === message && entry.type === type);
      if (existing?.timeoutId !== undefined) clearTimeout(existing.timeoutId);
      id = existing?.id || `${Date.now()}-${Math.random()}`;
      const timeoutId = resolvedDuration > 0
        ? setTimeout(() => remove(id), resolvedDuration) as unknown as number
        : undefined;
      const next: Toast = { id, message, extra, type, duration: resolvedDuration, timeoutId, key: Date.now() };
      const updated = existing
        ? toasts.map((entry) => entry.id === id ? next : entry)
        : [...toasts, next];
      // New success notices must never silently replace a persistent problem.
      const transient = updated.filter((entry) => (entry.duration || 0) > 0);
      const expired = new Set(transient.slice(0, Math.max(0, transient.length - MAX_TRANSIENT_TOASTS)).map((entry) => entry.id));
      return updated.filter((entry) => {
        if (!expired.has(entry.id)) return true;
        if (entry.timeoutId !== undefined) clearTimeout(entry.timeoutId);
        return false;
      });
    });
    return id;
  }

  function remove(id: string) {
    update((toasts) => {
      const entry = toasts.find((value) => value.id === id);
      if (entry?.timeoutId !== undefined) clearTimeout(entry.timeoutId);
      return toasts.filter((value) => value.id !== id);
    });
  }

  function clear() {
    update((toasts) => {
      toasts.forEach((entry) => {
        if (entry.timeoutId !== undefined) clearTimeout(entry.timeoutId);
      });
      return [];
    });
  }

  return {
    subscribe,
    show,
    success: (message: string, duration?: number) => show(message, "success", undefined, duration),
    error: (message: string, error: string, duration?: number) => show(message, "error", error, duration),
    info: (message: string, duration?: number) => show(message, "info", undefined, duration),
    warning: (message: string, duration?: number) => show(message, "warning", undefined, duration),
    remove,
    clear,
  };
}

export const toast = createToastStore();
