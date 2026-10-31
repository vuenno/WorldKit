# WorldKit — Project Overview

Context for agents and new contributors: what exists, why it is built this way, and where things
are. **Read this first, then `CLAUDE.md` (code rules) and `DESIGN.md` (visual rules).**
Keep this file current: update it in the same change whenever features, structure or key
decisions change.

_Last updated: 2026-10-31_

## What it is

WorldKit is a small, **Windows-only** (amd64) desktop application written in Go with the Gio UI
toolkit. Its visual identity is a dark UI with a single orange accent and the "WK WorldKit"
logo (sunset city skyline). It is a companion tool for players of **GTA World** (GTA:W), a
roleplay server on FiveM. Its first feature is **chat log capture**: saving the in-game chat,
which the game itself throws away on every reconnect, to plain text files on disk, and
browsing them in a **log viewer** with name and kind filters. Selected lines can be
**extracted** into transparent PNG "chat screenshots" (in-game look) for laying over game
screenshots, kept on the **Log Screenshots** page. The **Screenshot Editor** does that
laying over: pick a canvas size, add game screenshots and chat images as layers (move,
resize, rotate, opacity, order) and export a PNG or JPEG at a chosen quality; exports are
listed on the **Saved Screenshots** page.

## Current state

| Area             | Status                                                                 |
|------------------|------------------------------------------------------------------------|
| App shell        | Done: undecorated window with WorldKit's own title bar (drag area, minimize/maximize/close), sidebar + page area |
| Look             | Done: redesign from the **WorldKit Redesign** canvas — dark graphite ground, one orange accent, Geist + Geist Mono, line icons |
| Sidebar          | Done: collapsible (232 ↔ 66 dp, 200 ms), brand, Logs / Log Screenshots, a divider, Screenshot Editor (badge while it has layers) / Saved Screenshots, live capture status card, Settings at the bottom |
| Screenshot Editor| Done: canvas size from templates, a custom W×H, a preset or the first image; layers from game screenshots (multi-select Open dialog; PNG/JPEG/BMP/WebP ≤ 8192 px) and chat images (picker over Log Screenshots); drag to move with snapping to the canvas edges/center, corner/edge handles to resize (aspect kept, Shift free), rotation handle (Shift 15°), inspector fields (X/Y/W/H/rotation/opacity slider), Fit canvas / Actual size, duplicate, delete, hide, reorder (list buttons, `[` `]`, Ctrl+↑/↓), undo/redo, Ctrl+wheel zoom and pan, canvas background None/Black/White; **Close** returns to the template picker (asks first when there are layers); properties and layers sit in a compact 216 dp panel (24 dp controls) so the canvas gets the room |
| Export           | Done: dialog with PNG (Lossless or Smaller file = reduced colors, 10–100 %) or JPEG (quality 10–100 %, default 92), presets (format + quality + canvas size, saved from the dialog, default in Settings), Save As (only the chosen type), rendered off the UI goroutine, toast with Show in folder |
| Saved Screenshots| Done: every export (from `exports.json`) grouped by day: thumbnail, name, folder, size, format, file size; detail view; Show in folder; Delete (confirmed; only listed, unchanged files); missing files can be removed from the list |
| Logs page        | Done: "Live now" card for the session being captured; sessions grouped Today / Yesterday / Earlier this week / month, each with start time, duration, characters, line count and a kind bar; search by character or date |
| Log viewer       | Done: always-visible toolbar (find text, kind segments All/IC/me/do/b/PMs, Characters menu, timestamps switch), filter chips + Clear filters, timestamp column, drag/Shift+click/Ctrl+A/Esc selection, floating selection bar (Copy, **Extract image**, ×), toasts ("Screenshot saved · View" opens it) |
| Extract dialog   | Done: **Extract image** opens a modal with a live preview, the Transparent/Dark/Light backdrop, the game's chat look as set in its settings — **Font** (Arial · Mukta) and **Chat scale** (50–150 %, steps of 5, default 100; the image grows with it) — and **Max width** (200–1800 px; lines wrap to fit, default the in-game chat width at any scale); "Save as they are, or edit them first?" → **Edit** makes the preview itself editable: drag/Shift+click/Ctrl+A to select text on the image, **Italic**, a color picker (chat colors) and **Default** apply to it; click and type, Backspace/Delete/Enter, arrows/Home/End, Ctrl+Z undo; **reorder lines** by dragging the grip beside each (or Alt+Up/Down); **Merge lines** joins the selected lines one person said ("X says: A." + "X says: B." → "X says: A. B."); **presets** of background, chat font and scale, and max width (older font-size presets get the nearest scale): pick one in the dialog, **Save as preset…**, the default one applies to the first extract; **Copy image** (or Ctrl+C while the dialog is open) puts exactly what Save image would write on the clipboard (dialog stays open, toast "Image copied to clipboard") |
| Screenshot look  | Done: markup `/italic/` and `*action*` (in the /me color), markers removed; Dark/Light backdrops are a box behind each row of text, transparent elsewhere |
| Log Screenshots  | Done: gallery grouped by day, checkerboard 1:1 previews, hover actions (Show in folder, Delete); detail view with a Transparent/Dark/Light preview (boxes behind the text rows), **Copy image** (on the chosen background) and details; delete with confirmation |
| Settings page    | Done: capture status, FiveM data folder with **Browse…** (Windows folder dialog), start with Windows switch, Log Screenshots card (extract presets: make default, delete), Screenshot Editor card (export folder with Browse…, export presets: make default, delete), logs/screenshots folders, Updates card (automatic checks switch, version + **Check now**) |
| Auto-update      | Done: at startup and about once a day (`update.CheckInterval`, 20 h; failures retried after an hour, only logged) asks GitHub for the latest `v<version>` release of `vuenno/WorldKit` and compares it as semver with the build's version (a `dev` build never checks); a card in the page area's bottom-left offers it (**Update now**, **Later** = 24 h, **Skip this version**, **Release notes**); Update now downloads `WorldKit-Setup-<version>.exe` to `%TEMP%\WorldKit-update`, checks its size and SHA-256 (the release's `.sha256` asset), runs it silently and quits; the installer waits for WorldKit to exit, upgrades in place and starts it again. Not yet tried end to end with two real releases |
| Branding         | Done: logo in sidebar header, exe/taskbar icon                         |
| Chat capture     | Done: background polling of FiveM's KVS, one `.txt` per session, crash-safe appends |
| Chat parsing     | Done: `chatlog.Parse` → records with time, kind, speaker, text; `chatlog.Filter`; `chatlog.Summarize` |
| First-run prompts| Done: FiveM data folder (only if not found; Browse… or type), start with Windows (asked once) |
| Persistence      | `%AppData%\WorldKit\settings.json` (incl. export folder and presets, last update check, skipped/Later answers), `%AppData%\WorldKit\exports.json` (the export list), `%AppData%\WorldKit\logs\YYYY\MM\*.txt`, `%AppData%\WorldKit\screenshots\*.png`, `%AppData%\WorldKit\diagnostics\` (`app.log`, `crash-*.log`, `update-install.log`); exports wherever the user saves them (default `Pictures\WorldKit`) |
| Releases         | Done: manual **Release** workflow (`.github/workflows/release.yml`) runs the checks, builds with the typed version and publishes an Inno Setup installer and its `.sha256` under a new `v<version>` tag |
| Tests            | Animation, chat archive/parser/filter/library/summary, KVS reader, capture service, settings, diagnostics, folder picker setup, screenshot render/chat geometry against Chromium's measures (sizes, row heights, baselines, gamma buckets and curve, fake bold, hyphen breaks)/store (look round trip, older screenshots' row grids)/markup/document edits/hit-testing/backdrop boxes (no side padding, baked backdrop for copying), clipboard bitmap layout (CF_DIBV5), DirectWrite glyph masks (against Chromium's), extract dialog and picture editor (lines, style, chat look, old presets, keys, restyle, undo), number stepper, UI texts (days, durations, counts), filters, chat lines, selection menu placement, icons, gallery layout; compose (transform, hit-testing, resize/rotate incl. rotated, snapping, reorder, canvas resize, history, placement, linear-light blending, exact 1:1 pixels, missing source, all formats, quantizer alpha), exports index (add/list/missing/remove, delete only listed unchanged files), file dialogs (COM setup, extensions), settings presets; update (semver parse/compare, check schedule, Skip/Later, GitHub release parsing and URL checks, download size/hash checks, installer switches, service: dev builds, checks off, manual checks) and update status texts; benchmarks for 4K render/encode |

## Architecture

```
main()  (-minimized flag)               cmd/worldkit/main.go
 └─ run()                               cmd/worldkit/run.go  wiring: diagnostics, settings, capture, updates, autostart, window
     ├─ capture.Service.Run(ctx)        internal/capture     goroutine, polls every 3 s
     ├─ update.Service.Run(ctx)         internal/update      goroutine, wakes hourly; asks GitHub when due or on Check now
     │   ├─ fivem.ReadStamp / ReadChatLog   internal/fivem   copy KVS → temp, goleveldb read-only
     │   └─ chatlog.Archive.Save        internal/chatlog     append new lines to the session file
     └─ ui.Run(window, Services)        internal/ui/app.go   event loop: one Shell.Layout per frame
         └─ Shell                       internal/ui/app.go   sidebar | title bar over the page (+ updatePrompt) | prompt
             ├─ Sidebar                 internal/ui/sidebar.go  brand (move area), NavItems, captureCard, Settings
             ├─ titleBar                internal/ui/title_bar.go move area + caption buttons → window.Perform
             ├─ page: LogsPage | ScreenshotsPage | EditorPage | ExportsPage | SettingsPage
             │   ├─ LogsPage            internal/ui/logs_page.go  liveCard + sessionRows ──► logViewer
             │   │   ├─ loadSessionEntries: Library.Sessions + Records → chatlog.Summarize (backgroundLoad)
             │   │   └─ logViewer       internal/ui/log_viewer.go logFilters → chatLineList
             │   │       └─ selectionMenu → Extract image → extractDialog (preview via screenshot.Render, edits)
             │   │           → Save image → screenshot.Store.Save(lines, style) (background) → toast → View
             │   ├─ ScreenshotsPage     internal/ui/screenshots_page.go shotCards ──► screenshotView; shotDeletion
             │   ├─ EditorPage          internal/ui/editor_page.go templatePicker ──► workspace
             │   │   ├─ editorDoc       compose.Composition + History, selection, images (imageLoader: decode → 2048 px RGBA → ImageOp)
             │   │   ├─ canvasView      GPU preview (op.Affine per layer), gestures → compose.Moved/Resized/Rotated/Snapped
             │   │   ├─ layerInspector, layerList, chatPicker (Log Screenshots as layers)
             │   │   └─ exportDialog    Save As (filedialog) → compose.Export (CPU, linear light) → exports.Index.Add
             │   ├─ ExportsPage         internal/ui/exports_page.go exportRows (lazy thumbnails) ──► exportView; shotDeletion
             │   └─ SettingsPage        internal/ui/settings_page.go dataDirForm (Browse… → filedialog), exportSettings
             ├─ updatePrompt            internal/ui/update_prompt.go  Update now → Updates.Install (backgroundLoad) → close window
             └─ prompt: dataDirPrompt (capture needs a folder) | startupPrompt (not answered yet)
controls, text styles, icons            internal/ui/button.go, controls.go, text.go, icons.go
theme tokens + fonts                    internal/ui/theme.go (mirrors DESIGN.md), fonts.go (embedded Geist)
assets.Logo(), assets.Fonts             assets/assets.go     embedded logo.png and fonts/*.ttf
assets.MuktaRegular                     assets/chatfonts/    Mukta for chat screenshots only
```

The UI talks to domain code only through the small interfaces in `ui.Services`
(`CaptureService`, `SettingsStore`, `LogLibrary`, `ScreenshotStore`, `StartupEntry`,
`FileDialogs`, `ExportList`, `UpdateService`, plus `DefaultExportDir` and `Version`); capture and the update service call `window.Invalidate` when their status changes, and so does
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
| `cmd/worldkit/run.go`                  | `appVersion` (set by the release build); builds diagnostics, settings, capture, the update service, folder picker, startup entry, undecorated window (1280×820, min 900×600) |
| `internal/fivem/`                      | Finds FiveM's KVS, copies it, reads the GTA World chat log; MessagePack string decoder |
| `internal/chatlog/`                    | `Archive` (session files, lock, crash repair), `Parse` (line kinds, midnight rollover), `Library` (list/read sessions), `Filter` (names + kinds), `Characters` (speakers by line count), `Summarize` (counts by kind, time span) |
| `internal/capture/`                    | Background `Service`: polling loop, data folder changes, `Status` |
| `internal/settings/`                   | `settings.json` load + atomic save                          |
| `internal/update/`                     | Self-update: `Version` (semver), `Client` (`Latest` from GitHub's API, `Download` with size + SHA-256 checks), `StartInstaller` (silent Inno Setup switches), `IsCheckDue` / `ShouldOffer`, `Service` (background checks, `Offer`, `RemindLater`, `Skip`, `Install`) |
| `internal/diagnostics/`                | `Start`: `slog` → `app.log`, crash output → `crash-*.log`; `End`/`Close` |
| `internal/filedialog/`                 | Windows file dialogs via COM, no CGO: `PickFolder`, `OpenFiles` (multi-select), `Save` (one file type, extension enforced) |
| `internal/clipboard/`                  | `CopyImage`: an image on the Windows clipboard as `PNG` (transparency kept) and `CF_DIBV5`, via user32, no CGO |
| `internal/directwrite/`                | DirectWrite via COM, no CGO: `Open` a font file, `GlyphMask` (one glyph at a fractional pen position, ClearType averaged to gray, as Chromium draws chat text) |
| `internal/compose/`                    | Screenshot Editor model: `Composition`/`Layer` (center-based), geometry.go (`Transform`, hit-testing, handles, `Resized`, `Rotated`, `Snapped`), `History` (undo/redo), placement.go, decode.go (`ReadSize`, `Decode`, `Downscale`), render.go + linear.go (CPU export in linear light), encode.go (`Format`, PNG/JPEG), quantize.go (median cut + dithering), `Export` |
| `internal/exports/`                    | `Index`: `exports.json` list of exports; `List` marks missing/changed files; `Delete` only listed, unchanged files |
| `internal/atomicfile/`                 | `Write`: temp file + sync + rename, for settings, screenshots, exports |
| `internal/autostart/`                  | Start with Windows via the HKCU `Run` registry key          |
| `internal/screenshot/`                 | `Render` (lines + `Style` → `Picture`, the in-game chat as Chromium draws it, backdrop), chat.go (`ChatFont`, `Look`, chat scales, the game's chat CSS as measures: `chatGeometry`), font.go (Arial via DirectWrite, bundled Mukta, fallbacks, caches), glyphs.go (DirectWrite and fake-bold outline rasterizers, Skia's ClearType filter, glyph cache), gamma.go (Skia's per-color coverage tables), shadow.go (the triple text-shadow), `RowBoxes` (boxes of a saved image on its `RowGrid`), layout.go (per-character placement, kerning, wrapping at spaces and hyphens), picture.go (`PositionAt`, `CaretBounds`, `SelectionBounds`, `LineBounds`), `NewLine` / `ParseMarkup` (markup.go), `Document` (editable styled characters; lines.go: `MoveLine`, `SelectedLines`, `CanMergeLines` / `MergeLines`), `Store` (save/list/load/delete in `screenshots\`; `Load` → `SavedImage` with its row grid), pngtext.go (the look in a PNG tEXt chunk; older screenshots' font size) |
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
| `internal/ui/extract_dialog.go`        | `extractDialog`: preview, backdrop, chat font and scale, max width, Italic, color swatches, Default; `layoutOptionRow`, `fittedSize` |
| `internal/ui/picture_editor.go`        | `pictureEditor`: select, type and undo on the rendered preview image |
| `internal/ui/picture_lines.go`         | `pictureEditor` line tools: grips and drag to reorder, Alt+Up/Down, Merge lines |
| `internal/ui/screenshot_view.go`       | `screenshotView` (detail, backdrop boxes, details card), `backdropColor`, `shotDeletion` (confirm + delete) |
| `internal/ui/settings_page.go`         | Settings page: `settingsCard`, `settingRow`, `pathRow`      |
| `internal/ui/export_settings.go`       | Settings' Screenshot Editor card: export folder, presets    |
| `internal/ui/extract_settings.go`      | Settings' Log Screenshots card: extract presets             |
| `internal/ui/extract_presets.go`       | Extract dialog presets: apply, save, menu, descriptions     |
| `internal/ui/preset_naming.go`         | `presetNaming`: "Save as preset…" link, name field, Save/Cancel |
| `internal/ui/editor_page.go`           | `EditorPage`: toolbar, workspace, adding images, shortcuts, `confirmPrompt`, `layoutEnabled` |
| `internal/ui/editor_doc.go`            | `editorDoc`: composition, history, selection, decoded layer images |
| `internal/ui/editor_canvas.go`         | `canvasView`: fit/zoom/pan, drawing layers, selection frame, guides, gestures, keys |
| `internal/ui/editor_panels.go`         | `layerInspector` (fields, slider, buttons) and `layerList` (rows) |
| `internal/ui/editor_templates.go`      | `templatePicker`: canvas size tiles, custom size, presets   |
| `internal/ui/export_dialog.go`         | `exportDialog`: format/quality/preset, Save as preset, Save As + background export |
| `internal/ui/chat_picker.go`           | `chatPicker`: pick Log Screenshots to add as layers         |
| `internal/ui/exports_page.go`          | Saved Screenshots: `exportRow`, lazy thumbnails, `exportView` |
| `internal/ui/image_loader.go`          | `imageLoader`: decode images off the UI goroutine, 2 at a time, cancellable |
| `internal/ui/inputs.go`, `menu.go`     | `numberField`, `slider`; `dropdownMenu`                      |
| `internal/ui/capture_ui.go`            | `ui.Services` + interfaces, status text, `dataDirForm` (Browse…), first-run prompts, `openFolder` |
| `internal/ui/button.go`                | `button`: primary / secondary / ghost / danger / icon       |
| `internal/ui/controls.go`              | Segmented control, switch, chip, status pill, badge, link, `surfaceBox`, `textField`, `hoverArea` |
| `internal/ui/text.go`, `texts.go`      | Text styles + `pageHeader`; date/duration/count wording (tested) |
| `internal/ui/icons.go`                 | `strokeIcon`: line icons from a tiny SVG-path subset        |
| `internal/ui/dialog.go`                | Modal `dialog`, `card`, `outlinedSurface`, `floatingSurface` |
| `internal/ui/toast.go`                 | `toast`: timed confirmation with an optional link           |
| `internal/ui/image_copy.go`            | `imageCopy`: draw + copy an image off the UI goroutine, then toast; `layoutCornerToast` |
| `internal/ui/wrap.go`                  | `layoutWrappedCentered`: wrapping row layout                |
| `internal/ui/load.go`                  | `backgroundLoad`: one disk read at a time off the UI goroutine |
| `internal/ui/update_prompt.go`         | `UpdateService`, `updatePrompt` (the update card), `updateStatusText`, `openURL` |
| `internal/ui/animation.go`             | `expandAnimation` (reversible 0↔1 progress) + easing        |
| `internal/ui/logo.go`                  | `Logo` widget (sharp at any DPI)                            |
| `internal/ui/theme.go`, `fonts.go`     | All design tokens and `NewTheme()`; embedded-font shaper    |
| `assets/assets.go`, `assets/logo.png`, `assets/fonts/` | Embedded logo (384×384, the only logo source) and Geist/Geist Mono TTFs + `OFL.txt` |
| `assets/chatfonts/`                    | `Mukta-Regular.ttf` (Google Fonts' Mukta, the file the game loads) + its `OFL.txt`; screenshots only, never the UI |
| `cmd/worldkit/rsrc_windows_amd64.syso` | Generated exe icon + manifest; linked automatically by `go build` |
| `.github/workflows/release.yml`        | Manual release: checks, versioned build, installer, tag + GitHub release |
| `installer/worldkit.iss`               | Inno Setup script: per-user install, Start menu entry, uninstall keeps user data; self-update switches `/WAITPID` and `/RELAUNCH` (`[Code]`) |
| `README.md`, `docs/screenshots/`       | Public front page (logo, release badge, features, install); its images: chat images rendered by `screenshot.Render`, app screenshots and two RP scenes captured from WorldKit run with a throwaway `%APPDATA%` of made-up sessions (no real logs or names) |
| `CLAUDE.md`                            | Code rules, commands, commit checklist                      |
| `DESIGN.md`                            | Tokens, typography, components and every screen's layout    |
| `.graphifyignore`, `graphify-out/`     | Knowledge-graph config and output (see below)               |

## Key decisions (and why)

- **Gio over Fyne.** The dev machine has no C compiler and CGO is off. Fyne needs CGO + MinGW;
  Gio is pure Go on Windows, lightweight, and ships Material-style widgets.
- **Windows only.** Product decision. No cross-platform code paths or resources.
- **Logo resampled at runtime** (`golang.org/x/image/draw`, Catmull-Rom) to its exact pixel
  size, so it stays crisp. (An earlier note here said Gio has no mipmaps; Gio v0.10.3 does
  mipmap `FilterLinear` images on D3D11 — `gpu.go` picks `FilterLinearMipmapLinear` and the
  backend calls `GenerateMips` — so shrunken images are smooth, just softer than a CPU
  resample.) The PNG is 384 px: enough for a 160 dp hero up to ~240% display scaling while
  keeping the exe small.
- **Screenshot Editor: GPU preview, CPU export.** The canvas draws each layer with Gio
  (`op.Affine` + `paint.PushOpacity` only when faded), from a preview copy decoded once off
  the UI goroutine and shrunk to ≤ 2048 px (Catmull-Rom; a premultiplied `*image.RGBA`, which
  `paint.NewImageOp` uploads without a copy). The decoded original is dropped; export
  re-decodes from the file. 2048 also keeps every texture under the D3D11 limits (an
  oversized texture panics in Gio). Export (`compose.Render`) runs on the CPU at full size,
  one layer at a time (peak ≈ canvas + one image: ~150 MB at 4K): Catmull-Rom scale into a
  2 px transparent margin (soft rotated edges), then `draw.Transform` for rotation in
  parallel horizontal bands; a layer at its real size on whole pixels is copied exactly, so
  chat text keeps every pixel. Measured: 4K canvas with two full layers renders in ~0.35 s.
  One `Layer.Transform` (center-based: translate · rotate · scale) drives both, so the file
  matches the preview geometrically.
- **Export blends in linear light**, like Gio's sRGB render target does on screen, so a 50 %
  layer looks the same in the file as in the editor (white at 50 % over black = 188, not
  128). Each premultiplied sRGB channel goes straight through a 256-entry table to 16-bit
  linear (as the GPU samples it), opacity and Over are applied there, and a 4096-entry table
  converts back. Gamma-space blending (`draw.Over`) was the alternative; it visibly
  disagreed with the preview on faded layers and soft edges.
- **Formats.** PNG lossless (stdlib, default compression; opaque images are written as RGB
  automatically). PNG "Smaller file": our own median-cut palette (≤ 256 colors incl. alpha)
  and Floyd–Steinberg dithering with a per-bin nearest-color cache (the stdlib dither
  searches the whole palette per pixel: seconds at 4K); quality maps to 16–256 colors, below
  50 % no dithering; fully transparent pixels get their own entry and no error diffusion
  (diffusing alpha left blotches around soft edges). JPEG is always lossy (no lossless JPEG in
  the stdlib, 4:2:0 fixed), quality 10–100, default 92, flattened over black.
- **Exports are listed, not copied.** `exports.json` records each export's path, format,
  size and the file's size + modification time; Saved Screenshots stats them on load. Delete
  removes only a listed file that still matches (another file that took the name is never
  deleted); missing ones can be removed from the list. Thumbnails are decoded only for rows
  that are drawn, 2 at a time, kept in memory (≤ 128), never cached on disk.
- **Save As offers only the chosen format** (one file type, extension enforced), so the
  quality settings always match the file. The dialog opens in the last export folder, else
  Settings' export folder, else `Pictures\WorldKit`.
- **Presets** (settings.json) hold format, quality and canvas size. Saved from the export
  dialog; usable as canvas sizes on the template screen and in the canvas menu (which also
  makes the next export start with them); the Settings default is where the export dialog
  starts the first time.
- **The Extract dialog reads Ctrl+C before the log lines do.** The lines keep keyboard
  focus behind the dialog, and Gio hands a key event to the first `gtx.Event` call in the
  frame that matches it, so `logViewer.handleInput` updates the dialog first; its Ctrl+C
  filter has no focus tag, so it fires whatever is focused (the lines, or the picture
  editor after Edit).
- **Images are copied through Win32, not Gio** (`internal/clipboard`; Gio's clipboard is
  text-only): as `PNG`, which keeps transparency and is what Discord, browsers and image
  editors paste, plus `CF_DIBV5` (straight alpha) for bitmap-only apps like Paint. Chat
  images only (Extract dialog, Log Screenshot view); copying from the Screenshot Editor is
  still a next step.
- **No drag-and-drop or paste of images**: Gio on Windows delivers neither file drops nor
  image clipboard data, so images come in through the Open dialog (multi-select).
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
- **Screenshots reproduce the game's chat as Chromium draws it**, not by Gio: WorldKit
  renders onto an `image.RGBA` itself (no GPU, no window, unit-testable). GTA World's chat is
  a web page in FiveM's browser (CEF, Chromium); its stylesheet was read with `nui_devtools`
  (FiveM dev mode: launch with `+set moo 31337`): `.chat` is `font-size: 1.55vh`, weight
  700, `letter-spacing: 0.4px`, the player's **chat font** (Arial or Mukta) and **chat
  scale** multiply the size, lines are that size + 0.2 × 1.55vh tall, `.chat__messages` is
  `40vw` wide with `text-shadow: 0 0 0.18vh #000` three times. WorldKit uses those values
  at 1920×1080. Every Chromium step was then matched against headless Chrome/Edge drawing
  the same CSS on a transparent page, pixel for pixel (average error ≈ 1.5/255 for Arial,
  ≈ 3/255 for Mukta, shadows and wrapping included):
  - font size truncated to hundredths in float32 (its font cache: 10.044 px → 10.04), line
    heights rounded to 1/64 px (layout units), baseline = row top + round(ascent) +
    floor(half-leading), y rounded to whole pixels;
  - unhinted advances + GPOS kerning + 0.4 px spacing, each glyph drawn at its x rounded to
    the nearest quarter pixel;
  - Arial (a real Bold face) through **DirectWrite** (`internal/directwrite`): natural mode,
    vertical grid fit, ClearType coverage averaged to gray — Chromium's Skia does exactly
    this; Go's rasterizers were visibly off;
  - Mukta: the game loads only Mukta Regular from Google Fonts, so its bold is **Skia's
    fake bold**: the outline grows by size × 1/24…1/32 (FreeType-style emboldening here),
    drawn at 4× width through Skia's 12-tap ClearType filter and averaged; italic is a 0.25
    skew; characters Mukta lacks fall back to Arial (the CSS `sans-serif`);
  - Skia's mask gamma per text color: channels reduced to 3 bits, Skia's luminance, 8
    buckets, gamma 2.2 and contrast 1.0 (Windows' text settings);
  - the shadow: the run's glyphs as black text, blurred with a measured 5-tap kernel
    (Skia's small blur isn't a plain Gaussian), stacked three times, under each run.
  The game itself may still blend its browser layer onto the frame differently (a JPEG from
  a player looked slightly heavier than Chromium's own output); that last step needs a
  lossless in-game capture to measure.
- **Arial is read from Windows, not bundled; Mukta is.** Arial's license forbids
  redistribution, but every Windows install has `%WINDIR%\Fonts\arialbd.ttf` and
  `arialbi.ttf`. If they are missing, the bundled Go Bold / Bold Italic are used (drawn from
  outlines) and a warning is logged; if DirectWrite fails, Arial is drawn from outlines.
  Mukta (OFL) is embedded from `assets/chatfonts`, kept apart from `assets/fonts` so the UI
  never loads it. Fonts, faces and drawn glyphs are cached, because the extract dialog
  re-renders its preview on every keystroke.
- **Chat markup becomes styled characters.** `/italic/` and `*action*` (in the /me color)
  are parsed once when the dialog opens (`NewLine`); both markers are removed from the
  image. Markers only count at word edges, so `and/or`, `1/2`, `2 * 3` and the `* ` that
  starts a /me line stay literal. The first version kept the asterisks and colored with
  `!{#RRGGBB}` codes typed into one text field per line; users found the codes and fields
  clumsy, so styles now live per character in `screenshot.Document`, not in the text.
- **Editing happens on the preview image** (`pictureEditor`), not in text fields: Gio's
  editor can't color ranges, and the picture is what the user wants to change. `Render`
  places every character itself (`layout.go`, kerning within a face) and returns a
  `Picture` that maps image points to caret positions and back, so clicks, drags, the caret
  and the selection highlight line up with the drawn glyphs. Every line is rendered,
  empty ones too, so an emptied line can still be clicked; only non-empty lines are
  saved. The preview re-renders off the UI goroutine after each change and may trail the
  text by a keystroke. Typed text takes the style of the character before it. Undo keeps
  whole-document copies (a few lines each, capped at 200).
- **Lines move by grips, merge by speaker.** Each line has a grip in the margin left of the
  edited preview (the margin widens by `lineGripWidth` while editing); dragging it shows
  where the line lands, and the whole line moves (Alt+Up/Down does the same from the
  keyboard). Merge lines works on the lines the selection touches: it needs two or more
  with text, every one "speech" with the same prefix up to the first ": " once a leading
  `[HH:MM:SS] ` is skipped ("Mason Fletcher says: "), and lines starting with `*`, `(` or
  `[` (emotes, OOC, [INFO]) never count, mirroring chatlog's says/shouts/whispers rules.
  So "says" and "shouts" by one person don't merge, which keeps the result truthful. The
  first line keeps its timestamp; later lines add their words after a space, styles
  kept. Lines must be next to each other (empty lines between are dropped), which is why
  merging comes with reordering.
- **Backdrops are a box per row, not a fill.** Dark (`#000000`, the Magician's default) and
  Light draw a box behind each drawn row, exactly as wide as that row's text (no side
  padding, so no black reaches past a row's first or last letter; the Magician's 5 px was
  dropped on request),
  rows flush, the rest of the image transparent: the same per-line `inline-block`
  background the Magician draws, so it reads like familiar chat screenshots and covers as
  little of the game picture as possible. One box around the whole block was the
  alternative; it hides more of the screenshot under short lines. The choice is baked into
  the PNG; the detail view previews it on any shot via `screenshot.RowBoxes`, which finds
  the rows on the shot's `RowGrid`.
- **The look is the game's own settings: Font and Chat scale**, not a pixel size, so
  players enter what they see in game. It replaced a Font size stepper (10–24 px), which
  only approximated the game. Each PNG records its look in a `tEXt` chunk
  (`WorldKit:Chat` = "Arial 60", spliced in by pngtext.go because `image/png` can't write
  text chunks) so the gallery can find its rows (fractional row tops, as in game); older
  files have `WorldKit:FontSize` or nothing (14 px) and keep their old row grid. Old
  presets' font size becomes the nearest chat scale. A file name suffix was the
  alternative; it breaks when users rename.
- **Max width caps the image, defaulting to the in-game chat width.** The game's chat box is
  `40vw` (758 px of text at 1080p) at every scale, so bigger text wraps into more rows, as
  in game; players can widen their chat in game, so the width stays adjustable. The limit
  is an image width in pixels, since that is what has to fit (small canvases, 500 × 500, in
  the Screenshot Editor). Lines wrap at spaces and after a hyphen between letters (as
  Chromium does: "L-C-C"); over-long words break between characters so the limit always
  holds. Manual breaks stay separate lines (Enter in the picture editor), each wrapping on
  its own. It isn't recorded in the PNG: rows are found from the look alone.
- **Screenshots live next to the logs** (`%AppData%\WorldKit\screenshots\`), flat, named by
  save time in UTC like session files (`2026-10-18_101530.png`, `_2` on collisions), written
  via a temp file + rename. `Store.Delete` refuses paths outside that folder. Previews are
  shown at 1 image px per screen px so text stays as crisp as in the file (no resampling).
- **Chat colors come from the GTA World Chatlog Magician**
  (github.com/flipperatol121/chatlog-magician, the FiveM-era fork), which recreates the in-game
  chat. One color per line (`says [low]` light gray `#C6C4C4`, `says [lower]` darker gray
  `#939799`, as the Magician draws them): `[INFO]` lines are white, although the game draws the `[INFO]` tag
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
- **File dialogs via COM** (`internal/filedialog`, formerly `folderpicker`): the modern
  Explorer dialogs (`IFileOpenDialog` in folder or multi-select mode, `IFileSaveDialog`)
  through `x/sys/windows` syscalls, run on a locked OS thread off the UI goroutine. No new dependency and no CGO; the older `SHBrowseForFolder`
  tree dialog was the alternative.
- **Session list parses every session** to show characters, duration and kinds
  (`chatlog.Summarize`), off the UI goroutine. Fine for hundreds of sessions; cache summaries
  if history grows much larger.
- **Releases come from a manual workflow, versioned at build time.** The version is typed when
  the workflow runs; CI stamps it into the exe resources (go-winres) and Settings
  (`-X main.appVersion`, in `cmd/worldkit/run.go`), so nothing is bumped by hand. Local builds
  show `dev`.
- **Per-user Inno Setup installer** (`%LocalAppData%\Programs\WorldKit`, no admin prompt),
  matching the per-user startup entry. Uninstall removes that entry but keeps `%AppData%\WorldKit`
  (chat logs are irreplaceable).
- **Auto-update is WorldKit's first and only network access.** Before it WorldKit never
  went online (the forum post says so). Players had to notice new releases and reinstall by
  hand, so most would stay on old versions. Only `internal/update` uses the network: one
  GET to `api.github.com/repos/vuenno/WorldKit/releases/latest` about once a day (with a
  `WorldKit/<version>` User-Agent, nothing else), and the release's own files from
  `github.com/vuenno/WorldKit/releases/download/` after **Update now**; any other URL in
  GitHub's answer is refused. Failures are logged with `slog` and never shown, except as
  the Settings status line. Checks can be turned off in Settings. Alternatives: a "new
  version" link only (players wouldn't look), or an update library (unneeded: GitHub's API
  plus the existing installer covers it with the standard library).
- **Updates run the normal installer silently**, not an in-place exe swap: the installer
  already knows the install folder, shortcuts, tasks and uninstall entry. WorldKit starts
  it with `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CLOSEAPPLICATIONS /RELAUNCH=1
  /WAITPID=<pid> /LOG=…\diagnostics\update-install.log` and closes its window; the script
  waits up to 30 s for that process (Restart Manager closes it otherwise), upgrades in
  place and starts the new WorldKit (a `[Run]` entry with `Check: IsRelaunch`). The
  download must match the release's listed size and the `.sha256` file the workflow
  publishes; a release without one is never installed. There is no code signing yet, so
  this guards against broken downloads, not against a compromised GitHub account.

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
- **Keep `paint.ImageOp`s, never make them in `Layout`.** Gio keys GPU textures by the op's
  handle, so a new op every frame re-uploads every frame; it also frees textures not drawn in
  a frame, so keep the pixels alive with the op (`imageVersion`).
- **Pointer positions are local** to the handler's area: in headless tests, add the area's
  window offset before queuing events (`canvasView` sits below the toolbar, right of the
  sidebar).
- **Update checks run only in release builds**: `appVersion` must parse as a version, so
  `go run` (`dev`) never checks. To try the updater locally, build with
  `-ldflags "-X main.appVersion=0.0.1"`; **Update now** then installs the real latest
  release over the per-user install.
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
- Screenshots: measure how the game blends its chat layer onto the frame from a **lossless
  in-game capture** (Arial 60 % and Mukta 75 % over a plain background) and add that last
  step if it differs from Chromium's output; other resolutions than 1080p (the vh-based
  sizes and shadow scale with screen height). Link a screenshot back to its session. Extract
  dialog: a custom color besides the swatches, remember the last look between launches (today only a default preset
  does); picture editor: double-click to select a word, Ctrl+V (Ctrl+C copies the image),
  Up/Down between rows, a blinking caret.
- A second WorldKit instance exits with "keep previous app log … used by another process"
  (diagnostics start before the single-instance check) instead of saying it's already running.
- Sidebar counts next to Logs / Log Screenshots (the design shows them).
- Cache session summaries if the session list gets slow.
- Tray icon so WorldKit can keep capturing with its window closed.
- Bring back more nav destinations when they have real content.
- Persist sidebar expanded/collapsed state and last page between launches.
- Auto-update: try it end to end once two releases with `.sha256` files exist (the
  installer's `[Code]` is only compiled by Inno Setup, which the dev machine lacks); sign
  the installer and exe and verify the signature before running a download; show download
  progress.
- Screenshot Editor: drag to reorder layers, snapping to other layers' edges, paste/copy
  images (Win32 clipboard), save and reopen work in progress (a small project JSON
  referencing the source files), crop, a **Use in editor** button on a Log Screenshot,
  any background color, editing presets in Settings, an estimated file size in the export
  dialog, per-pixel hit-testing (clicks on transparent parts of a chat image select it today).
