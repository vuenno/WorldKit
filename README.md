<p align="center">
  <img src="assets/logo.png" alt="WorldKit logo" width="160">
</p>

<h1 align="center">WorldKit</h1>

<p align="center">
  A desktop companion for <b>GTA World</b> roleplayers: keep every chat log, turn it into
  in-game-looking chat screenshots, and compose them over your game shots.
</p>

<p align="center">
  <a href="https://github.com/vuenno/WorldKit/releases/latest"><img src="https://img.shields.io/github/v/release/vuenno/WorldKit?label=release&color=FF7A1A" alt="Latest release"></a>
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" alt="Go 1.27">
  <img src="https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078D4" alt="Windows 10 | 11">
</p>

---

GTA World throws its chat away every time you reconnect. WorldKit saves it in the
background, lets you search and filter it, and turns any lines into transparent chat images
drawn exactly like the in-game chat, ready to lay over a screenshot.

<p align="center">
  <img src="docs/screenshots/scene-bar.png" alt="RP scene made with WorldKit: a bar with chat over it" width="420">
  <img src="docs/screenshots/scene-envelope.png" alt="RP scene made with WorldKit: a quiet deal at a table" width="420"><br>
  <sub>Two RP scenes made entirely in WorldKit: chat images (Arial, 60 %) laid over game screenshots on a 550 × 550 canvas.</sub>
</p>

## Features

### Chat logs, saved automatically

WorldKit watches FiveM while you play and saves each session's chat to a plain text file.
It reads a copy of FiveM's data, never the live files. Sessions are grouped by day, with
characters, duration and line counts at a glance.

![Logs page](docs/screenshots/logs.png)

### A log viewer built for RP

Find text, filter by kind (IC, /me, /do, /b, PMs) or by character, toggle timestamps, and
select lines to copy or extract.

![Log viewer](docs/screenshots/log-viewer.png)

### Chat screenshots that look like the game

**Extract image** turns selected lines into a transparent PNG drawn the way the in-game
chat is: same font (**Arial** or **Mukta**), same **chat scale**, same colors and soft
shadow, wrapped at the same width. `/italic/` and `*actions*` markup work as you type them
in game. Save your look as a preset.

![Extract image dialog](docs/screenshots/extract-dialog.png)

### Edit the text right on the image

Click into the picture and type, select words and make them italic or recolor them with
the chat colors, drag lines into a new order, or merge lines one person said. Ctrl+Z undoes
any of it.

![Editing a chat image: typed words, italics and a recolored phrase](docs/screenshots/extract-edit.png)

### Match your in-game chat

Switch the font and chat scale to what your game uses, cap the width, and add a dark or
light box behind each line when you need it. The preview redraws as you go.

![The same lines in Mukta at 75 % with a dark backdrop](docs/screenshots/extract-look.png)

<p align="center">
  <img src="docs/screenshots/chat-arial.png" alt="Chat image in Arial"><br>
  <sub>Arial, 100 %</sub>
</p>
<p align="center">
  <img src="docs/screenshots/chat-mukta.png" alt="Chat image in Mukta"><br>
  <sub>Mukta, 100 %</sub>
</p>

### Build whole RP scenes

The Screenshot Editor puts it together: pick a canvas (templates or any custom size), add
game screenshots and chat images as layers, then move, resize, rotate, fade and reorder
them freely. Export PNG or JPEG at the quality you choose; every export is listed on the
Saved Screenshots page.

![Screenshot Editor composing a 550 × 550 bar scene](docs/screenshots/editor.png)

## Install

1. Download `WorldKit-Setup-<version>.exe` from the
   [latest release](https://github.com/vuenno/WorldKit/releases/latest).
2. Run it, open WorldKit, and point it at your FiveM data folder if it doesn't find it.
3. Play. Your chat is saved as you go.

WorldKit runs on Windows 10 and 11 (64-bit) and can start with Windows. Your logs,
screenshots and settings stay on your PC in `%AppData%\WorldKit`.

## Build from source

Requires Go 1.27+ on Windows. No CGO.

```bash
go run ./cmd/worldkit
go build -ldflags="-H windowsgui" -o bin/worldkit.exe ./cmd/worldkit
go test ./...
```

New to the code? Start with [`PROJECT_OVERVIEW.md`](PROJECT_OVERVIEW.md), then
[`CLAUDE.md`](CLAUDE.md) (code rules) and [`DESIGN.md`](DESIGN.md) (visual rules).

## Credits

- UI built with [Gio](https://gioui.org).
- Fonts: [Geist](https://vercel.com/font) (UI) and [Mukta](https://fonts.google.com/specimen/Mukta)
  (chat images), both under the SIL Open Font License. Arial is read from Windows, never
  bundled.

WorldKit is a fan-made tool. It is not affiliated with GTA World, Cfx.re/FiveM or Rockstar
Games.
