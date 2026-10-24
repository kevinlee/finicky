/* Browser integrations for the Finicky fork. All launch requests use a URL,
   never executable code. The native app validates the destination again. */
const extensionAPI = typeof browser !== "undefined" ? browser : chrome;
const menus = {
  "finicky-route-link": {title: "Open with Finicky", contexts: ["link"]},
  "finicky-choose-link": {title: "Choose browser or profile…", contexts: ["link"]},
  "finicky-choose-page": {title: "Open this page in another profile…", contexts: ["page"]},
};
function requestURL(raw, prompt = true) {
  const target = new URL(raw);
  if (!["http:", "https:"].includes(target.protocol)) return null;
  return "finicky-plus://open?url=" + encodeURIComponent(target.href) + (prompt ? "&prompt=true" : "");
}
function openTarget(raw, tab, prompt) {
  try {
    const request = requestURL(raw, prompt);
    if (request && tab?.id != null) extensionAPI.tabs.update(tab.id, {url: request});
  } catch { /* Ignore unsupported or missing URLs. */ }
}
extensionAPI.runtime.onInstalled.addListener(async () => {
  await extensionAPI.contextMenus.removeAll();
  for (const [id, options] of Object.entries(menus)) extensionAPI.contextMenus.create({id, ...options});
});
extensionAPI.contextMenus.onClicked.addListener((info, tab) => {
  if (!(info.menuItemId in menus)) return;
  openTarget(info.linkUrl || info.pageUrl || tab?.url, tab, info.menuItemId !== "finicky-route-link");
});
(extensionAPI.action || extensionAPI.browserAction).onClicked.addListener(tab => openTarget(tab.url, tab, true));
extensionAPI.commands.onCommand.addListener(async command => {
  if (command !== "choose-profile") return;
  const tabs = await extensionAPI.tabs.query({active: true, currentWindow: true});
  if (tabs[0]) openTarget(tabs[0].url, tabs[0], true);
});
