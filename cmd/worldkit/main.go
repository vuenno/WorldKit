package main

import (
	"flag"
	"log/slog"
	"os"

	"gioui.org/app"

	"world-kit/internal/autostart"
)

func main() {
	isMinimized := flag.Bool(autostart.MinimizedFlag, false, "start minimized (used when starting with Windows)")
	flag.Parse()
	go func() {
		if err := run(*isMinimized); err != nil {
			slog.Error("application stopped", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}
