// Follow macOS appearance, including changes while settings are open.
const appearance = window.matchMedia("(prefers-color-scheme: dark)");
const updateAppearance = () => document.documentElement.classList.toggle("dark", appearance.matches);
updateAppearance();
appearance.addEventListener("change", updateAppearance);
if (import.meta.hot) import.meta.hot.dispose(() => appearance.removeEventListener("change", updateAppearance));
