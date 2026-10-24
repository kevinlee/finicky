<script lang="ts">
  import Icon from "./StudioIcon.svelte";
  import { matchesFavorite } from "../lib/settingsModel";
  import {
    effectiveFeatures,
    parseShorteners,
    shortenerError,
    nativeServiceChoices,
    selectedNativeServices,
    toggleNativeService,
  } from "../lib/featuresModel";
  import type { Features, SavedProfile } from "../types";
  export let features: Partial<Features> | undefined;
  export let profiles: SavedProfile[] = [];
  export let browsers: string[] = [];
  export let disabled = false;
  export let onChange: (features: Features) => void;
  export let onPreview: () => void;
  export let onClipboard: () => void;
  let shortenersText = "";
  let lastShorteners: string[] | undefined;
  $: current = effectiveFeatures(features);
  $: if (current.extraShorteners !== lastShorteners) {
    lastShorteners = current.extraShorteners;
    shortenersText = current.extraShorteners.join("\n");
  }
  function editShorteners(value: string) {
    shortenersText = value;
    const parsed = parseShorteners(value);
    lastShorteners = parsed;
    update({ extraShorteners: parsed });
  }
  $: alternate = profiles.find((profile) =>
    matchesFavorite(profile, {
      browser: current.alternativeBrowser || "",
      profile: current.alternativeProfile,
      profileDirectory: current.alternativeProfileDirectory,
      private: current.alternativePrivate,
    }),
  );
  $: alternateValue = alternate
    ? `profile:${alternate.id}`
    : current.alternativeBrowser
      ? "custom"
      : "none";
  const toggles: {
    key:
      | "stripTracking"
      | "unwrapRedirects"
      | "expandShortURLs"
      | "nativeApps"
      | "historyEnabled";
    name: string;
    hint: string;
  }[] = [
    {
      key: "stripTracking",
      name: "Remove tracking parameters",
      hint: "Clean known marketing and click-tracking parameters before opening a link.",
    },
    {
      key: "unwrapRedirects",
      name: "Unwrap redirect links",
      hint: "Extract the destination from supported tracking and security wrappers.",
    },
    {
      key: "expandShortURLs",
      name: "Expand shortened links",
      hint: "Resolve supported short links before choosing a browser. This makes a network request to the shortener.",
    },
    {
      key: "nativeApps",
      name: "Open supported links in native apps",
      hint: "Hand supported meeting, music, and other links to their installed Mac apps.",
    },
    {
      key: "historyEnabled",
      name: "Keep recent link history",
      hint: "Save recent routed URLs on this Mac. You can review and clear them in History.",
    },
  ];
  function update(patch: Partial<Features>) {
    onChange({ ...current, ...patch });
  }
  function chooseAlternate(value: string) {
    if (value === "custom") {
      update({ alternativeBrowser: current.alternativeBrowser || "Safari" });
      return;
    }
    const profile = profiles.find((entry) => `profile:${entry.id}` === value);
    update({
      alternativeBrowser: profile?.browser || "",
      alternativePrivate: Boolean(profile?.private),
      alternativeProfile: profile?.profile || "",
      alternativeProfileDirectory: profile?.profileDirectory || "",
    });
  }
</script>

<section class="feature-section" aria-labelledby="appearance-heading">
  <div class="section-heading">
    <div>
      <h2 id="appearance-heading">Menu bar appearance</h2>
      <p>A familiar marker for your link destinations.</p>
    </div>
  </div>
  <div class="preferences-panel">
    <div class="preference-row preference-control">
      <div>
        <h3>Menu bar icon</h3>
        <p>Choose the symbol Finicky Plus uses in your Mac’s menu bar.</p>
      </div>
      <select
        aria-label="Menu bar icon"
        value={current.menuIcon}
        {disabled}
        onchange={(event) =>
          update({
            menuIcon: event.currentTarget.value as Features["menuIcon"],
          })}
        ><option value="default">Finicky Plus icon</option><option
          value="browser">Default browser icon</option
        ><option value="globe">Globe</option><option value="arrow">Arrow</option
        ></select
      >
    </div>
  </div>
</section>

<section class="feature-section" aria-labelledby="picker-heading">
  <div class="section-heading">
    <div>
      <h2 id="picker-heading">Browser picker</h2>
      <p>Choose the right destination at the moment you open a link.</p>
    </div>
    <button class="button small secondary" onclick={onPreview}
      ><Icon name="profiles" size={14} />Preview picker</button
    >
  </div>
  <div class="preferences-panel">
    <div class="preference-row preference-control">
      <div>
        <h3>When to ask</h3>
        <p>Choose automatically, or pause to select a browser or profile.</p>
      </div>
      <select
        aria-label="When to show the browser picker"
        value={current.promptMode}
        {disabled}
        onchange={(event) =>
          update({
            promptMode: event.currentTarget.value as Features["promptMode"],
          })}
        ><option value="never">Use my routing rules</option><option
          value="unmatched">When no rule matches</option
        ><option value="always">For every link</option></select
      >
    </div>
    <div class="preference-row preference-control">
      <div>
        <h3>Hold a key to choose</h3>
        <p>Hold this key while opening an external link to show the picker.</p>
      </div>
      <select
        aria-label="Browser picker modifier key"
        value={current.promptModifier}
        {disabled}
        onchange={(event) =>
          update({
            promptModifier: event.currentTarget
              .value as Features["promptModifier"],
          })}
        ><option value="fn">Fn / Globe</option><option value="option"
          >Option ⌥</option
        ><option value="shift">Shift ⇧</option><option value="control"
          >Control ⌃</option
        ><option value="command">Command ⌘</option><option value="none"
          >Disabled</option
        ></select
      >
    </div>
    <div class="preference-row">
      <div>
        <h3>Open from clipboard</h3>
        <p>Send a copied web link through Finicky Plus.</p>
      </div>
      <button class="button small secondary" onclick={onClipboard}
        >Open clipboard link</button
      >
    </div>
  </div>
  <p class="field-hint feature-hint">
    Picker preview uses your saved settings. Profile shortcuts and hidden
    profiles are managed in Profiles.
  </p>
</section>

<section class="feature-section" aria-labelledby="alternate-heading">
  <div class="section-heading">
    <div>
      <h2 id="alternate-heading">Alternative destination</h2>
      <p>Keep another browser or profile within easy reach.</p>
    </div>
  </div>
  <div class="panel feature-fields">
    <div class="field">
      <label for="alternative-target">Alternative browser or profile</label
      ><select
        id="alternative-target"
        value={alternateValue}
        {disabled}
        onchange={(event) => chooseAlternate(event.currentTarget.value)}
        ><option value="none">No alternative destination</option
        >{#each profiles as profile}<option value={`profile:${profile.id}`}
            >{profile.name} · {profile.browser}</option
          >{/each}<option value="custom">Custom browser or profile</option
        ></select
      >
    </div>
    {#if alternateValue === "custom"}<div class="field-row">
        <div class="field">
          <label for="alternative-browser">Browser</label><input
            id="alternative-browser"
            list="alternative-browsers"
            value={current.alternativeBrowser || ""}
            {disabled}
            onchange={(event) =>
              update({ alternativeBrowser: event.currentTarget.value.trim() })}
          />
        </div>
        <div class="field">
          <label for="alternative-profile">Profile directory (optional)</label
          ><input
            id="alternative-profile"
            placeholder="Default or Profile 2"
            value={current.alternativeProfileDirectory || ""}
            {disabled}
            onchange={(event) =>
              update({
                alternativeProfileDirectory: event.currentTarget.value.trim(),
                alternativeProfile: "",
              })}
          />
        </div>
      </div>{/if}
    {#if alternateValue !== "none"}<label class="check-label alternate-private"
        ><input
          type="checkbox"
          checked={current.alternativePrivate || false}
          {disabled}
          onchange={(event) =>
            update({ alternativePrivate: event.currentTarget.checked })}
        />Use private browsing for the alternative destination</label
      >{#if current.alternativeBrowser?.toLowerCase().includes("safari")}<p
          class="field-hint"
        >
          Safari private browsing is unavailable through this integration.
        </p>{/if}{/if}
    <p class="field-hint">
      Hold your picker modifier key while opening a link to use this
      destination. Choosing a saved profile copies its destination here.
    </p>
  </div>
</section>

<section class="feature-section" aria-labelledby="handling-heading">
  <div class="section-heading">
    <div>
      <h2 id="handling-heading">Link handling</h2>
      <p>A little cleanup before a link reaches its destination.</p>
    </div>
  </div>
  <div class="preferences-panel">
    {#each toggles as option}<div class="preference-row">
        <div>
          <h3>{option.name}</h3>
          <p>{option.hint}</p>
        </div>
        <button
          class="switch"
          class:on={current[option.key]}
          role="switch"
          aria-checked={current[option.key]}
          aria-label={option.name}
          {disabled}
          onclick={() => update({ [option.key]: !current[option.key] })}
          ><span></span></button
        >
      </div>{/each}
  </div>
  <div class="panel feature-fields service-preferences">
    <div class="section-heading">
      <div>
        <h3>Native app services</h3>
        <p>Choose which supported links may open in an installed app.</p>
      </div>
      <div class="inline-actions">
        <button
          type="button"
          class="text-button"
          disabled={disabled || !current.nativeApps}
          onclick={() => update({ nativeAppServices: undefined })}>All</button
        ><button
          type="button"
          class="text-button"
          disabled={disabled || !current.nativeApps}
          onclick={() => update({ nativeAppServices: [] })}>None</button
        >
      </div>
    </div>
    <fieldset
      class="service-choices"
      disabled={disabled || !current.nativeApps}
    >
      <legend class="sr-only">Enabled native app services</legend
      >{#each nativeServiceChoices as service}<label class="check-label"
          ><input
            type="checkbox"
            checked={selectedNativeServices(current.nativeAppServices).includes(
              service.id,
            )}
            onchange={(event) =>
              update({
                nativeAppServices: toggleNativeService(
                  current.nativeAppServices,
                  service.id,
                  event.currentTarget.checked,
                ),
              })}
          /><span class="service-option-text"
            >{service.name}{#if service.hint}<small>{service.hint}</small
              >{/if}</span
          ></label
        >{/each}
    </fieldset>
    <p class="field-hint">
      {current.nativeApps
        ? "Only selected services with a compatible installed app are handed off."
        : "Turn on native app links above to use these service choices."}
    </p>
  </div>
  <div class="panel feature-fields extra-shorteners">
    <div class="field">
      <label for="extra-shorteners">Additional shortener domains</label
      ><textarea
        id="extra-shorteners"
        rows="3"
        placeholder="go.example.com"
        value={shortenersText}
        {disabled}
        aria-describedby="shortener-hint"
        aria-invalid={Boolean(shortenerError(current.extraShorteners))}
        oninput={(event) => editShorteners(event.currentTarget.value)}
      ></textarea>
      <p id="shortener-hint" class="field-hint">
        One hostname per line. These are resolved only when short-link expansion
        is enabled.
      </p>
      {#if shortenerError(current.extraShorteners)}<p
          class="error-text"
          role="alert"
        >
          {shortenerError(current.extraShorteners)}
        </p>{/if}
    </div>
  </div>
</section>
<section class="feature-section" aria-labelledby="transform-heading">
  <div class="section-heading">
    <div>
      <h2 id="transform-heading">URL transformation</h2>
      <p>Customize a link before its routing rules are checked.</p>
    </div>
  </div>
  <div class="panel feature-fields">
    <div class="field">
      <label for="global-transform"
        >Global transformation script (optional)</label
      ><textarea
        id="global-transform"
        class="script-editor"
        rows="6"
        spellcheck="false"
        autocapitalize="off"
        value={current.transformScript || ""}
        {disabled}
        placeholder="return url.replace('old.example', 'new.example');"
        oninput={(event) =>
          update({ transformScript: event.currentTarget.value })}></textarea>
      <p class="field-hint">
        Write a JavaScript function body using <code>url</code> (a string),
        <code>options.opener</code>, and <code>options.originalUrl</code>.
        Return a non-empty, absolute URL string. Leave blank to disable.
      </p>
      <p class="field-hint">
        Runs after link cleanup, before JavaScript rewrites and routing rules.
        Use Test a link after applying your changes.
      </p>
    </div>
  </div>
</section>
<datalist id="alternative-browsers"
  >{#each browsers as browser}<option value={browser}></option>{/each}</datalist
>
