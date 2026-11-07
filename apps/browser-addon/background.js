/* Browser integrations for Waystile. All launch requests use a URL,
   never executable code. The native app validates the destination again. */
const extensionAPI = typeof browser !== "undefined" ? browser : chrome;
const menus = {
  "finicky-route-link": {title: "Open with Waystile", contexts: ["link"]},
  "finicky-choose-link": {title: "Choose browser or profile…", contexts: ["link"]},
  "finicky-choose-page": {title: "Open this page in another profile…", contexts: ["page"]},
};
function requestURL(raw, prompt = true, tab) {
  if (typeof raw !== "string") return null;
  const target = new URL(raw);
  if (!["http:", "https:"].includes(target.protocol)) return null;
  // This comes from the browser's Tab object, never the page or message body.
  // It remains unauthenticated metadata once transported by an open URL scheme.
  const privacy = typeof tab?.incognito === "boolean"
    ? "&sourcePrivacy=" + (tab.incognito ? "private" : "public") : "";
  return "finicky-plus://open?url=" + encodeURIComponent(target.href) + (prompt ? "&prompt=true" : "") + privacy;
}
function openTarget(raw, tab, prompt) {
  try {
    if (!Number.isInteger(tab?.id) || tab.id < 0) return false;
    const request = requestURL(raw, prompt, tab);
    if (!request) return false;
    // The tab may close between the gesture and navigation; do not log its URL.
    const update = extensionAPI.tabs.update(tab.id, {url: request});
    update?.catch(() => {});
    return true;
  } catch { /* Ignore unsupported or missing URLs. */ }
  return false;
}
extensionAPI.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!message || typeof message !== "object" || Array.isArray(message)
      || message.type !== "finicky-open-link"
      || typeof message.url !== "string" || typeof message.prompt !== "boolean"
      || Object.keys(message).some(key => !["type", "url", "prompt"].includes(key))
      || !sender || sender.id !== extensionAPI.runtime.id) return;
  // runtime supplies sender.tab; the message cannot choose a tab or its privacy.
  sendResponse({accepted: openTarget(message.url, sender.tab, message.prompt)});
});
extensionAPI.runtime.onInstalled.addListener(async () => {
  await extensionAPI.contextMenus.removeAll();
  for (const [id, options] of Object.entries(menus)) extensionAPI.contextMenus.create({id, ...options});
});
extensionAPI.contextMenus.onClicked.addListener((info, tab) => {
  if (!Object.hasOwn(menus, info.menuItemId)) return;
  openTarget(info.linkUrl || info.pageUrl || tab?.url, tab, info.menuItemId !== "finicky-route-link");
});
(extensionAPI.action || extensionAPI.browserAction).onClicked.addListener(tab => openTarget(tab?.url, tab, true));
extensionAPI.commands.onCommand.addListener(async command => {
  if (command !== "choose-profile") return;
  const tabs = await extensionAPI.tabs.query({active: true, currentWindow: true});
  if (tabs[0]) openTarget(tabs[0].url, tabs[0], true);
});
