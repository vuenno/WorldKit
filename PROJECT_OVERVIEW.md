# WorldKit — Project Overview

Context for agents and new contributors: what exists, why it is built this way, and where things
are. **Read this first, then `CLAUDE.md` (code rules) and `DESIGN.md` (visual rules).**
Keep this file current: update it in the same change whenever features, structure or key
decisions change.

_Last updated: 2026-10-18_

## What it is

WorldKit is a small, **Windows-only** (amd64) desktop application written in Go with the Gio UI
toolkit. Its visual identity is a dark UI with a single orange accent and the "WK WorldKit"
logo (sunset city skyline). The product's actual features are **not defined yet**: the
current build is the application shell that future features plug into.

## Current state

| Area             | Status                                                                 |
|------------------|------------------------------------------------------------------------|
| App shell        | Done: window, dark background, sidebar + page area                     |
| Sidebar          | Done: collapsible (200 ↔ 58 dp), 200 ms ease-in-out animation, hover/active states |
| Navigation       | Done: Home, Explore, Map, Library, Settings (switch the page title)     |
| Home page        | Done: centered logo hero + welcome text                                |
| Other pages      | Placeholders ("This page is empty for now.")                           |
| Branding         | Done: logo in sidebar header, Home hero, exe/title bar/taskbar icon    |
| Domain logic     | None yet (`internal/<domain>/` packages will hold it)                  |
| Persistence      | None yet                                                               |
| Tests            | Unit tests for the animation (`internal/ui/animation_test.go`)        |

## Architecture

```
main()                                  cmd/worldkit/main.go
 └─ app.Window ─► ui.Run(window)        internal/ui/app.go   event loop: one Shell.Layout per frame
     └─ Shell                           internal/ui/app.go   sidebar + selected page
         ├─ Sidebar                     internal/ui/sidebar.go
         │   ├─ header: toggle button + brand (Logo + "WorldKit"), fades when collapsing
         │   ├─ NavItem × N  ──► navRow (shared icon + label row renderer)
         │   └─ expandAnimation         internal/ui/animation.go
         └─ page: layoutHomePage | layoutPlaceholderPage
Logo                                    internal/ui/logo.go  resamples per pixel size, cached
theme tokens (colors, sizes, timing)    internal/ui/theme.go mirrors DESIGN.md
assets.Logo()                           assets/assets.go     embedded logo.png, decoded once
```

Gio is immediate-mode: every frame, `Layout` methods handle input first, then draw. Widget state
(`widget.Clickable`, animation state, selection) lives in structs that persist across frames.

## File map

| Path                                   | Purpose                                                     |
|----------------------------------------|-------------------------------------------------------------|
| `cmd/worldkit/main.go`                 | Creates the window (960×600, min 640×400), starts `ui.Run`  |
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

- Define WorldKit's real features and give each nav destination real content.
- Primary/secondary button components (spec in `DESIGN.md`; `colorPrimaryHover` reserved for them).
- Persist sidebar expanded/collapsed state and last page between launches.
