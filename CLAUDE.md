# WorldKit — Engineering Guide

WorldKit is a small desktop application written in Go. This file defines how code in
this repository is written. Follow it for every change.

## Goals (in priority order)

1. **It works.** Every commit builds and runs: `go build ./... && go vet ./...`.
2. **It reads well.** A new contributor understands a function without scrolling.
3. **It stays small.** Prefer the standard library; add a dependency only when it saves real work.
4. **It is easy to change.** Code is reusable where reuse is proven, not speculative.

## Stack

- **Go** 1.27+ (see `go.mod`).
- **UI:** [Gio](https://gioui.org) (`gioui.org`) — immediate-mode, pure Go, no CGO needed on
  Windows, Material-style widgets via `gioui.org/widget/material`.
- No other runtime dependencies without discussion.

## Project layout

```
cmd/worldkit/        main package: wiring only (create window, start app). No logic.
internal/ui/         screens, widgets, theme. Knows about Gio; knows nothing about storage.
internal/<domain>/   business logic (pure Go, no Gio imports, fully unit-testable).
assets/              icons, fonts, embedded files (use //go:embed).
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
  `window.Invalidate()`.
- Animations: use `expandAnimation` (or a sibling type in `animation.go`) driven by `gtx.Now`;
  request frames with `gtx.Execute(op.InvalidateCmd{})` only while animating.
- Constraints gotcha: `layout.W`/`E` keep `Min.Y` (and `N`/`S` keep `Min.X`). Clear the other
  axis's minimum before laying out text, or labels stretch and draw off-center.

## Concurrency

- Start a goroutine only when you know how it stops (pass `context.Context`).
- Protect shared state with a mutex or channel — never both for the same data.
- Run tests with `-race` before merging concurrency changes.

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
go run ./cmd/worldkit                 # run the app
go build -ldflags="-H windowsgui" -o bin/worldkit.exe ./cmd/worldkit   # release build (no console)
go vet ./...                          # static checks
gofmt -l .                            # must print nothing
go test -race ./...                   # tests
go mod tidy                           # keep go.mod clean
```

## Before you commit

- [ ] `gofmt`, `go vet`, `go test ./...` pass
- [ ] No unused code, no debug prints
- [ ] New names are self-explanatory without a comment
- [ ] No new dependency unless justified in the commit message
