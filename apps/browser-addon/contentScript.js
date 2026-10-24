// Option-click routes a link; Option-Shift-click forces the native picker.
// Synthetic events never invoke external applications.
window.addEventListener("click", event => {
  if (!event.isTrusted || !event.altKey || event.button !== 0) return;
  const origin = event.target instanceof Element ? event.target : event.target?.parentElement;
  const anchor = origin?.closest("a[href]");
  if (!anchor) return;
  try {
    const target = new URL(anchor.href, document.baseURI);
    if (!["http:", "https:"].includes(target.protocol)) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    window.location.href = "finicky-plus://open?url=" + encodeURIComponent(target.href) + (event.shiftKey ? "&prompt=true" : "");
  } catch { /* Ignore unsupported links. */ }
}, true);
