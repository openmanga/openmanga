# Storyboarder Next — desktop app

Tauri 2 + Svelte 5 + TypeScript. The `sb` CLI (`../cli`) is the engine, shipped as a sidecar.

```sh
npm install
npm run sb                      # build ../cli into src-tauri/binaries/sb-aarch64-apple-darwin
npx tauri dev --release         # run (always release)
npx tauri build                 # .app + .dmg in src-tauri/target/release/bundle/
```

- `src-tauri/src/lib.rs`: `sb` command (runs `sb … --json`), project folder watcher (`project-changed` event).
- `src/lib/sb.svelte.ts`: app state built from `sb` JSON; every edit is an `sb` call followed by a refresh.
- `src/views/`: Welcome, Editor (Sidebar, Toolbar, Inspector, PageView, BoardView, SpreadView, GridView), sheets, Settings window.
- `SB_USER_DATA=<dir>` isolates prefs/recents (passed through to `sb`).
- Dev-only UI verification hook: with `SB_UI_SCRIPT=<file.js>` set, the app runs new contents of that file
  (`window.__sb` exposes the state and actions); unset, it does nothing.
