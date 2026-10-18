# WorldKit — Project Overview

Context for agents and new contributors: what exists, why it is built this way, and where things
are. **Read this first, then `CLAUDE.md` (code rules) and `DESIGN.md` (visual rules).**
Keep this file current: update it in the same change whenever features, structure or key
decisions change.

_Last updated: 2026-10-18_

## What it is

WorldKit is a small, **Windows-only** (amd64) desktop application written in Go with the Gio UI
toolkit. Its visual identity is a dark UI with a single orange accent and the "WK WorldKit"
logo (sunset city skyline). It is a companion tool for players of **GTA World** (GTA:W), a
roleplay server on FiveM. Its first feature is **chat log capture**: saving the in-game chat,
which the game itself throws away on every reconnect, to plain text files on disk.

## Current state

| Area             | Status                                                                 |
|------------------|------------------------------------------------------------------------|
| App shell        | Done: window, dark background, sidebar + page area                     |
| Sidebar          | Done: collapsible (200 ↔ 58 dp), 200 ms ease-in-out animation, hover/active states |
| Navigation       | Done: Home, Explore, Map, Library, Settings (switch the page title)     |
| Home page        | Done: centered logo hero + welcome text + capture status line          |
| Settings page    | Done: capture status, open logs folder, FiveM data folder, start with Windows |
| Other pages      | Placeholders ("This page is empty for now.")                           |
| Branding         | Done: logo in sidebar header, Home hero, exe/title bar/taskbar icon    |
| Chat capture     | Done: background polling of FiveM's KVS, one `.txt` per session, crash-safe appends |
| Chat parsing     | Done (no UI yet): `chatlog.Parse` → records with time, kind, speaker, text |
| First-run prompts| Done: FiveM data folder (only if not found), start with Windows (asked once) |
| Persistence      | `%AppData%\WorldKit\settings.json` and `%AppData%\WorldKit\logs\YYYY\MM\*.txt` |
| Tests            | Animation, chat archive/parser, KVS reader, capture service, settings, status text |

## Architecture

```
main()  (-minimized flag)               cmd/worldkit/main.go
 └─ run()                               cmd/worldkit/run.go  wiring: settings, capture, autostart, window
     ├─ capture.Service.Run(ctx)        internal/capture     goroutine, polls every 3 s
     │   ├─ fivem.ReadStamp / ReadChatLog   internal/fivem   copy KVS → temp, goleveldb read-only
     │   └─ chatlog.Archive.Save        internal/chatlog     append new lines to the session file
     └─ ui.Run(window, Services)        internal/ui/app.go   event loop: one Shell.Layout per frame
         └─ Shell                       internal/ui/app.go   sidebar + selected page + prompt
             ├─ Sidebar                 internal/ui/sidebar.go
             │   ├─ header: toggle button + brand (Logo + "WorldKit"), fades when collapsing
             │   ├─ NavItem × N  ──► navRow (shared icon + label row renderer)
             │   └─ expandAnimation     internal/ui/animation.go
             ├─ page: layoutHomePage | SettingsPage | layoutPlaceholderPage
             └─ prompt: dataDirPrompt (capture needs a folder) | startupPrompt (not answered yet)
Logo                                    internal/ui/logo.go  resamples per pixel size, cached
theme tokens (colors, sizes, timing)    internal/ui/theme.go mirrors DESIGN.md
assets.Logo()                           assets/assets.go     embedded logo.png, decoded once
```

The UI talks to domain code only through the small interfaces in `ui.Services`
(`CaptureService`, `SettingsStore`, `StartupEntry`); capture calls `window.Invalidate` when its
status changes.

### Chat capture data flow

1. GTA World's FiveM resource writes the whole current session's chat to the KVS key
   `res:client:gtaw_chat_log` (MessagePack string, `\n`-separated lines) on every new line, in
   `%AppData%\CitizenFX\kvs` (LevelDB). A reconnect starts a new value with a
   `[DATE: 18/OCT/2026 | TIME: 09:57:01]` header (UTC). `res:client:gtaw_server_version` = "GTA World"
   confirms the data is GTA World's (the store is shared by all FiveM servers).
2. Every 3 s, `capture` compares the folder's file stamp. On change it copies the store to a temp
   folder and reads the key from the copy.
3. `chatlog.Archive.Save` appends only the part not yet saved, then `Sync`s. A new header means a
   new file; dropped oldest lines are handled by overlap matching.
4. Files: `%AppData%\WorldKit\logs\2026\10\2026-10-18_095701.txt`, exactly as the game wrote them.
   `chatlog.Parse` turns a file into records when needed.

Gio is immediate-mode: every frame, `Layout` methods handle input first, then draw. Widget state
(`widget.Clickable`, animation state, selection) lives in structs that persist across frames.

## File map

| Path                                   | Purpose                                                     |
|----------------------------------------|-------------------------------------------------------------|
| `cmd/worldkit/main.go`                 | Parses `-minimized`, starts `run` and the Gio main loop     |
| `cmd/worldkit/run.go`                  | Builds settings, capture service, startup entry, window (960×600, min 640×400) |
| `internal/fivem/`                      | Finds FiveM's KVS, copies it, reads the GTA World chat log; MessagePack string decoder |
| `internal/chatlog/`                    | `Archive` (session files, lock, crash repair) and `Parse` (line kinds, midnight rollover) |
| `internal/capture/`                    | Background `Service`: polling loop, data folder changes, `Status` |
| `internal/settings/`                   | `settings.json` load + atomic save                          |
| `internal/autostart/`                  | Start with Windows via the HKCU `Run` registry key          |
| `internal/ui/settings_page.go`         | Settings page                                               |
| `internal/ui/capture_ui.go`            | `ui.Services`, status text, data folder form, first-run prompts, text field |
| `internal/ui/button.go`, `dialog.go`   | Primary/secondary buttons; modal dialog and `card`          |
| `cmd/worldkit/rsrc_windows_amd64.syso` | Generated exe icon + manifest; linked automatically by `go build` |
| `internal/ui/app.go`                   | Event loop, `Shell`, pages, `pageTitle`/`caption` helpers   |
| `internal/ui/sidebar.go`               | `Sidebar`, `NavItem`, `navRow`, divider/border helpers      |
| `internal/ui/animation.go`             | `expandAnimation` (reversible 0↔1 progress) + easing        |
| `internal/ui/logo.go`                  | `Logo` widget (sharp at any DPI)                            |
| `internal/ui/theme.go`                 | All design tokens and `NewTheme()`                          |
| `assets/assets.go`, `assets/logo.png`  | Embedded logo (384×384, transparent); the only logo source  |
| `CLAUDE.md`                            | Code rules, commands, commit checklist                      |
| `DESIGN.md`                            | Palette, typography, spacing, sidebar/logo/button specs     |
| `.graphifyignore`, `graphify-out/`     | Knowledge-graph config and output (see below)               |

## Key decisions (and why)

- **Gio over Fyne.** The dev machine has no C compiler and CGO is off. Fyne needs CGO + MinGW;
  Gio is pure Go on Windows, lightweight, and ships Material-style widgets.
- **Windows only.** Product decision. No cross-platform code paths or resources.
- **Logo resampled at runtime** (`golang.org/x/image/draw`, Catmull-Rom). Gio only scales
  images linearly with no mipmaps, which looks jagged when shrinking. The PNG is 384 px:
  enough for the 160 dp hero up to ~240% display scaling while keeping the exe small.
- **One-off tools via `go run module@version`**, not `go.mod` `tool` entries, to keep the module
  graph minimal (e.g. `go-winres` for the exe icon; the command is in `CLAUDE.md`).
- **Chat comes from FiveM's KVS, read passively.** Community tools instead attach to FiveM's CEF
  DevTools port (13172) and run scripts inside the server's page; cheat tools use that port too,
  so WorldKit avoids it. GTA World moved from RAGE:MP to FiveM in 2026 (RAGE:MP shut down); the
  old RAGE:MP `.storage` format is not supported (no import planned).
- **Raw text files are the source of truth**, not JSON or SQLite: lossless, greppable, no schema
  migrations; better parse rules re-parse history. A disposable SQLite FTS index can be added if
  history ever grows past hundreds of MB.
- **Only FiveM's data folder is needed**, not the install folder. WorldKit detects
  `%AppData%\CitizenFX\kvs` and asks for the path only if it isn't there (accepts the CitizenFX
  folder or `kvs` itself; `-cl2` clients use `kvs_cl2`).
- **Capture runs only while WorldKit runs.** A one-time prompt offers starting with Windows
  (starts minimized via `-minimized`). Closing the window stops capture.
- **Timestamps are UTC**, as the game writes them; shown as-is.
- **Single instance per archive:** `archive.lock` is held open with no sharing; a second WorldKit
  shows "already running" instead of writing.
- **Collapsed sidebar width is derived**, not hard-coded: `2 × (8 + 12) + 18 = 58 dp`, so icons
  sit at the same x position in both states and never jump during the animation.

## Known gotchas

- **Gio constraints:** `layout.W`/`E` keep `Min.Y`. Clear it before laying out text or labels
  stretch and render off-center (this bug happened once; see `navRow.layoutIconAndLabel`).
- **Animation frames:** request redraws with `gtx.Execute(op.InvalidateCmd{})` only while the
  animation is running, or the app redraws forever at idle.
- **Visual checks without a screen:** the app window may open behind other windows, so
  synthetic clicks/screenshots of the real window are unreliable. Render off-screen instead with
  `gioui.org/gpu/headless` in a throwaway `_test.go` and drive input through `input.Router`.
- **Claude desktop sandbox:** tools run inside a packaged app, so writes under `%APPDATA%` are
  redirected to a private folder. Install user-level tools elsewhere (graphify lives under
  `%USERPROFILE%\.local`).

- **Chat log quirks:** the KVS value is rewritten whole on every line and replaced on reconnect,
  so anything not captured before the next connect is gone. Line kinds are recognised only by
  text patterns (`says:`, `* `, `(( … ))`, `[INFO]`); unknown lines stay `KindUnknown`.
- **Not yet verified:** copying the KVS while FiveM is running (a sharing violation is logged and
  retried on the next poll), and whether the game caps the chat buffer size.
- **`-race` needs CGO**, which the dev machine lacks, so race tests can't run locally.
- **Start with Windows registers the running exe.** Enabling it from `go run` registers a temp
  build; use the release build.

## Knowledge graph (graphify)

[graphify](https://github.com/Graphify-Labs/graphify) turns the codebase into a queryable graph
so agents can find structure before reading files. Code is parsed **locally** (tree-sitter, Go
supported); nothing is sent to an LLM in code-only mode.

- Installed per user: `graphify.exe` in `%USERPROFILE%\.local\bin` (via `uv`, Python 3.12).
- `graphify-out/graph.json` and `GRAPH_REPORT.md` are committed; cache, HTML view and the
  machine-specific root file are git-ignored.
- Refresh after code changes (no API cost):

```bash
graphify update .
```

- Useful queries:

```bash
graphify query "how does the sidebar animate"
```

```bash
graphify explain "Sidebar"
```

## Next steps (not started)

- Verify capture while FiveM is running (live copy of the KVS) with a real play session.
- Chat log browser: list sessions by date, search text, filter by kind/speaker (uses `chatlog.Parse`).
- Tray icon so WorldKit can keep capturing with its window closed.
- Folder picker dialog for the FiveM data folder (today the path is typed/pasted).
- Give the remaining nav destinations real content.
- Persist sidebar expanded/collapsed state and last page between launches.
