package main

import (
	"log/slog"
	"os"

	"gioui.org/app"
	"gioui.org/unit"

	"world-kit/internal/ui"
)

func main() {
	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("WorldKit"),
			app.Size(unit.Dp(960), unit.Dp(600)),
			app.MinSize(unit.Dp(640), unit.Dp(400)),
		)
		if err := ui.Run(window); err != nil {
			slog.Error("application stopped", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}
