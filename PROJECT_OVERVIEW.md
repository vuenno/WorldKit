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
which the game itself throws away on every reconnect, to plain text files on disk, and
browsing them in a **log viewer** with name and kind filters. Selected lines can be
**extracted** into transparent PNG "chat screenshots" (in-game look) for laying over game
screenshots, kept on the **Log Screenshots** page.

## Current state

| Area             | Status                                                                 |
|------------------|------------------------------------------------------------------------|
| App shell        | Done: window, dark background, sidebar + page area                     |
| Sidebar          | Done: collapsible (200 ↔ 58 dp), 200 ms ease-in-out animation, hover/active states |
| Navigation       | Done: Logs, Log Screenshots, Settings (Home/Explore/Map/Library were removed for now) |
| Logs page        | Done: saved sessions newest first (start time, last updated, session #) + capture status; reloads when shown and after new lines are saved |
| Log viewer       | Done: lines in GTA:W's in-game chat colors (12 sp, scrollbar), Filters panel: actions (/me, /do, /b, IC), auto-detected characters, show/hide timestamps; drag/Shift+click/Ctrl+A to select lines; Copy or Ctrl+C copies the selection (or all visible lines); a horizontal popup menu over the selection offers **Extract** (save as a screenshot image) |
| Log Screenshots  | Done: saved screenshots newest first (1:1 preview, save date, size); open one in full; Show in folder; Delete (with confirmation) |
| Settings page    | Done: capture status, open logs folder, FiveM data folder, start with Windows |
| Branding         | Done: logo in sidebar header, exe/title bar/taskbar icon               |
| Chat capture     | Done: background polling of FiveM's KVS, one `.txt` per session, crash-safe appends |
| Chat parsing     | Done: `chatlog.Parse` → records with time, kind, speaker, text; `chatlog.Filter` |
| First-run prompts| Done: FiveM data folder (only if not found), start with Windows (asked once) |
| Persistence      | `%AppData%\WorldKit\settings.json`, `%AppData%\WorldKit\logs\YYYY\MM\*.txt`, `%AppData%\WorldKit\screenshots\*.png` |
| Tests            | Animation, chat archive/parser/filter/library, KVS reader, capture service, settings, status and session texts, background load, screenshot render/store, selection menu placement |

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
             │   ├─ NavItem × 3 (Logs, Log Screenshots, Settings) ──► navRow (shared icon + label row renderer)
             │   └─ expandAnimation     internal/ui/animation.go
             ├─ page: LogsPage | ScreenshotsPage | SettingsPage
             │   ├─ LogsPage            internal/ui/logs_page.go  sessionRow list ──► logViewer
             │   │   ├─ chatlog.Library.Sessions / Records    read off the UI goroutine (backgroundLoad)
             │   │   └─ logViewer       internal/ui/log_viewer.go chatlog.Filter → chatLineList (colored lines, line selection)
             │   │       └─ selectionMenu → Extract → screenshot.Store.Save (background) → PNG
             │   └─ ScreenshotsPage     internal/ui/screenshots_page.go shotCard list ──► screenshotView (full image, delete)
             │       └─ screenshot.Store.List / Load / Delete            off the UI goroutine (backgroundLoad)
             └─ prompt: dataDirPrompt (capture needs a folder) | startupPrompt (not answered yet)
Logo                                    internal/ui/logo.go  resamples per pixel size, cached
theme tokens (colors, sizes, timing)    internal/ui/theme.go mirrors DESIGN.md
assets.Logo()                           assets/assets.go     embedded logo.png, decoded once
```

The UI talks to domain code only through the small interfaces in `ui.Services`
(`CaptureService`, `SettingsStore`, `LogLibrary`, `ScreenshotStore`, `StartupEntry`); capture calls
`window.Invalidate` when its status changes, and so does `backgroundLoad` when a disk read
finishes.

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
5. The Logs page lists files via `chatlog.Library` (read-only; start time from the file name,
   "last updated" from its modification time, session # by age). The viewer parses one file and
   shows the lines that pass `chatlog.Filter`, still exactly as stored.

Gio is immediate-mode: every frame, `Layout` methods handle input first, then draw. Widget state
(`widget.Clickable`, animation state, selection) lives in structs that persist across frames.

## File map

| Path                                   | Purpose                                                     |
|----------------------------------------|-------------------------------------------------------------|
| `cmd/worldkit/main.go`                 | Parses `-minimized`, starts `run` and the Gio main loop     |
| `cmd/worldkit/run.go`                  | Builds settings, capture service, startup entry, window (960×600, min 640×400) |
| `internal/fivem/`                      | Finds FiveM's KVS, copies it, reads the GTA World chat log; MessagePack string decoder |
| `internal/chatlog/`                    | `Archive` (session files, lock, crash repair), `Parse` (line kinds, midnight rollover), `Library` (list/read sessions), `Filter` (names + kinds), `Characters` (speakers by line count) |
| `internal/capture/`                    | Background `Service`: polling loop, data folder changes, `Status` |
| `internal/settings/`                   | `settings.json` load + atomic save                          |
| `internal/autostart/`                  | Start with Windows via the HKCU `Run` registry key          |
| `internal/ui/logs_page.go`             | Logs page: session list, `sessionRow`, session title/date texts |
| `internal/ui/log_viewer.go`            | `logViewer`: header (Back, Filters, Copy), filter panel, line list |
| `internal/ui/log_filters.go`           | `logFilters`: action + character options → `chatlog.Filter` |
| `internal/ui/wrap.go`                  | `layoutWrapped`: left-to-right layout that wraps to new rows |
| `internal/ui/chat_line.go`             | `chatLine` (one colored line), `chatLineColor` (kind → in-game color) |
| `internal/ui/chat_line_list.go`        | `chatLineList`: scrolling lines, mouse/keyboard line selection, copy text, places the selection menu |
| `internal/ui/selection_menu.go`        | `selectionMenu` (horizontal popup over a selection: Extract), `menuItem`, `selectionMenuOrigin` |
| `internal/screenshot/`                 | `Render` (lines → transparent PNG, in-game look), `Store` (save/list/load/delete in `screenshots\`) |
| `internal/ui/screenshots_page.go`      | Log Screenshots page: `shotCard` list with 1:1 previews, `loadShots` |
| `internal/ui/screenshot_view.go`       | `screenshotView`: one screenshot in full, Show in folder, Delete + confirm dialog |
| `internal/ui/line_selection.go`        | `lineSelection`: anchor/end range of whole lines (pure logic, tested) |
| `internal/ui/load.go`                  | `backgroundLoad`: one disk read at a time off the UI goroutine |
| `internal/ui/settings_page.go`         | Settings page                                               |
| `internal/ui/capture_ui.go`            | `ui.Services`, status text, data folder form, first-run prompts, text field |
| `internal/ui/button.go`, `dialog.go`   | Primary/secondary/toggle buttons; modal dialog, `card`, `panel`, `outlinedSurface` |
| `cmd/worldkit/rsrc_windows_amd64.syso` | Generated exe icon + manifest; linked automatically by `go build` |
| `internal/ui/app.go`                   | Event loop, `Shell` (page switching, reload on show), `pageTitle`/`caption` helpers |
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
  enough for a 160 dp hero up to ~240% display scaling while keeping the exe small.
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
- **Log viewer selects whole lines, not characters.** Each line is its own label so it keeps
  its in-game color (Gio's editor/selectable paints all text with one material, applied per
  wrapped line, so it can't color by chat line), and only on-screen lines are laid out. One
  pointer handler over the list maps the mouse y to a line using last frame's row heights
  (`chatLineList.lineAt`). Trade-off: no partial-line selection; whole lines are what players
  paste into forums and screenshot tools anyway.
- **Selection menu is drawn outside the lines' event area.** A Gio handler also gets events
  for areas drawn inside its clip, so a menu inside the list would start a new selection on
  every click. `chatLineList` lays the menu out after popping the list's clip and positions it
  from last frame's row heights (`lineBounds`).
- **Screenshots are rendered in pure Go**, not by Gio: `golang.org/x/image/font/opentype` draws
  onto an `image.RGBA`. No GPU, no window, unit-testable, and no new dependency (x/image was
  already required). Style follows the Chatlog Magician's output (bold Arial, black outline,
  transparent background).
- **Arial Bold is read from Windows, not bundled.** Its license forbids redistribution, but
  every Windows install has `%WINDIR%\Fonts\arialbd.ttf` (WorldKit is Windows-only). If it is
  missing or unreadable, the bundled Go Bold is used and a warning is logged. Roboto was
  considered; Arial matches the community's existing chat screenshots.
- **Screenshots live next to the logs** (`%AppData%\WorldKit\screenshots\`), flat, named by
  save time in UTC like session files (`2026-10-18_101530.png`, `_2` on collisions), written
  via a temp file + rename. `Store.Delete` refuses paths outside that folder. Previews are
  shown at 1 image px per screen px so text stays as crisp as in the file (no resampling).
- **Chat colors come from the GTA World Chatlog Magician**
  (github.com/flipperatol121/chatlog-magician, the FiveM-era fork), which recreates the in-game
  chat. One color per line: `[INFO]` lines are white, although the game draws the `[INFO]` tag
  blue; "near you" vs "far" brightness for says is not modeled.
- **Log filters live behind a Filters button** to keep the viewer header calm. Options in a group
  combine with OR, the Actions and Characters groups with AND; an empty group keeps everything.
  **IC** = everything in character: says, shouts, whispers, `/me` and `/do` (all but `/b`, PMs
  and server lines). `/me` and `/do` also have their own options to show only emotes.
- **Characters are detected, not typed:** every speaker in the session (`chatlog.Characters`),
  most active first. Picking one keeps lines they speak *or are mentioned in* (case-insensitive,
  `_` = space), so `* John Doe looks at Jane Roe.` shows under Jane Roe too. Only speakers are
  detected; a character who is only mentioned isn't listed.
- **Hiding timestamps is display-only.** `Record.RawWithoutTimestamp` drops the leading
  `[HH:MM:SS] ` when lines are shown and copied; the stored files are never changed.
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
  Don't queue `pointer.Drag` (the router panics); queue a `pointer.Move` with `Buttons` set and
  the router turns it into a drag.
- **CRLF checkout:** `core.autocrlf=true` checks files out with CRLF, so `gofmt -l .` lists
  unchanged files locally. Git stores LF; check with `git ls-files --eol`.
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
- Log viewer: live-update the open session as new lines arrive (today it reads once on open),
  free-text search, a PM action option, a search box for long character lists, colored tags
  inside a line (e.g. blue `[INFO]`).
- Screenshots: copy the image to the clipboard (Gio's clipboard is text-only, needs Win32),
  more selection-menu actions (Copy), screenshot style options (font size, wrap width,
  background), delete from the list without opening.
- Tray icon so WorldKit can keep capturing with its window closed.
- Folder picker dialog for the FiveM data folder (today the path is typed/pasted).
- Bring back more nav destinations when they have real content.
- Persist sidebar expanded/collapsed state and last page between launches.
