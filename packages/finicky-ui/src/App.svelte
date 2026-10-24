<script lang="ts">
  import { tick, onDestroy } from "svelte";
  import Icon from "./components/StudioIcon.svelte";
  import LogViewer from "./pages/LogViewer.svelte";
  import About from "./pages/About.svelte";
  import History from "./pages/History.svelte";
  import Integrations from "./pages/Integrations.svelte";
  import FeaturePreferences from "./components/FeaturePreferences.svelte";
  import {
    effectiveFeatures,
    shortenerError,
    shortcutError,
  } from "./lib/featuresModel";
  import ToastContainer from "./components/ToastContainer.svelte";
  import { toast } from "./lib/toast";
  import { normalizeRulesFile } from "./lib/rulesModel";
  import {
    defaultDestinationValue,
    matchesFavorite,
    effectiveOption,
  } from "./lib/settingsModel";
  import {
    testProfileDescription,
    testResultTarget,
    isPromptResult,
  } from "./lib/testResult";
  import type { NativeTestResult } from "./lib/testResult";
  import type {
    ConfigInfo,
    HistoryEntry,
    IntegrationInfo,
    ConfigOptions,
    LogEntry,
    Rule,
    RulesFile,
    SavedProfile,
    UpdateInfo,
  } from "./types";
  type Page =
    | "routing"
    | "profiles"
    | "test"
    | "preferences"
    | "history"
    | "integrations"
    | "diagnostics"
    | "about";
  const navigation: { id: Page; icon: string; label: string }[] = [
    { id: "routing", icon: "route", label: "Routing" },
    { id: "profiles", icon: "profiles", label: "Profiles" },
    { id: "test", icon: "test", label: "Test a link" },
    { id: "preferences", icon: "preferences", label: "Preferences" },
    { id: "history", icon: "history", label: "History" },
    { id: "integrations", icon: "code", label: "Integrations" },
  ];
  const knownApps = [
    { id: "com.tinyspeck.slackmacgap", name: "Slack", color: "#8d5e8c" },
    { id: "com.openai.codex", name: "ChatGPT", color: "#4e8171" },
    { id: "com.anthropic.claudefordesktop", name: "Claude", color: "#b97e60" },
    { id: "com.todesktop.230313mzl4w4u92", name: "Cursor", color: "#636b7a" },
  ];
  const subtitles: Record<Page, string> = {
    routing: "A home for every link.",
    profiles: "Keep your different worlds a little more separate.",
    test: "See exactly where a link will land.",
    preferences: "Make Finicky Plus feel at home on your Mac.",
    history: "Find a link’s path, or give it a fresh start.",
    integrations: "Your browser routing, connected to your workflow.",
    diagnostics: "A closer look at what’s happening.",
    about: "A little utility with a sense of direction.",
  };
  const pageTitles: Record<Page, string> = {
    routing: "Routing",
    profiles: "Profiles",
    test: "Test a link",
    preferences: "Preferences",
    history: "History",
    integrations: "Integrations",
    diagnostics: "Diagnostics",
    about: "About Finicky Plus",
  };
  const optionDefinitions: {
    key: keyof ConfigOptions;
    name: string;
    hint: string;
    fallback: boolean;
  }[] = [
    {
      key: "keepRunning",
      name: "Keep running",
      hint: "Stay ready in the background to route your links.",
      fallback: true,
    },
    {
      key: "hideIcon",
      name: "Hide menu bar icon",
      hint: "Keep Finicky Plus out of the menu bar. Takes effect after restarting Finicky Plus.",
      fallback: false,
    },
    {
      key: "logRequests",
      name: "Log link requests",
      hint: "Write routed URLs to the diagnostic log on disk. Changes apply immediately.",
      fallback: false,
    },
    {
      key: "checkForUpdates",
      name: "Check for updates",
      hint: "Available when an update source is configured for this build.",
      fallback: true,
    },
  ];
  let page: Page = "routing";
  let version = "";
  let config: ConfigInfo = { configPath: "" };
  let updateInfo: UpdateInfo | null = null;
  let logs: LogEntry[] = [];
  let history: HistoryEntry[] = [];
  let historyLoaded = false;
  let clearingHistory = false;
  let integrations: IntegrationInfo | null = null;
  let exportedJSON = "";
  let importing = false;
  let requestingBrowserAccess = false;
  let importTimer: ReturnType<typeof setTimeout> | undefined;
  let installedBrowsers: string[] = [];
  let discoveredProfiles: {
    id: string;
    name: string;
    profile: string;
    profileDirectory: string;
  }[] = [];
  let draft: RulesFile = { defaultBrowser: "", rules: [] };
  let baseline: RulesFile = clone(draft);
  let incoming: RulesFile | null = null;
  let submitted: RulesFile | null = null;
  let loaded = false;
  let saving = false;
  let externalChange = false;
  let saveError = "";
  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  let savedOnce = false;
  let testUrl = "";
  let sourceApp = "";
  let testing = false;
  let testResult: NativeTestResult | null = null;
  let testTimer: ReturnType<typeof setTimeout> | undefined;
  const isPreview = Boolean(window.finickyPreview);
  $: profiles = draft.profiles ?? [];
  $: dirty = JSON.stringify(draft) !== JSON.stringify(baseline);
  $: numErrors = logs.filter(
    (log) => log.level?.toLowerCase() === "error",
  ).length;
  $: defaultSelection = defaultDestinationValue(draft, profiles);
  $: browserChoices = [
    ...new Set(
      [
        "Google Chrome",
        "Safari",
        ...installedBrowsers,
        ...profiles.map((p) => p.browser),
        draft.defaultBrowser,
      ].filter((browser) => Boolean(browser) && browser !== "prompt"),
    ),
  ];
  $: validationError =
    (((Boolean(draft.defaultBrowser) || !config.isJSConfig) &&
      draft.defaultPrivate &&
      unsupportedPrivate(draft.defaultBrowser || "Safari")) ||
    (draft.features?.alternativePrivate &&
      unsupportedPrivate(draft.features.alternativeBrowser || ""))
      ? "Safari private browsing is unavailable through this integration. Choose another destination or turn off private browsing."
      : "") ||
    shortenerError(effectiveFeatures(draft.features).extraShorteners) ||
    (draft.rules.find(
      (rule) =>
        rule.enabled !== false &&
        (!rule.browser ||
          (!rule.match?.some(Boolean) && !rule.sourceApps?.some(Boolean))),
    )
      ? "Every enabled rule needs a source or website pattern, and a browser."
      : "");
  function navigate(next: Page) {
    page = next;
    window.scrollTo({ top: 0, behavior: "instant" });
  }
  function clone<T>(value: T): T {
    return JSON.parse(JSON.stringify(value));
  }
  function safeColor(value?: string): string {
    return /^#[0-9a-f]{6}$/i.test(value || "") ? value! : "#698078";
  }
  function appName(id: string): string {
    return knownApps.find((a) => a.id === id)?.name ?? id;
  }
  type Target = {
    browser: string;
    private?: boolean;
    profile?: string;
    profileDirectory?: string;
  };
  function sameTarget(a: Target, b: Target): boolean {
    return (
      a.browser === b.browser &&
      (a.profileDirectory || "") === (b.profileDirectory || "") &&
      (a.profileDirectory ? true : (a.profile || "") === (b.profile || ""))
    );
  }
  function targetProfile(target: Target): SavedProfile | undefined {
    return profiles.find((p) => matchesFavorite(p, target));
  }
  function targetName(target: Target): string {
    if (target.browser === "prompt") return "Choose browser / profile";
    return (
      targetProfile(target)?.name ||
      target.profile ||
      target.profileDirectory ||
      target.browser ||
      "Safari"
    );
  }
  function targetColor(target: Target): string {
    return safeColor(targetProfile(target)?.color);
  }
  function ruleName(rule: Rule): string {
    return (
      rule.name ||
      (rule.sourceApps?.length
        ? `${rule.sourceApps.map(appName).join(", ")} links`
        : rule.match?.[0] || "Untitled rule")
    );
  }
  function cleanTarget(target: Target): Target {
    return {
      browser: target.browser,
      ...(target.profile ? { profile: target.profile } : {}),
      ...(target.profileDirectory
        ? { profileDirectory: target.profileDirectory }
        : {}),
    };
  }
  function mutate(fn: (model: RulesFile) => void) {
    if (saving || importing) return;
    const next = clone(draft);
    fn(next);
    draft = next;
    saveError = "";
    savedOnce = false;
  }
  function send(message: unknown) {
    window.webkit?.messageHandlers?.finicky?.postMessage(
      JSON.stringify(message),
    );
  }
  function receive(raw: any) {
    let message: any;
    try {
      message = typeof raw === "string" ? JSON.parse(raw) : raw;
    } catch {
      return;
    }
    switch (message.type) {
      case "version":
        version = message.message;
        break;
      case "buildInfo":
        break;
      case "config":
        if (message.message) {
          config = message.message;
          testResult = null;
        }
        break;
      case "updateInfo":
        updateInfo = message.message;
        break;
      case "installedBrowsers":
        installedBrowsers = message.message || [];
        break;
      case "browserProfiles":
        if (message.message?.browser === profileForm.browser) {
          discoveredProfiles = Array.isArray(message.message.profileDetails)
            ? message.message.profileDetails
            : [];
        }
        break;
      case "history":
        history = Array.isArray(message.message) ? message.message : [];
        historyLoaded = true;
        break;
      case "historyCleared":
        clearingHistory = false;
        if (message.message?.error || message.message?.success === false)
          toast.show(
            "History could not be cleared",
            "error",
            message.message?.error,
          );
        else {
          history = [];
          toast.show("Link history cleared", "success");
        }
        break;
      case "browserAccessResult":
        requestingBrowserAccess = false;
        if (message.message?.success) {
          toast.show("Browser profile access granted", "success");
          send({ type: "getInstalledBrowsers" });
          discoverProfiles(profileForm.browser);
        } else if (message.message?.error)
          toast.show(
            "Browser profile access was not granted",
            "error",
            message.message.error,
          );
        else toast.show("Browser profile access was not changed", "info");
        break;
      case "integrations":
        integrations = message.message;
        break;
      case "settingsExport":
        if (message.message?.error)
          toast.show(
            "Settings could not be exported",
            "error",
            message.message.error,
          );
        else exportedJSON = message.message?.json || "";
        break;
      case "settingsImported":
        importing = false;
        clearTimeout(importTimer);
        if (message.message?.success) {
          toast.show("Settings imported", "success");
          send({ type: "getRules" });
          send({ type: "getHistory" });
        } else
          toast.show(
            "Settings were not imported",
            "error",
            message.message?.error,
          );
        break;
      case "openPickerResult":
      case "openClipboardResult":
        if (message.message?.error || message.message?.success === false)
          toast.show(
            "The link could not be opened",
            "error",
            message.message?.error,
          );
        else if (isPreview)
          toast.show("Native action is simulated in this preview", "info");
        break;
      case "rules": {
        const next = normalizeRulesFile(message.message);
        if (saving) incoming = next;
        else if (!loaded || !dirty) {
          draft = next;
          baseline = clone(next);
          loaded = true;
          externalChange = false;
          testResult = null;
        } else if (JSON.stringify(next) !== JSON.stringify(baseline)) {
          incoming = next;
          externalChange = true;
        }
        break;
      }
      case "saveRulesSuccess":
        if (!submitted) break;
        if (submitted) {
          draft = clone(incoming || submitted);
          baseline = clone(draft);
        }
        saving = false;
        submitted = null;
        incoming = null;
        externalChange = false;
        saveError = "";
        savedOnce = true;
        testResult = null;
        clearTimeout(saveTimer);
        toast.show(
          isPreview
            ? "Preview changes saved"
            : "Changes written to Finicky Plus",
          "success",
        );
        break;
      case "saveRulesError":
        saving = false;
        submitted = null;
        incoming = null;
        clearTimeout(saveTimer);
        saveError =
          message.message?.error ||
          "Finicky Plus could not save these changes.";
        toast.show("Changes were not applied", "error", saveError);
        break;
      case "testUrlResult":
        testResult = message.message;
        testing = false;
        clearTimeout(testTimer);
        break;
      default:
        try {
          const entry =
            typeof message.message === "string"
              ? JSON.parse(message.message)
              : message.message || message;
          if (entry.level) logs = [...logs, entry];
        } catch {
          /* Unrecognized native status message. */
        }
    }
  }
  const queued = window.finicky?._queue ?? [];
  window.finicky = { sendMessage: send, receiveMessage: receive };
  for (const message of queued) receive(message);
  send({ type: "getRules" });
  send({ type: "getInstalledBrowsers" });
  onDestroy(() => {
    clearTimeout(saveTimer);
    clearTimeout(testTimer);
    clearTimeout(importTimer);
  });
  function apply() {
    if (!dirty || saving || importing || externalChange || validationError)
      return;
    saving = true;
    saveError = "";
    submitted = clone(draft);
    incoming = null;
    send({ type: "saveRules", payload: submitted });
    saveTimer = setTimeout(() => {
      saving = false;
      submitted = null;
      externalChange = true;
      saveError =
        "Finicky Plus has not confirmed this save. Reload the configuration to check what was written before applying again.";
      send({ type: "getRules" });
    }, 12000);
  }
  function discard() {
    draft = clone(incoming || baseline);
    baseline = clone(draft);
    incoming = null;
    externalChange = false;
    saveError = "";
    closeModal();
  }
  function moveRule(index: number, direction: number) {
    mutate((model) => {
      const destination = index + direction;
      if (destination < 0 || destination >= model.rules.length) return;
      [model.rules[index], model.rules[destination]] = [
        model.rules[destination],
        model.rules[index],
      ];
    });
  }
  function setDefault(value: string) {
    if (value === "custom") return;
    mutate((model) => {
      const target: Target | undefined = value.startsWith("profile:")
        ? profiles.find((p) => p.id === value.slice(8))
        : { browser: value.slice(8) };
      if (!target) return;
      model.defaultBrowser = target.browser;
      model.defaultPrivate = Boolean(target.private);
      delete model.defaultProfile;
      delete model.defaultProfileDirectory;
      if (target.profile) model.defaultProfile = target.profile;
      if (target.profileDirectory)
        model.defaultProfileDirectory = target.profileDirectory;
    });
  }
  let modal:
    "rule" | "profile" | "deleteRule" | "deleteProfile" | "discard" | "" = "";
  let modalElement: HTMLDialogElement;
  let modalError = "";
  let editIndex = -1;
  let ruleForm: Rule = { match: [], browser: "Google Chrome" };
  let matchType = "app";
  let matchText = "";
  let selectedApps: string[] = [];
  let customApps = "";
  let selectedTarget = "custom";
  let profileForm: SavedProfile = {
    id: "",
    name: "",
    browser: "Google Chrome",
    color: "#4569db",
  };
  let originalProfile: SavedProfile | null = null;
  let updateReferences = true;
  async function openModal(type: typeof modal) {
    modalError = "";
    modal = type;
    await tick();
    modalElement.showModal();
  }
  function closeModal() {
    modalElement?.close();
    modal = "";
    modalError = "";
  }
  function editRule(index = -1) {
    editIndex = index;
    ruleForm =
      index >= 0
        ? clone(draft.rules[index])
        : {
            name: "",
            match: [],
            sourceApps: [],
            ...cleanTarget(profiles[0] || { browser: "Google Chrome" }),
            enabled: true,
          };
    matchType = ruleForm.sourceApps?.length
      ? ruleForm.match?.length
        ? "both"
        : "app"
      : "website";
    if (index < 0) matchType = "app";
    matchText = (ruleForm.match || []).join("\n");
    selectedApps = (ruleForm.sourceApps || []).filter((id) =>
      knownApps.some((app) => app.id === id),
    );
    customApps = (ruleForm.sourceApps || [])
      .filter((id) => !knownApps.some((app) => app.id === id))
      .join("\n");
    selectedTarget =
      ruleForm.browser === "prompt"
        ? "picker"
        : targetProfile(ruleForm)?.id || "custom";
    openModal("rule");
  }
  function selectRuleTarget(value: string) {
    selectedTarget = value;
    if (value === "picker") {
      ruleForm = {
        ...ruleForm,
        browser: "prompt",
        profile: undefined,
        profileDirectory: undefined,
      };
      return;
    }
    if (value === "custom" && ruleForm.browser === "prompt")
      ruleForm = { ...ruleForm, browser: "Google Chrome" };
    const selected = profiles.find((p) => p.id === value);
    if (selected) {
      delete ruleForm.profile;
      delete ruleForm.profileDirectory;
      ruleForm = {
        ...ruleForm,
        ...cleanTarget(selected),
        private: selected.private || false,
      };
    }
  }
  function saveRule(event: SubmitEvent) {
    event.preventDefault();
    const next = clone(ruleForm);
    if (next.private && unsupportedPrivate(next.browser)) {
      modalError =
        "Safari cannot open a private window through this integration. Turn off private browsing or choose a supported browser.";
      return;
    }
    next.name = next.name?.trim();
    next.match =
      matchType === "app"
        ? []
        : matchText
            .split("\n")
            .map((v) => v.trim())
            .filter(Boolean);
    next.sourceApps =
      matchType === "website"
        ? []
        : [
            ...new Set([
              ...selectedApps,
              ...customApps
                .split(/[\n,]/)
                .map((v) => v.trim())
                .filter(Boolean),
            ]),
          ];
    if (matchType !== "website" && !next.sourceApps.length) {
      modalError = "Choose at least one source app.";
      return;
    }
    if (matchType !== "app" && !next.match.length) {
      modalError = "Enter at least one website pattern.";
      return;
    }
    if (!next.browser.trim()) {
      modalError = "Choose a browser for this rule.";
      return;
    }
    if (
      next.sourceApps.some(
        (id) => !/^[A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z0-9.-]+$/.test(id),
      )
    ) {
      modalError = "Use valid app bundle IDs, such as com.example.app.";
      return;
    }
    if (next.profileDirectory && /[\\/]/.test(next.profileDirectory)) {
      modalError = "Enter the profile folder name only, such as Profile 2.";
      return;
    }
    next.browser = next.browser.trim();
    next.profile = next.profile?.trim();
    next.profileDirectory = next.profileDirectory?.trim();
    if (!next.profile?.trim()) delete next.profile;
    if (!next.profileDirectory?.trim()) delete next.profileDirectory;
    mutate((model) => {
      if (editIndex < 0) model.rules.push(next);
      else model.rules[editIndex] = next;
    });
    closeModal();
  }
  function newProfileId(): string {
    return typeof crypto.randomUUID === "function"
      ? crypto.randomUUID()
      : `profile-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
  }
  function discoverProfiles(browser: string) {
    discoveredProfiles = [];
    send({ type: "getBrowserProfiles", browser });
  }
  function chooseDiscoveredProfile(directory: string) {
    const selected = discoveredProfiles.find(
      (entry) => entry.profileDirectory === directory,
    );
    if (!selected) return;
    profileForm = {
      ...profileForm,
      profileDirectory: selected.profileDirectory,
      profile: undefined,
      name: profileForm.name || selected.name,
    };
  }
  function unsupportedPrivate(browser: string): boolean {
    return browser.toLowerCase().includes("safari");
  }
  function editProfile(p?: SavedProfile) {
    originalProfile = p ? clone(p) : null;
    profileForm = p
      ? clone(p)
      : {
          id: newProfileId(),
          name: "",
          browser: "Google Chrome",
          color: ["#39836b", "#4569db", "#a4454b"][profiles.length % 3],
        };
    updateReferences = true;
    discoverProfiles(profileForm.browser);
    openModal("profile");
  }
  function saveProfile(event: SubmitEvent) {
    event.preventDefault();
    const next = clone(profileForm);
    if (next.private && unsupportedPrivate(next.browser)) {
      modalError =
        "Safari cannot open a private window through this integration. Turn off private browsing or choose a supported browser.";
      return;
    }
    next.name = next.name.trim();
    next.browser = next.browser.trim();
    next.profileDirectory = next.profileDirectory?.trim();
    next.profile = next.profile?.trim();
    next.shortcut = next.shortcut?.trim().toLowerCase();
    const keyError = shortcutError(next.shortcut || "", profiles, next.id);
    if (keyError) {
      modalError = keyError;
      return;
    }
    if (!next.name || !next.browser) {
      modalError = "Add a name and a browser for this profile.";
      return;
    }
    if (
      next.profileDirectory &&
      (/[\\/]/.test(next.profileDirectory) ||
        [".", ".."].includes(next.profileDirectory))
    ) {
      modalError =
        "Use the final profile folder name, such as Default or Profile 2.";
      return;
    }
    if (next.profileDirectory) delete next.profile;
    mutate((model) => {
      const previous = originalProfile;
      model.profiles = (model.profiles || []).filter((p) => p.id !== next.id);
      if (previous)
        model.profiles.splice(
          profiles.findIndex((p) => p.id === next.id),
          0,
          next,
        );
      else model.profiles.push(next);
      if (previous && updateReferences && !sameTarget(previous, next)) {
        model.rules = model.rules.map((rule) => {
          if (!sameTarget(rule, previous)) return rule;
          const updated = { ...rule };
          delete updated.profile;
          delete updated.profileDirectory;
          return { ...updated, ...cleanTarget(next) };
        });
        if (
          matchesFavorite(previous, {
            browser: model.defaultBrowser,
            profile: model.defaultProfile,
            profileDirectory: model.defaultProfileDirectory,
            private: model.defaultPrivate,
          })
        ) {
          model.defaultBrowser = next.browser;
          delete model.defaultProfile;
          delete model.defaultProfileDirectory;
          if (next.profile) model.defaultProfile = next.profile;
          if (next.profileDirectory)
            model.defaultProfileDirectory = next.profileDirectory;
        }
      }
    });
    closeModal();
  }
  function grantBrowserAccess() {
    if (requestingBrowserAccess) return;
    requestingBrowserAccess = true;
    send({ type: "grantBrowserAccess" });
  }
  function importSettings(json: string) {
    if (dirty || saving || importing) return;
    importing = true;
    send({ type: "importSettings", json });
    importTimer = setTimeout(() => {
      importing = false;
      toast.show(
        "Import has not been confirmed",
        "warning",
        "Check the saved settings before importing again.",
      );
      send({ type: "getRules" });
    }, 12000);
  }
  function testTarget(result: typeof testResult): Target {
    return testResultTarget(result);
  }
  function testLink(event: SubmitEvent) {
    event.preventDefault();
    try {
      const url = new URL(testUrl);
      if (!["http:", "https:"].includes(url.protocol)) throw new Error();
    } catch {
      testResult = { error: "Enter a full http or https URL." };
      return;
    }
    testing = true;
    testResult = null;
    clearTimeout(testTimer);
    testTimer = setTimeout(() => {
      testing = false;
      testResult = {
        error:
          "Finicky Plus has not returned a result. Check Diagnostics, then try again.",
      };
    }, 10000);
    send({
      type: "testUrl",
      url: testUrl.trim(),
      openerBundleId: sourceApp,
      openerName:
        knownApps.find((app) => app.id === sourceApp)?.name || "Other app",
    });
  }
</script>

<div class="finicky-shell">
  <aside class="sidebar">
    <button
      class="brand"
      onclick={() => navigate("routing")}
      aria-label="Finicky Plus home"
      ><span class="brand-mark"><Icon name="route" size={25} /></span><span
        class="brand-name"
        >Finicky Plus<span class="brand-sub">LINKS, SORTED.</span></span
      ></button
    >
    <div class="sidebar-label">YOUR WORKSPACE</div>
    <nav aria-label="Main navigation">
      {#each navigation as item}<button
          class:active={page === item.id}
          class="nav-item"
          onclick={() => navigate(item.id)}
          aria-current={page === item.id ? "page" : undefined}
          ><Icon name={item.icon} size={18} /><span>{item.label}</span
          >{#if item.id === "routing" && draft.rules.length}<span
              class="nav-count">{draft.rules.length}</span
            >{/if}</button
        >{/each}
    </nav>
    <div class="sidebar-bottom">
      <div class="sidebar-secondary">
        <button
          class:active={page === "diagnostics"}
          class="nav-item"
          onclick={() => navigate("diagnostics")}
          ><Icon name="logs" size={17} /><span>Diagnostics</span
          >{#if numErrors}<span class="error-count">{numErrors}</span
            >{/if}</button
        ><button
          class:active={page === "about"}
          class="nav-item"
          onclick={() => navigate("about")}
          ><Icon name="info" size={17} /><span>About Finicky Plus</span></button
        >
      </div>
      <div class="local-status">
        <span></span>{isPreview ? "Interactive preview" : "Only on your Mac"}
      </div>
      <p>Your profiles. Your rules.<br />No cloud account needed.</p>
      {#if version}<span class="version-label">{version}</span>{/if}
    </div>
  </aside>
  <main class="main-panel">
    {#if isPreview}<div class="preview-banner">
        <span class="preview-dot"></span><strong>Interactive preview</strong
        ><span>Example profiles · changes stay in this browser</span>
      </div>{/if}
    <header class="main-header">
      <div>
        <div class="eyebrow">
          FINICKY PLUS <span>/</span>
          {page === "test" ? "LINK PREVIEW" : "BROWSER ROUTING"}
        </div>
        <h1>{pageTitles[page]}</h1>
        <p>{subtitles[page]}</p>
      </div>
      <div class="config-status" class:pending={dirty}>
        <span></span>{!loaded
          ? "Connecting…"
          : dirty
            ? "Unapplied changes"
            : "Configuration loaded"}
      </div>
    </header>
    <div class="page-content" inert={saving}>
      {#if externalChange}<div class="notice-banner warning">
          <Icon name="info" size={16} /><span
            >Your configuration changed outside this window. Reload it before
            making more changes.</span
          ><button class="text-button" onclick={() => openModal("discard")}
            >Reload</button
          >
        </div>{/if}
      {#if config.isJSConfig && ["routing", "test", "preferences"].includes(page)}<div
          class="notice-banner"
        >
          <Icon name="code" size={16} /><span
            ><strong>JavaScript configuration is active.</strong> Its handlers run
            before saved rules. A saved default handles links that match no rule.
            Saved general preferences override values in your config file.</span
          >
        </div>{/if}
      {#if page === "routing" || page === "profiles"}
        <section class="section">
          <div class="section-heading">
            <div>
              <h2>
                {page === "routing"
                  ? "Your profiles"
                  : "Saved browser profiles"}
                <span class="subtle-count">{profiles.length || ""}</span>
              </h2>
              {#if page === "profiles"}<p>
                  Give each profile a recognizable name and color.
                </p>{/if}
            </div>
            {#if page === "routing" && profiles.length}<button
                class="text-button"
                onclick={() => navigate("profiles")}
                >Manage profiles <Icon name="arrow" size={13} /></button
              >{:else}<button
                class="button secondary small"
                onclick={() => editProfile()}
                ><Icon name="plus" size={14} />Add profile</button
              >{/if}
          </div>
          {#if profiles.length}<div
              class="profile-grid"
              class:expanded={page === "profiles"}
            >
              {#each profiles as p (p.id)}<article
                  class="profile-card"
                  style={`--profile-color:${safeColor(p.color)}`}
                >
                  <div class="profile-card-top">
                    <span class="profile-avatar"
                      >{p.name.slice(0, 1).toUpperCase()}</span
                    ><button
                      class="icon-button"
                      onclick={() => editProfile(p)}
                      aria-label={`Edit ${p.name} profile`}
                      ><Icon name="edit" size={14} /></button
                    >
                  </div>
                  <h3>{p.name}</h3>
                  <div class="profile-meta">
                    <span></span>{p.browser.replace("Google ", "")} · {p.profileDirectory ||
                      p.profile ||
                      "Last used"}
                  </div>
                  {#if p.shortcut || p.hidden || p.private}<div
                      class="profile-flags"
                    >
                      {#if p.shortcut}<span
                          ><kbd>{p.shortcut.toUpperCase()}</kbd> Picker key</span
                        >{/if}{#if p.hidden}<span>Hidden in picker</span
                        >{/if}{#if p.private}<span>Private</span>{/if}
                    </div>{/if}
                  {#if page === "profiles"}<p class="profile-card-description">
                      {draft.rules.filter(
                        (rule) => rule.enabled !== false && sameTarget(rule, p),
                      ).length} enabled routing rules
                    </p>
                    <button
                      class="button secondary full-width"
                      onclick={() => editProfile(p)}
                      >Edit profile <Icon name="edit" size={13} /></button
                    >{/if}
                </article>{/each}
            </div>{:else}<div class="profile-empty">
              <span class="empty-icon"><Icon name="profiles" size={25} /></span>
              <div>
                <h3>A space for every part of your day.</h3>
                <p>
                  Add your personal, work, or school browser profiles to get
                  started.
                </p>
              </div>
              <button class="button secondary" onclick={() => editProfile()}
                >Add a profile <Icon name="plus" size={14} /></button
              >
            </div>{/if}
        </section>
      {/if}
      {#if page === "routing"}
        <section class="section rules-section">
          <div class="section-heading">
            <div>
              <h2>Routing rules</h2>
              <p>Checked from top to bottom. The first match wins.</p>
            </div>
            <button class="button secondary small" onclick={() => editRule()}
              ><Icon name="plus" size={14} />Add rule</button
            >
          </div>
          <div class="rules-panel">
            {#if draft.rules.length}<ol class="rule-list">
                {#each draft.rules as rule, index}<li
                    class="rule-row"
                    class:disabled-rule={rule.enabled === false}
                  >
                    <span class="rule-priority"
                      >{String(index + 1).padStart(2, "0")}</span
                    >
                    <div class="rule-main">
                      <div class="rule-title-row">
                        <h3>{ruleName(rule)}</h3>
                        <span class="rule-type"
                          >{rule.sourceApps?.length
                            ? rule.match.length
                              ? "APP + WEBSITE"
                              : "SOURCE APP"
                            : "WEBSITE"}</span
                        >
                      </div>
                      <div class="rule-conditions">
                        {#each rule.sourceApps || [] as source}<span
                            class="condition-tag"
                            ><span
                              class="app-monogram"
                              style={`--app-color:${knownApps.find((a) => a.id === source)?.color || "#798272"}`}
                              >{appName(source).slice(0, 1)}</span
                            >{appName(source)}</span
                          >{/each}{#each rule.match || [] as pattern}<span
                            class="condition-tag pattern">{pattern}</span
                          >{/each}
                      </div>
                    </div>
                    <div class="rule-destination">
                      <Icon name="arrow" size={15} /><span
                        class="profile-pill"
                        style={`--profile-color:${targetColor(rule)}`}
                        ><span></span>{targetName(rule)}</span
                      >
                    </div>
                    <div class="rule-actions">
                      <button
                        class="switch"
                        class:on={rule.enabled !== false}
                        role="switch"
                        aria-checked={rule.enabled !== false}
                        aria-label={`${rule.enabled !== false ? "Disable" : "Enable"} ${ruleName(rule)}`}
                        onclick={() =>
                          mutate(
                            (model) =>
                              (model.rules[index].enabled =
                                rule.enabled === false),
                          )}><span></span></button
                      ><span class="action-divider"></span><button
                        class="icon-button"
                        aria-label={`Edit ${ruleName(rule)}`}
                        onclick={() => editRule(index)}
                        ><Icon name="edit" size={14} /></button
                      >
                      <div class="reorder-buttons">
                        <button
                          class="icon-button"
                          disabled={index === 0}
                          aria-label={`Move ${ruleName(rule)} up`}
                          onclick={() => moveRule(index, -1)}
                          ><Icon name="up" size={12} /></button
                        ><button
                          class="icon-button"
                          disabled={index === draft.rules.length - 1}
                          aria-label={`Move ${ruleName(rule)} down`}
                          onclick={() => moveRule(index, 1)}
                          ><Icon name="down" size={12} /></button
                        >
                      </div>
                      <button
                        class="icon-button delete-action"
                        aria-label={`Delete ${ruleName(rule)}`}
                        onclick={() => {
                          editIndex = index;
                          openModal("deleteRule");
                        }}><Icon name="trash" size={14} /></button
                      >
                    </div>
                  </li>{/each}
              </ol>{:else}<div class="empty-state">
                <Icon name="route" size={27} />
                <h3>Every link starts somewhere.</h3>
                <p>Add a rule to send links to the right browser profile.</p>
                <button class="button secondary" onclick={() => editRule()}
                  ><Icon name="plus" size={14} />Add your first rule</button
                >
              </div>{/if}
            <div class="fallback-row">
              <span class="fallback-icon"><Icon name="globe" size={17} /></span>
              <div>
                <h3>Everything else</h3>
                <p>When no rule matches a link</p>
              </div>
              <label class="sr-only" for="default-browser"
                >Default destination</label
              ><select
                id="default-browser"
                class="default-select"
                value={defaultSelection}
                onchange={(event) => setDefault(event.currentTarget.value)}
                ><option value="browser:"
                  >{config.isJSConfig
                    ? `Use JavaScript default${config.defaultBrowser ? ` · ${config.defaultBrowser}` : ""}`
                    : "Safari (default)"}</option
                ><option value="browser:prompt"
                  >Choose with browser picker</option
                >{#each profiles as p}<option value={`profile:${p.id}`}
                    >{p.name}</option
                  >{/each}{#each browserChoices as browser}<option
                    value={`browser:${browser}`}
                    >{browser} · last used profile</option
                  >{/each}{#if defaultSelection === "custom"}<option
                    value="custom"
                    >{draft.defaultBrowser} · {draft.defaultProfileDirectory ||
                      draft.defaultProfile}</option
                  >{/if}</select
              >
            </div>
            {#if (Boolean(draft.defaultBrowser) || !config.isJSConfig) && draft.defaultBrowser !== "prompt"}<div
                class="fallback-private"
              >
                <label class="check-label"
                  ><input
                    type="checkbox"
                    checked={draft.defaultPrivate || false}
                    disabled={saving || importing}
                    onchange={(event) =>
                      mutate((model) => {
                        model.defaultPrivate = event.currentTarget.checked;
                      })}
                  />Use private browsing for the default destination</label
                >{#if unsupportedPrivate(draft.defaultBrowser || "Safari")}<p
                    class="field-hint"
                  >
                    Safari private browsing is unavailable through this
                    integration.
                  </p>{/if}
              </div>{/if}
          </div>
          <div class="help-note">
            <Icon name="info" size={13} />
            <p>
              Move specific website rules above app rules when they should take
              priority. Disabled rules are skipped.
            </p>
          </div>
        </section>
        <button class="test-prompt" onclick={() => navigate("test")}
          ><span class="test-prompt-icon"><Icon name="test" size={18} /></span
          ><span
            ><strong>Give your rules a quick check</strong><span
              >Preview a link’s destination without opening it.</span
            ></span
          ><Icon name="arrow" size={16} /></button
        >
      {:else if page === "profiles"}
        <div class="info-panel">
          <Icon name="info" size={18} />
          <div>
            <h3>A familiar name, an exact destination</h3>
            <p>
              Labels and colors are saved in Finicky Plus. In Chrome, open <code
                >chrome://version</code
              >
              and find <strong>Profile Path</strong>. Its final folder name,
              such as <code>Default</code> or <code>Profile 2</code>, identifies
              the profile even when several share the same name.
            </p>
          </div>
        </div>
        <button class="add-profile-card" onclick={() => editProfile()}
          ><Icon name="plus" size={16} />Add another browser profile</button
        >
      {:else if page === "test"}
        <div class="test-layout">
          <form class="panel test-form" onsubmit={testLink}>
            <div class="section-heading">
              <div>
                <h2>Try a link</h2>
                <p>Tests your applied configuration.</p>
              </div>
              <span class="small-label">PREVIEW</span>
            </div>
            {#if dirty}<div class="inline-note">
                <Icon name="info" size={14} />
                <p>
                  You have unapplied changes. Apply them first to include them
                  in this test.
                </p>
              </div>{/if}
            <div class="field">
              <label for="test-url">Link URL</label><input
                id="test-url"
                type="url"
                bind:value={testUrl}
                oninput={() => (testResult = null)}
                placeholder="https://example.com"
                required
                disabled={testing}
              />
              <p class="field-hint">Use a full http or https link.</p>
            </div>
            <div class="field">
              <label for="test-source">Opened from</label><select
                id="test-source"
                bind:value={sourceApp}
                onchange={() => (testResult = null)}
                disabled={testing}
                ><option value="">Other app</option
                >{#each knownApps as source}<option value={source.id}
                    >{source.name}</option
                  >{/each}</select
              >
            </div>
            <button
              class="button primary full-width"
              type="submit"
              disabled={testing}
              >{#if testing}<span class="spinner"></span>Checking route…{:else}<Icon
                  name="test"
                  size={16}
                />Preview destination{/if}</button
            >
          </form>
          <div class="test-result" aria-live="polite">
            {#if testResult?.error}<div class="inline-error" role="alert">
                <Icon name="info" size={17} />{testResult.error}
              </div>{:else if testResult?.browser || isPromptResult(testResult)}<div
                class="result-card"
              >
                <div class="eyebrow">LINK DESTINATION</div>
                <div class="result-destination">
                  <span
                    class="result-avatar"
                    style={`--profile-color:${targetColor(testTarget(testResult))}`}
                    >{targetName(testTarget(testResult)).slice(0, 1)}</span
                  >
                  <div>
                    <h2>{targetName(testTarget(testResult))}</h2>
                    <p>
                      {#if isPromptResult(testResult)}Finicky Plus will ask you
                        where to open this link.{:else}{testResult?.browser} · {testProfileDescription(
                          testResult,
                        )}{/if}
                    </p>
                  </div>
                </div>
                <div class="result-explanation">
                  <span
                    ><Icon name="check" size={15} />Resolved by {isPreview
                      ? "the preview"
                      : "Finicky Plus"}</span
                  >
                  <p>
                    {knownApps.find((a) => a.id === sourceApp)?.name ||
                      "Other app"} → {targetName(testTarget(testResult))}
                  </p>
                </div>
                <div class="result-url">
                  <Icon name="external" size={13} /><span
                    >{testResult?.url || testUrl}</span
                  >
                </div>
                <p class="fine-print">
                  This checks the route without opening your browser.
                </p>
              </div>{:else}<div class="test-empty">
                <span class="empty-icon"><Icon name="route" size={27} /></span>
                <h3>Follow the route</h3>
                <p>
                  Choose an app and a URL to see where<br />your applied rules
                  send the link.
                </p>
              </div>{/if}
          </div>
        </div>
      {:else if page === "preferences"}
        <div class="section-heading">
          <h2>General</h2>
          {#if config.isJSConfig}<span class="small-label"
              >SAVED VALUES TAKE PRIORITY</span
            >{/if}
        </div>
        <div class="preferences-panel">
          {#each optionDefinitions as option}<div class="preference-row">
              <div>
                <h3>{option.name}</h3>
                <p>{option.hint}</p>
              </div>
              <button
                class="switch"
                class:on={effectiveOption(
                  draft.options,
                  config.options,
                  option.key,
                  option.fallback,
                )}
                role="switch"
                aria-checked={effectiveOption(
                  draft.options,
                  config.options,
                  option.key,
                  option.fallback,
                )}
                aria-label={option.name}
                disabled={saving || importing || !loaded}
                onclick={() =>
                  mutate(
                    (model) =>
                      (model.options = {
                        ...model.options,
                        [option.key]: !effectiveOption(
                          model.options,
                          config.options,
                          option.key,
                          option.fallback,
                        ),
                      }),
                  )}><span></span></button
              >
            </div>{/each}
        </div>
        <FeaturePreferences
          features={draft.features}
          {profiles}
          browsers={browserChoices}
          disabled={saving || importing || !loaded}
          onChange={(features) =>
            mutate((model) => {
              model.features = features;
            })}
          onPreview={() =>
            send({ type: "openPicker", url: "https://example.com" })}
          onClipboard={() => send({ type: "openClipboard" })}
        />
        <div class="config-info panel">
          <span class="config-info-icon"><Icon name="code" size={20} /></span>
          <div>
            <h2>Configuration</h2>
            <p class="config-path">
              {config.configPath || draft.path || "No configuration loaded"}
            </p>
            {#if draft.path && draft.path !== config.configPath}<p
                class="config-path"
              >
                {draft.path}
              </p>{/if}
          </div>
        </div>
        {#if updateInfo?.hasUpdate}<div class="info-panel">
            <Icon name="info" size={18} />
            <div>
              <h3>Finicky Plus {updateInfo.version} is available</h3>
              <p>
                <a
                  href={updateInfo.downloadUrl}
                  target="_blank"
                  rel="noreferrer">Download update</a
                >
                ·
                <a href={updateInfo.releaseUrl} target="_blank" rel="noreferrer"
                  >Release notes</a
                >
              </p>
            </div>
          </div>{/if}
      {:else if page === "history"}
        <History
          entries={history}
          loaded={historyLoaded}
          enabled={effectiveFeatures(baseline.features).historyEnabled}
          clearing={clearingHistory}
          onRefresh={() => send({ type: "getHistory" })}
          onClear={() => {
            clearingHistory = true;
            send({ type: "clearHistory" });
          }}
          onOpen={(url) => send({ type: "openPicker", url })}
        />
      {:else if page === "integrations"}
        <Integrations
          info={integrations}
          {exportedJSON}
          {importing}
          dirty={dirty || saving}
          onLoad={() => send({ type: "getIntegrations" })}
          onExport={() => send({ type: "exportSettings" })}
          onImport={importSettings}
          {requestingBrowserAccess}
          onGrantBrowserAccess={grantBrowserAccess}
        />
      {:else if page === "diagnostics"}<div class="legacy-page">
          <LogViewer messageBuffer={logs} onClearLogs={() => (logs = [])} />
        </div>
      {:else if page === "about"}<div class="legacy-page">
          <About {version} />
        </div>{/if}
    </div>
    <footer class="changes-footer">
      <div class="save-state">
        <span class="save-state-icon" class:pending={dirty}
          ><Icon name={dirty ? "edit" : "check"} size={13} /></span
        >
        <div>
          <strong
            >{saving
              ? "Applying changes…"
              : saveError
                ? "Couldn’t confirm your changes"
                : dirty
                  ? "Unapplied changes"
                  : savedOnce
                    ? isPreview
                      ? "Saved in this preview"
                      : "Written to Finicky Plus"
                    : "Configuration loaded"}</strong
          ><span class:error-text={Boolean(saveError || validationError)}
            >{saveError ||
              validationError ||
              (dirty
                ? "Apply when your routing looks right."
                : isPreview
                  ? "Your Mac’s Finicky Plus configuration is unchanged."
                  : "Changes are saved only when you apply them.")}</span
          >
        </div>
      </div>
      <div class="footer-actions">
        {#if dirty}<button
            class="text-button"
            onclick={() => openModal("discard")}
            disabled={saving}>Discard</button
          >{/if}<button
          class="button primary"
          onclick={apply}
          disabled={!loaded ||
            !dirty ||
            saving ||
            externalChange ||
            Boolean(validationError)}
          >{#if saving}<span class="spinner"></span>{/if}Apply changes</button
        >
      </div>
    </footer>
  </main>
</div>

<dialog class="editor-dialog" bind:this={modalElement} oncancel={closeModal}>
  {#if modal === "rule"}<form onsubmit={saveRule}>
      <header class="dialog-header">
        <div>
          <div class="eyebrow">FINICKY PLUS / ROUTING</div>
          <h2>{editIndex < 0 ? "Add routing rule" : "Edit routing rule"}</h2>
        </div>
        <button
          type="button"
          class="icon-button"
          onclick={closeModal}
          aria-label="Close dialog"><Icon name="close" /></button
        >
      </header>
      <div class="dialog-body">
        <div class="field">
          <label for="rule-name">Rule name</label><input
            id="rule-name"
            bind:value={ruleForm.name}
            placeholder="Work apps"
            maxlength="100"
          />
        </div>
        <div class="field">
          <label for="match-type">Match links by</label><select
            id="match-type"
            bind:value={matchType}
            ><option value="app">Source app</option><option value="website"
              >Website pattern</option
            ><option value="both">Source app and website</option></select
          >
        </div>
        {#if matchType !== "website"}<fieldset class="app-choices">
            <legend>Opened from any of these apps</legend
            >{#each knownApps as source}<label class="app-choice"
                ><span
                  class="app-monogram"
                  style={`--app-color:${source.color}`}
                  >{source.name.slice(0, 1)}</span
                ><span>{source.name}</span><input
                  type="checkbox"
                  bind:group={selectedApps}
                  value={source.id}
                /></label
              >{/each}
          </fieldset>
          <details class="custom-apps" open={Boolean(customApps)}>
            <summary>Other apps</summary>
            <div class="field">
              <label for="custom-apps">App bundle IDs</label><textarea
                id="custom-apps"
                bind:value={customApps}
                placeholder="com.example.app"
                rows="2"></textarea>
              <p class="field-hint">One app bundle ID per line.</p>
            </div>
          </details>{/if}{#if matchType !== "app"}<div class="field">
            <label for="match-patterns">Website patterns</label><textarea
              id="match-patterns"
              bind:value={matchText}
              placeholder="*example.edu/*"
              rows="3"></textarea>
            <p class="field-hint">
              One Finicky Plus URL pattern per line. Use <code>*</code> as a
              wildcard. {matchType === "both"
                ? "The source app and a website pattern must both match."
                : "Any matching pattern triggers this rule."}
            </p>
          </div>{/if}
        <div class="field">
          <label for="rule-target">Open in</label><select
            id="rule-target"
            value={selectedTarget}
            onchange={(event) => selectRuleTarget(event.currentTarget.value)}
            >{#each profiles as p}<option value={p.id}
                >{p.name} · {p.browser}</option
              >{/each}<option value="picker">Choose with browser picker</option
            ><option value="custom">Custom browser or profile</option></select
          >
        </div>
        {#if selectedTarget === "custom"}<div class="field">
            <label for="rule-browser"
              >Browser or app name, bundle ID, or path</label
            ><input
              id="rule-browser"
              list="browsers"
              bind:value={ruleForm.browser}
              required
            />
          </div>
          <div class="field">
            <label for="rule-directory">Profile directory (optional)</label
            ><input
              id="rule-directory"
              bind:value={ruleForm.profileDirectory}
              placeholder="Default or Profile 2"
            />
            <p class="field-hint">
              Leave empty to use the browser’s last used profile.
            </p>
          </div>
          {#if ruleForm.profile}<div class="field">
              <label for="rule-legacy-profile">Existing profile name</label
              ><input id="rule-legacy-profile" bind:value={ruleForm.profile} />
              <p class="field-hint">
                The profile directory takes priority when provided.
              </p>
            </div>{/if}{/if}
        <details
          class="custom-apps rule-transform"
          open={Boolean(ruleForm.transformScript)}
        >
          <summary>Transform matching links</summary>
          <div class="field">
            <label for="rule-transform">Transformation script (optional)</label
            ><textarea
              id="rule-transform"
              class="script-editor"
              rows="5"
              spellcheck="false"
              autocapitalize="off"
              bind:value={ruleForm.transformScript}
              placeholder="return url.replace('old.example', 'new.example');"
            ></textarea>
            <p class="field-hint">
              Write a JavaScript function body using <code>url</code> (a
              string), <code>options.opener</code>, and
              <code>options.originalUrl</code>. Return a non-empty, absolute URL
              string.
            </p>
            <p class="field-hint">
              Runs for links matched by this rule, after any handler URL rewrite
              and before choosing the destination. Leave blank to disable.
            </p>
          </div>
        </details>
        {#if selectedTarget !== "picker"}<div class="launch-options">
            <label class="check-label"
              ><input type="checkbox" bind:checked={ruleForm.private} />Open in
              private browsing</label
            ><label class="check-label"
              ><input type="checkbox" bind:checked={ruleForm.newWindow} />Open
              in a new window</label
            >
            <p class="field-hint">
              {unsupportedPrivate(ruleForm.browser)
                ? "Safari private browsing is unavailable through this integration."
                : "Availability depends on the browser."}
            </p>
          </div>{/if}<label class="check-label"
          ><input
            type="checkbox"
            checked={ruleForm.enabled !== false}
            onchange={(event) =>
              (ruleForm.enabled = event.currentTarget.checked)}
          />Enable this rule</label
        >{#if modalError}<div class="inline-error" role="alert">
            {modalError}
          </div>{/if}
      </div>
      <footer class="dialog-footer">
        <button class="button secondary" type="button" onclick={closeModal}
          >Cancel</button
        ><button class="button primary" type="submit"
          >{editIndex < 0 ? "Add rule" : "Save rule"}</button
        >
      </footer>
    </form>
  {:else if modal === "profile"}<form onsubmit={saveProfile}>
      <header class="dialog-header">
        <div>
          <div class="eyebrow">FINICKY PLUS / PROFILES</div>
          <h2>{originalProfile ? "Edit profile" : "Add browser profile"}</h2>
        </div>
        <button
          type="button"
          class="icon-button"
          onclick={closeModal}
          aria-label="Close dialog"><Icon name="close" /></button
        >
      </header>
      <div class="dialog-body">
        <div
          class="profile-edit-heading"
          style={`--profile-color:${safeColor(profileForm.color)}`}
        >
          <span class="profile-avatar"
            >{profileForm.name.slice(0, 1) || "P"}</span
          >
          <p>
            A friendly label for a browser profile or installed app.<br />Your
            browser account name stays the same.
          </p>
        </div>
        <div class="field-row">
          <div class="field">
            <label for="profile-name">Profile name</label><input
              id="profile-name"
              bind:value={profileForm.name}
              placeholder="Personal, Work, Academic…"
              required
              maxlength="40"
            />
          </div>
          <div class="field color-field">
            <label for="profile-color">Color</label><input
              id="profile-color"
              type="color"
              bind:value={profileForm.color}
            />
          </div>
        </div>
        <div class="field">
          <label for="profile-browser">Browser or custom app</label><input
            id="profile-browser"
            list="browsers"
            bind:value={profileForm.browser}
            placeholder="Google Chrome, com.example.app, or /Applications/My App.app"
            onchange={(event) => discoverProfiles(event.currentTarget.value)}
            required
          />
          <p class="field-hint">
            Enter an installed browser, app name, bundle ID, or absolute app
            path. This also supports installed web apps.
          </p>
        </div>
        {#if discoveredProfiles.length}<div class="field">
            <label for="detected-profile">Profiles found on this Mac</label
            ><select
              id="detected-profile"
              value={profileForm.profileDirectory || ""}
              onchange={(event) =>
                chooseDiscoveredProfile(event.currentTarget.value)}
              ><option value="">Choose a detected profile…</option
              >{#each discoveredProfiles as detected}<option
                  value={detected.profileDirectory}
                  >{detected.name} · {detected.profileDirectory}</option
                >{/each}</select
            >
            <p class="field-hint">
              The directory identifies the profile, even when several profiles
              share a name.
            </p>
          </div>{/if}
        <div class="field">
          <label for="profile-directory">Profile directory (optional)</label
          ><input
            id="profile-directory"
            bind:value={profileForm.profileDirectory}
            placeholder="Default or Profile 2"
          />
          <p class="field-hint">
            In Chrome, open <code>chrome://version</code>. Enter only the last
            folder in <strong>Profile Path</strong>. This folder is used as
            entered. Leave empty for an app without profiles or the browser’s
            last used profile.
          </p>
        </div>
        <details class="custom-apps" open={Boolean(profileForm.profile)}>
          <summary>Use a browser profile name instead</summary>
          <div class="field">
            <label for="profile-exact-name">Exact profile name</label><input
              id="profile-exact-name"
              bind:value={profileForm.profile}
              placeholder="Name as shown in the browser"
            />
            <p class="field-hint">
              A directory, when provided, takes priority over this name.
            </p>
          </div>
        </details>
        <div class="field">
          <label for="profile-shortcut"
            >Picker keyboard shortcut (optional)</label
          ><input
            id="profile-shortcut"
            bind:value={profileForm.shortcut}
            maxlength="1"
            placeholder="W"
            class="shortcut-input"
          />
          <p class="field-hint">
            One letter (A–Z). Press it while the picker is open. Number keys are
            reserved for picker positions.
          </p>
        </div>
        <div class="launch-options">
          <label class="check-label"
            ><input type="checkbox" bind:checked={profileForm.hidden} />Hide
            this profile from the picker</label
          ><label class="check-label"
            ><input type="checkbox" bind:checked={profileForm.private} />Use
            private browsing when chosen in the picker</label
          >
          <p class="field-hint">
            {unsupportedPrivate(profileForm.browser)
              ? "Safari private browsing is unavailable through this integration."
              : "Private browsing depends on browser support."} Existing routing rules
            have their own launch options.
          </p>
        </div>
        {#if originalProfile}<label class="check-label"
            ><input type="checkbox" bind:checked={updateReferences} />Update
            rules using this profile if its destination changes</label
          >{/if}{#if modalError}<div class="inline-error" role="alert">
            {modalError}
          </div>{/if}
        <div class="inline-note">
          <Icon name="info" size={14} />
          <p>
            Finicky Plus saves a shortcut to this profile. Create or delete
            actual browser profiles in your browser.
          </p>
        </div>
      </div>
      <footer class="dialog-footer">
        {#if originalProfile}<button
            class="text-button danger-text"
            type="button"
            onclick={() => (modal = "deleteProfile")}>Remove shortcut</button
          >{/if}<span class="spacer"></span><button
          class="button secondary"
          type="button"
          onclick={closeModal}>Cancel</button
        ><button class="button primary" type="submit">Save profile</button>
      </footer>
    </form>
  {:else if modal}<header class="dialog-header">
      <h2>
        {modal === "deleteRule"
          ? "Delete routing rule?"
          : modal === "deleteProfile"
            ? "Remove profile shortcut?"
            : externalChange
              ? "Reload configuration?"
              : "Discard unapplied changes?"}
      </h2>
      <button
        type="button"
        class="icon-button"
        onclick={closeModal}
        aria-label="Close dialog"><Icon name="close" /></button
      >
    </header>
    <div class="dialog-body">
      <p class="confirmation-copy">
        {modal === "deleteRule"
          ? `“${ruleName(draft.rules[editIndex])}” will be removed from your draft. Apply changes to update Finicky Plus.`
          : modal === "deleteProfile"
            ? "This removes the saved label from Finicky Plus. Existing routing rules and the browser profile will stay in place."
            : externalChange
              ? "Load the latest configuration from disk? Your unapplied edits in this window will be discarded."
              : "Return to your last applied configuration? Your unapplied changes will be discarded."}
      </p>
    </div>
    <footer class="dialog-footer">
      <button class="button secondary" onclick={closeModal}>Cancel</button
      ><button
        class="button"
        class:danger={modal.startsWith("delete")}
        class:primary={!modal.startsWith("delete")}
        onclick={() => {
          if (modal === "deleteRule") {
            mutate((model) => model.rules.splice(editIndex, 1));
            closeModal();
          } else if (modal === "deleteProfile") {
            mutate(
              (model) =>
                (model.profiles = model.profiles?.filter(
                  (p) => p.id !== originalProfile?.id,
                )),
            );
            closeModal();
          } else discard();
        }}
        >{modal === "deleteRule"
          ? "Delete rule"
          : modal === "deleteProfile"
            ? "Remove shortcut"
            : externalChange
              ? "Reload"
              : "Discard changes"}</button
      >
    </footer>{/if}
</dialog>
<datalist id="browsers"
  >{#each browserChoices as browser}<option value={browser}
    ></option>{/each}</datalist
>
<ToastContainer />
