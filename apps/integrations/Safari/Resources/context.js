// Send only the link the user right-clicked. Normal clicks stay in Safari.
document.addEventListener("contextmenu", event => {
  const anchor = event.composedPath().find(node =>
    node instanceof HTMLAnchorElement && node.href
  );
  const linkURL = anchor && /^https?:$/i.test(anchor.protocol) ? anchor.href : "";
  safari.extension.setContextMenuEventUserInfo(event, { linkURL });
}, true);
