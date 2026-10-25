# Waystile interface and development

Waystile extends Finicky's native routing engine with a profile-oriented Svelte settings window. It keeps the existing routing bridge and configuration contracts while adding visual controls for everyday use.

The interface centers on named browser destinations, ordered rules, and testing links with a source application. Visual rules remain in the existing `rules.json`; JavaScript configurations keep their priority. The UI does not attempt to execute, import, or overwrite custom JavaScript. See the [shared UI foundation](docs/UI.md) for typography, color, controls, and interaction conventions.

## Develop the interface

```sh
cd packages/finicky-ui
npm ci
npm run dev
```

Open `http://127.0.0.1:5178`. The development-only browser bridge seeds illustrative Personal, Work, and Academic profiles. Changes in this preview stay in browser storage. Profile directories in the preview are sample data, not discovered accounts. The same Svelte components run inside the native app, where the real bridge handles settings and URL evaluation.

## Build the native app locally

Requires macOS, Xcode command-line tools, Go 1.24 or later, and Node.js 22 or later.

```sh
(cd packages/config-api && npm ci)
(cd packages/finicky-ui && npm ci)
bash scripts/build-preview.sh
```

This produces an internal `apps/finicky/build/Finicky-arm64.app` bundle on Apple silicon, or `Finicky-amd64.app` on Intel. The internal build names remain for compatibility. Neither `scripts/build.sh` nor the preview wrapper installs an application or changes the default browser. Use `scripts/build-fork.sh` to produce the public `Waystile.app` bundle.

To test the native window alongside the installed router:

```sh
bash scripts/run-preview.sh
```

The launcher makes a separate **Waystile Preview** bundle and uses an isolated rules file under the ignored build directory. It leaves the default browser and installed router alone and simulates browser launches. Pass an explicit fixture file as the first argument to exercise another configuration. Opening the regular app directly starts normal routing behavior.

## Profile directories

A profile's friendly label can differ from its Chrome name. Explicit `profileDirectory` values select an existing Chromium profile folder directly and avoid reading Chrome's protected `Local State` file. The folder is visible at `chrome://version` under **Profile Path**. Only one folder component is accepted; paths and traversal are rejected. Creating, deleting, signing into, and renaming Chrome accounts remain Chrome operations.

## Verification

```sh
(cd packages/finicky-ui && npm run check && npm run build)
(cd packages/finicky-ui && node --experimental-strip-types --test tests/*.test.mjs)
(cd packages/config-api && npm run check-types && npx vitest run)
(cd apps/finicky/src && go test ./...)
```

The native app's tester evaluates the saved configuration, including custom JavaScript and source-app context. A draft rule must be applied before it can affect that result. Website patterns retain Finicky's existing wildcard semantics.
