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
| App shell        | Done: undecorated window with WorldKit's own title bar (drag area, minimize/maximize/close), sidebar + page area |
| Look             | Done: redesign from the **WorldKit Redesign** canvas — dark graphite ground, one orange accent, Geist + Geist Mono, line icons |
| Sidebar          | Done: collapsible (232 ↔ 66 dp, 200 ms), brand, Logs / Log Screenshots, live capture status card, Settings at the bottom |
| Logs page        | Done: "Live now" card for the session being captured; sessions grouped Today / Yesterday / Earlier this week / month, each with start time, duration, characters, line count and a kind bar; search by character or date |
| Log viewer       | Done: always-visible toolbar (find text, kind segments All/IC/me/do/b/PMs, Characters menu, timestamps switch), filter chips + Clear filters, timestamp column, drag/Shift+click/Ctrl+A/Esc selection, floating selection bar (Copy, **Extract image**, ×), toasts ("Screenshot saved · View" opens it) |
| Log Screenshots  | Done: gallery grouped by day, checkerboard 1:1 previews, hover actions (Show in folder, Delete); detail view with Transparent/Dark/Light backdrop and details; delete with confirmation |
| Settings page    | Done: capture status, FiveM data folder with **Browse…** (Windows folder dialog), start with Windows switch, logs/screenshots folders |
| Branding         | Done: logo in sidebar header, exe/taskbar icon                         |
| Chat capture     | Done: background polling of FiveM's KVS, one `.txt` per session, crash-safe appends |
| Chat parsing     | Done: `chatlog.Parse` → records with time, kind, speaker, text; `chatlog.Filter`; `chatlog.Summarize` |
| First-run prompts| Done: FiveM data folder (only if not found; Browse… or type), start with Windows (asked once) |
| Persistence      | `%AppData%\WorldKit\settings.json`, `%AppData%\WorldKit\logs\YYYY\MM\*.txt`, `%AppData%\WorldKit\screenshots\*.png`, `%AppData%\WorldKit\diagnostics\` (`app.log`, `crash-*.log`) |
| Tests            | Animation, chat archive/parser/filter/library/summary, KVS reader, capture service, settings, diagnostics, folder picker setup, screenshot render/store, UI texts (days, durations, counts), filters, chat lines, selection menu placement, icons, gallery layout |

## Architecture

```
main()  (-minimized flag)               cmd/worldkit/main.go
 └─ run()                               cmd/worldkit/run.go  wiring: diagnostics, settings, capture, autostart, window
     ├─ capture.Service.Run(ctx)        internal/capture     goroutine, polls every 3 s
     │   ├─ fivem.ReadStamp / ReadChatLog   internal/fivem   copy KVS → temp, goleveldb read-only
     │   └─ chatlog.Archive.Save        internal/chatlog     append new lines to the session file
     └─ ui.Run(window, Services)        internal/ui/app.go   event loop: one Shell.Layout per frame
         └─ Shell                       internal/ui/app.go   sidebar | title bar over the page | prompt
             ├─ Sidebar                 internal/ui/sidebar.go  brand (move area), NavItems, captureCard, Settings
             ├─ titleBar                internal/ui/title_bar.go move area + caption buttons → window.Perform
             ├─ page: LogsPage | ScreenshotsPage | SettingsPage
             │   ├─ LogsPage            internal/ui/logs_page.go  liveCard + sessionRows ──► logViewer
             │   │   ├─ loadSessionEntries: Library.Sessions + Records → chatlog.Summarize (backgroundLoad)
             │   │   └─ logViewer       internal/ui/log_viewer.go logFilters → chatLineList
             │   │       └─ selectionMenu → Extract image → screenshot.Store.Save (background) → toast → View
             │   ├─ ScreenshotsPage     internal/ui/screenshots_page.go shotCards ──► screenshotView; shotDeletion
             │   └─ SettingsPage        internal/ui/settings_page.go dataDirForm (Browse… → folderpicker)
             └─ prompt: dataDirPrompt (capture needs a folder) | startupPrompt (not answered yet)
controls, text styles, icons            internal/ui/button.go, controls.go, text.go, icons.go
theme tokens + fonts                    internal/ui/theme.go (mirrors DESIGN.md), fonts.go (embedded Geist)
assets.Logo(), assets.Fonts             assets/assets.go     embedded logo.png and fonts/*.ttf
```

The UI talks to domain code only through the small interfaces in `ui.Services`
(`CaptureService`, `SettingsStore`, `LogLibrary`, `ScreenshotStore`, `StartupEntry`,
`FolderPicker`); capture calls `window.Invalidate` when its status changes, and so does
`backgroundLoad` when a disk read finishes. `Run` also records the window handle
(`app.Win32ViewEvent`) so the folder dialog is modal to WorldKit.

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
   "last updated" from its modification time, session # by age) and parses each one for its
   summary. The viewer parses one file and shows the lines that pass the filters, still exactly
   as stored.

Gio is immediate-mode: every frame, `Layout` methods handle input first, then draw. Widget state
(`widget.Clickable`, animation state, selection) lives in structs that persist across frames.

## File map

| Path                                   | Purpose                                                     |
|----------------------------------------|-------------------------------------------------------------|
| `cmd/worldkit/main.go`                 | Parses `-minimized`, starts `run` and the Gio main loop     |
| `cmd/worldkit/run.go`                  | Builds diagnostics, settings, capture, folder picker, startup entry, undecorated window (1280×820, min 900×600) |
| `internal/fivem/`                      | Finds FiveM's KVS, copies it, reads the GTA World chat log; MessagePack string decoder |
| `internal/chatlog/`                    | `Archive` (session files, lock, crash repair), `Parse` (line kinds, midnight rollover), `Library` (list/read sessions), `Filter` (names + kinds), `Characters` (speakers by line count), `Summarize` (counts by kind, time span) |
| `internal/capture/`                    | Background `Service`: polling loop, data folder changes, `Status` |
| `internal/settings/`                   | `settings.json` load + atomic save                          |
| `internal/diagnostics/`                | `Start`: `slog` → `app.log`, crash output → `crash-*.log`; `End`/`Close` |
| `internal/folderpicker/`               | Windows "Select folder" dialog (`IFileOpenDialog` via COM, no CGO) |
| `internal/autostart/`                  | Start with Windows via the HKCU `Run` registry key          |
| `internal/screenshot/`                 | `Render` (lines → transparent PNG, in-game look), `Store` (save/list/load/delete in `screenshots\`) |
| `internal/ui/app.go`                   | Event loop (window events, maximize state, HWND), `Shell` (pages, prompts, View link) |
| `internal/ui/title_bar.go`             | `titleBar`, `moveArea`, caption buttons and glyphs          |
| `internal/ui/sidebar.go`               | `Sidebar`, `NavItem`, `navRow`, `captureCard` (+ `captureCardTexts`) |
| `internal/ui/logs_page.go`             | Logs page: `loadSessionEntries`, `liveCard`, `sessionRow`, `kindBar`, `styledList` |
| `internal/ui/log_viewer.go`            | `logViewer`: header, toolbar, summary row, chat panel, toast |
| `internal/ui/log_filters.go`           | `logFilters`: find, kind segments, characters menu (deferred popover), chips, timestamps |
| `internal/ui/chat_line.go`             | `chatLine` (time column + colored text), `chatLineColor`, `screenshotLines` |
| `internal/ui/chat_line_list.go`        | `chatLineList`: scrolling lines, selection, copy text, places the selection bar |
| `internal/ui/selection_menu.go`        | `selectionMenu` (count, Copy, Extract image, ×), `selectionMenuOrigin` |
| `internal/ui/line_selection.go`        | `lineSelection`: anchor/end range of whole lines (pure logic, tested) |
| `internal/ui/screenshots_page.go`      | Log Screenshots gallery: `shotCard`, day groups, checkerboard, 1:1 image drawing |
| `internal/ui/screenshot_view.go`       | `screenshotView` (detail, backdrops, details card), `shotDeletion` (confirm + delete) |
| `internal/ui/settings_page.go`         | Settings page: `settingsCard`, `settingRow`, `pathRow`      |
| `internal/ui/capture_ui.go`            | `ui.Services` + interfaces, status text, `dataDirForm` (Browse…), first-run prompts, `openFolder` |
| `internal/ui/button.go`                | `button`: primary / secondary / ghost / danger / icon       |
| `internal/ui/controls.go`              | Segmented control, switch, chip, status pill, badge, link, `surfaceBox`, `textField`, `hoverArea` |
| `internal/ui/text.go`, `texts.go`      | Text styles + `pageHeader`; date/duration/count wording (tested) |
| `internal/ui/icons.go`                 | `strokeIcon`: line icons from a tiny SVG-path subset        |
| `internal/ui/dialog.go`                | Modal `dialog`, `card`, `outlinedSurface`, `floatingSurface` |
| `internal/ui/toast.go`                 | `toast`: timed confirmation with an optional link           |
| `internal/ui/wrap.go`                  | `layoutWrappedCentered`: wrapping row layout                |
| `internal/ui/load.go`                  | `backgroundLoad`: one disk read at a time off the UI goroutine |
| `internal/ui/animation.go`             | `expandAnimation` (reversible 0↔1 progress) + easing        |
| `internal/ui/logo.go`                  | `Logo` widget (sharp at any DPI)                            |
| `internal/ui/theme.go`, `fonts.go`     | All design tokens and `NewTheme()`; embedded-font shaper    |
| `assets/assets.go`, `assets/logo.png`, `assets/fonts/` | Embedded logo (384×384, the only logo source) and Geist/Geist Mono TTFs + `OFL.txt` |
| `cmd/worldkit/rsrc_windows_amd64.syso` | Generated exe icon + manifest; linked automatically by `go build` |
| `CLAUDE.md`                            | Code rules, commands, commit checklist                      |
| `DESIGN.md`                            | Tokens, typography, components and every screen's layout    |
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
- **Filters are always visible** above the log (redesign): a find field, one line kind at a
  time (**All · IC · /me · /do · /b · PMs**; **IC** = says, shouts, whispers, `/me` and `/do`),
  and characters from a menu, shown as removable chips. Characters combine with OR; kind,
  characters and find text with AND. A single kind is simpler to read than the old
  multi-toggle panel, and the filters' state is visible without opening anything.
- **Characters are detected, not typed:** every speaker in the session (`chatlog.Characters`),
  most active first. Picking one keeps lines they speak *or are mentioned in* (case-insensitive,
  `_` = space), so `* John Doe looks at Jane Roe.` shows under Jane Roe too. Only speakers are
  detected; a character who is only mentioned isn't listed.
- **Hiding timestamps is display-only.** `Record.RawWithoutTimestamp` drops the leading
  `[HH:MM:SS] ` when lines are shown and copied; the stored files are never changed.
- **Collapsed sidebar width is derived**, not hard-coded: `2 × (12 + 12) + 18 = 66 dp`, so icons
  sit at the same x position in both states and never jump during the animation.
- **The redesign comes from a Claude Design canvas** ("WorldKit Redesign": one artboard per
  screen and a type/color/component sheet). DESIGN.md is the source of truth in the repo; the
  canvas is the picture of it. Built for what a player does: check capture is running (sidebar
  card), find a session (who was in it, when, how long), read and filter it, extract lines.
- **Geist + Geist Mono, embedded** (OFL-1.1, `assets/fonts`, ~0.8 MB): Gio's default Go font
  looked dated. The shaper loads only embedded fonts, so text is identical on every PC. Mono is
  for times, counts and paths. Screenshot images keep Arial Bold to match the game.
- **Own title bar** (`app.Decorated(false)` + `titleBar`): the white Windows caption clashed
  with the dark UI. Gio marks the empty strip as the move area (`system.ActionInputOp`), so
  Windows keeps snapping, edge resizing and double-click maximize; the buttons call
  `window.Perform`.
- **Line icons are drawn from path strings** (`strokeIcon`), not Material icons: they match
  the design's stroke style and need no icon dependency.
- **Folder picker via COM** (`internal/folderpicker`): the modern Explorer dialog
  (`IFileOpenDialog`, folder mode) through `x/sys/windows` syscalls, run on its own locked OS
  thread off the UI goroutine. No new dependency and no CGO; the older `SHBrowseForFolder`
  tree dialog was the alternative.
- **Session list parses every session** to show characters, duration and kinds
  (`chatlog.Summarize`), off the UI goroutine. Fine for hundreds of sessions; cache summaries
  if history grows much larger.

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
- **Crashes are silent in the release build** (no console). Read
  `%AppData%\WorldKit\diagnostics\crash-*.log` for the panic trace and `app.log` /
  `app.previous.log` for what led up to it. Under the Claude desktop sandbox, `%AppData%`
  writes are redirected (see above), so look in the real folder when the app runs normally.
  A crash file left behind means that run didn't exit cleanly (the next start logs it).
- **Don't close diagnostics in a plain `defer` in `run`.** A panic in the UI goroutine
  unwinds through it; closing there released crash output and deleted the empty crash file
  before the runtime wrote the trace, so the app just vanished. `run` defers
  `Session.End(&runErr)`, which logs the panic and panics again instead of closing.
- **`layout.List.ScrollBy` leaves `Position.First` out of range** until the next layout
  (e.g. -1 after scrolling up at the top). Clamp before indexing per-line data:
  `chatLineList.firstShownLine`. Dragging a selection above the top of a log crashed the app
  this way (index -1).
- **`moveArea` must stay clear of buttons** and sizes itself from `Constraints.Min`: inside a
  `layout.Stack` `Max` is the whole window, which once stretched the sidebar header to full height.
- **Gio blends in linear light**: a 12% tint looks much stronger than in a browser. Tint
  opacities in `theme.go` are tuned down (0.06–0.07) to look like the design.
- **A hover tracker must wrap what it tracks** (`hoverArea` registers itself as the parent):
  an area registered on top of buttons takes their clicks, which once made screenshot cards
  unclickable.
- **Labels squeezed to ~0 width wrap one character per line** and grow very tall, pushing
  siblings out (this shrank the screenshot preview). Give captions in tight rows `MaxLines = 1`.
- **Popovers use `op.Defer`** (the Characters menu) so they draw above later siblings, with a
  transparent window-sized layer under them that closes them on an outside click.
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
  a search box for long character lists, colored tags inside a line (e.g. blue `[INFO]`).
- Screenshots: **Copy image** to the clipboard (Gio's clipboard is text-only; needs Win32
  `CF_PNG`/`CF_DIB`), link a screenshot back to its session, style options (font size, wrap
  width).
- Sidebar counts next to Logs / Log Screenshots (the design shows them).
- Cache session summaries if the session list gets slow.
- Tray icon so WorldKit can keep capturing with its window closed.
- Bring back more nav destinations when they have real content.
- Persist sidebar expanded/collapsed state and last page between launches.
