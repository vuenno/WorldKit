# WorldKit — Engineering Guide

WorldKit is a small **Windows-only** desktop application written in Go. This file defines how
code in this repository is written. Follow it for every change. Visual rules live in `DESIGN.md`.
**Start with `PROJECT_OVERVIEW.md`** for what exists, why, and where things are.

## Keeping the docs current (required)

Every change must leave these three files accurate. Update them **in the same change** as the
code, whenever the change is worth noting:

| File                  | Update when the change…                                                        |
|-----------------------|--------------------------------------------------------------------------------|
| `PROJECT_OVERVIEW.md` | adds/removes a feature, page or package; moves files; makes a key decision; adds a gotcha; completes or adds a next step. Bump its _Last updated_ date. |
| `DESIGN.md`           | adds or changes a color, size, spacing, typography, animation, component or logo usage. Keep it in sync with `internal/ui/theme.go`. |
| `CLAUDE.md`           | introduces a new convention, rule, command, tool, dependency or platform constraint. |

- Purely internal refactors with no visible, structural or rule change need no doc update.
- Edit only what changed. Don't rewrite sections that are still correct.
- If code and docs disagree, fix the docs in the same change; never leave them stale.

## Goals (in priority order)

1. **It works.** Every commit builds and runs: `go build ./... && go vet ./...`.
2. **It reads well.** A new contributor understands a function without scrolling.
3. **It stays small.** Prefer the standard library; add a dependency only when it saves real work.
4. **It is easy to change.** Code is reusable where reuse is proven, not speculative.

## Stack

- **Go** 1.27+ (see `go.mod`).
- **UI:** [Gio](https://gioui.org) (`gioui.org`) — immediate-mode, pure Go, no CGO needed on
  Windows. `gioui.org/widget/material` only for labels, editors and lists; buttons, controls,
  icons and the title bar are WorldKit's own (`internal/ui`).
- **Fonts:** Geist and Geist Mono, embedded from `assets/fonts` (OFL-1.1, keep `OFL.txt`
  next to them). The text shaper loads only these; never fall back to system fonts.
- `golang.org/x/image` for high-quality image resampling (`draw.CatmullRom`), the editor's
  rotated/scaled export (`draw.Transform`), its BMP/WebP decoders, and chat screenshots'
  font parsing (`font/sfnt`) and outline rasterizing (`vector`).
- `github.com/syndtr/goleveldb` (pulls in `github.com/golang/snappy`) to read FiveM's LevelDB
  key-value store, where GTA World keeps its chat log. Pure Go, read-only use, on a copy only.
- `golang.org/x/sys/windows` (already required by Gio) for the registry (start with Windows),
  exclusive file handles (single-instance archive lock) and the image clipboard. Images are
  copied as both `PNG` (keeps transparency) and `CF_DIBV5`; Gio's clipboard is text-only.
- **Platform:** Windows only (amd64). Don't add code paths, build tags or resources for other OSes/arches.
- **Network: only `internal/update`**, and only GitHub: the latest-release API for
  `vuenno/WorldKit` (about once a day, switchable in Settings) and that release's own
  assets. Network failures are logged, never shown as errors. No other network access
  (telemetry, other hosts) without discussion.
- **Never open FiveM's live data.** Copy `%AppData%\CitizenFX\kvs` to a temp folder and read the copy.
- **User data** lives in `os.UserConfigDir()\WorldKit` (`%AppData%\WorldKit`): `settings.json`,
  `exports.json`, `logs\` and `screenshots\`. Chat logs are stored raw (one `.txt` per session) and parsed on
  read; never rewrite or reformat them in place. Screenshots are PNGs WorldKit renders itself
  (`internal/screenshot`, on the CPU, no GPU); deletes only ever
  touch that folder, or a Screenshot Editor export that `exports.json` lists and that is
  unchanged since export (same size and modification time, `exports.Index.Delete`). Exports
  are written where the user picks (default `Pictures\WorldKit`) and never copied.
  Screenshot fonts are the game's chat fonts: Arial Bold and Arial Bold Italic
  (`arialbd.ttf`, `arialbi.ttf`), read from `%WINDIR%\Fonts` and drawn with DirectWrite
  (`internal/directwrite`; Go Bold / Go Bold Italic as fallback) — never bundle them (their
  license forbids redistribution) — and Mukta Regular (OFL), bundled in `assets/chatfonts`
  with its `OFL.txt`, never loaded by the UI. Chat markup (`/italic/`,
  `*action*`) is parsed once into styled characters (`screenshot.Document`); edits change
  those characters, never the stored log text.
- **Chat screenshots copy Chromium, measured.** Every number in `internal/screenshot`'s
  chat.go, gamma.go, glyphs.go and shadow.go comes from GTA World's chat stylesheet (read
  with FiveM's `nui_devtools`; dev mode: launch FiveM with `+set moo 31337`) or was fitted
  to headless Chrome drawing that CSS on a transparent page. Don't change one by eye:
  re-measure against Chrome (`chrome --headless=new --disable-gpu
  --force-device-scale-factor=1 --default-background-color=00000000 --screenshot`) and
  keep the tests' Chromium values passing.
- Write user files through `internal/atomicfile` (temp file + rename), never in place.
- **Diagnostics** go to `%AppData%\WorldKit\diagnostics\` (`internal/diagnostics`, started in
  `run.go`): `slog` output in `app.log` (previous run: `app.previous.log`) and unrecovered
  panics/fatal errors in `crash-<UTC start time>.log` via `runtime/debug.SetCrashOutput`.
  The release build has no console, so check these first when the app misbehaves or closes.
  `run` must `defer session.End(&runErr)`, never a plain `Close`, or panics lose their trace.
- No other runtime dependencies without discussion. Run one-off tools with `go run module@version`
  instead of adding them to `go.mod`.

## Project layout

```
cmd/worldkit/        main package: wiring only (create window, services, start app). No logic.
                     main.go parses flags; run.go builds the services and window.
                     rsrc_windows_amd64.syso = generated exe icon + manifest (linked
                     automatically by `go build`); never edit by hand.
internal/ui/         screens, widgets, theme. Knows about Gio; knows nothing about storage.
internal/<domain>/   business logic (pure Go, no Gio imports, fully unit-testable).
assets/              embedded files (//go:embed). logo.png (384×384, transparent) is the
                     single source of the logo, used everywhere.
```

- `main.go` stays under ~40 lines.
- Domain packages must **never** import `gioui.org/...`. UI depends on domain, never the reverse.
- Use `internal/` for everything; nothing here is a public library.

## Naming

- Names say *what*, not *how*: `visibleCountries`, not `filteredList2`.
- Booleans read as questions: `isLoading`, `hasUnsavedChanges`, `canSubmit`.
- Functions are verbs: `loadSettings`, `renderCountryCard`. Constructors: `NewX`.
- Short names only in short scopes (`i` in a 3-line loop, `gtx` for `layout.Context` — a Gio convention).
- Package names: short, lowercase, singular, no `util`/`common`/`helpers` grab-bags.
- Don't stutter: `ui.Theme`, not `ui.UITheme`.
- Exported identifiers get a doc comment starting with the identifier's name.

## Functions

- One job per function. If you need "and" to describe it, split it.
- Aim for ≤ 40 lines and ≤ 3 levels of nesting. Use early returns instead of `else`.
- ≤ 4 parameters; beyond that, pass an options struct.
- Accept interfaces, return concrete types. Define interfaces where they are *used*, keep them small.
- No naked returns. No `init()` side effects.

## Errors

- Always handle errors; never `_ =` an error without a comment saying why.
- Wrap with context: `fmt.Errorf("load settings %q: %w", path, err)`.
- Use `errors.Is` / `errors.As`, never string comparison.
- `panic` only for programmer errors that cannot happen at runtime.
- Show user-facing errors in the UI; log technical detail with `log/slog`.

## UI code (Gio)

- **State lives in structs, layout lives in methods.** Each screen is a struct holding its widget
  state (`widget.Clickable`, `widget.Editor`, …) with a `Layout(gtx layout.Context) layout.Dimensions` method.
- Widget state fields must persist between frames — never create `widget.Clickable` inside `Layout`.
- Handle input events at the top of `Layout` (`if s.saveButton.Clicked(gtx) { ... }`), then draw.
- Build reusable pieces as small functions or types in `internal/ui` (e.g. `Card`, `PrimaryButton`)
  instead of repeating layout trees.
- All colors, spacing and text sizes come from the tokens in `internal/ui/theme.go`, which mirror
  `DESIGN.md`. Follow `DESIGN.md` for every visual decision; update both together.
  No magic numbers in layouts.
- Never block the UI goroutine. Do slow work in a goroutine, then send the result back and call
  `window.Invalidate()`. For one-shot reads (disk, parsing) use `backgroundLoad` in
  `internal/ui/load.go`: `Start` in input handling, `Result` at the top of `Layout`.
- **The window has no Windows title bar** (`app.Decorated(false)`). Window buttons live in
  `titleBar`; drag areas use `moveArea` (`system.ActionInputOp(system.ActionMove)`). Never put a
  clickable inside a move area: Windows takes those clicks.
- Icons are `strokeIcon` path strings in `internal/ui/icons.go`; add new ones there.
- Windows APIs without a Go wrapper (e.g. the file dialogs, `internal/filedialog`; DirectWrite, `internal/directwrite`;
  the image clipboard, `internal/clipboard`) are called
  through COM/syscalls with `golang.org/x/sys/windows` in their own `internal/<name>` package,
  never CGO.
- Images shown by Gio: make the `paint.ImageOp` once (from an `*image.RGBA`, which Gio
  uploads without a copy) and keep it with its pixels; never `NewImageOp` in `Layout`. Cap
  textures at `previewMaxSide` (2048): an oversized texture panics in Gio's D3D11 backend.
- Several decodes at once go through `imageLoader` (`internal/ui/image_loader.go`, a small
  worker pool with a context); single reads through `backgroundLoad`.
- Animations: use `expandAnimation` (or a sibling type in `animation.go`) driven by `gtx.Now`;
  request frames with `gtx.Execute(op.InvalidateCmd{})` only while animating.
- Constraints gotcha: `layout.W`/`E` keep `Min.Y` (and `N`/`S` keep `Min.X`). Clear the other
  axis's minimum before laying out text, or labels stretch and draw off-center.

## Concurrency

- Start a goroutine only when you know how it stops (pass `context.Context`).
- Protect shared state with a mutex or channel — never both for the same data.
- Run tests with `-race` before merging concurrency changes. `-race` needs CGO and a C compiler,
  which the dev machine lacks; when it can't run, say so in the commit/PR.

## Reuse vs. duplication

- Duplicate once; extract on the third occurrence (rule of three).
- Extract by *meaning*, not by coincidental shape.
- Prefer composition (embedding small structs) over deep abstraction layers.

## Comments

- Explain *why*, not *what*. Good names make *what* obvious.
- Delete commented-out code; git remembers.
- `// TODO(name):` with a reason, or don't leave it.

## Testing

- Domain logic: table-driven tests in `_test.go` next to the code.
- Test behavior through exported APIs; avoid testing private details.
- UI: keep logic out of `Layout` so it can be tested without a window.

## Commands

```bash
go run ./cmd/worldkit                 # run the app (-minimized: start in the taskbar)
go build -ldflags="-H windowsgui" -o bin/worldkit.exe ./cmd/worldkit   # release build (no console)
go vet ./...                          # static checks
gofmt -l .                            # must print nothing
go test -race ./...                   # tests
go mod tidy                           # keep go.mod clean
```

**Releasing:** GitHub → Actions → **Release** → Run workflow, enter the version (`0.2.0`, no `v`).
It runs the checks, builds with that version, packs `installer/worldkit.iss` with Inno Setup 6
and publishes `WorldKit-Setup-<version>.exe` plus its `.sha256` as release `v<version>`
(creating the tag). Installed copies find it and update themselves (`internal/update`), so
keep the asset names, the `.sha256` file and the installer's `/WAITPID` / `/RELAUNCH`
switches working. Never hard-code the version: `main.appVersion` (`cmd/worldkit/run.go`) is
set with `-ldflags -X` and CI re-stamps the exe resources.
To try the installer locally (needs Inno Setup 6, after the release build above):

```bash
iscc /DAppVersion=0.0.0 installer/worldkit.iss
```

After changing `assets/logo.png`, regenerate the exe icon (run in `cmd/worldkit`):

```bash
go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --icon ../../assets/logo.png --manifest gui --product-name WorldKit --file-description WorldKit --product-version 0.1.0 --file-version 0.1.0
```

## Before you commit

- [ ] `gofmt`, `go vet`, `go test ./...` pass
- [ ] No unused code, no debug prints
- [ ] New names are self-explanatory without a comment
- [ ] No new dependency unless justified in the commit message
- [ ] `PROJECT_OVERVIEW.md`, `DESIGN.md` and `CLAUDE.md` updated where the change requires it
      (see "Keeping the docs current")
- [ ] Knowledge graph refreshed after code changes: `graphify update .`
