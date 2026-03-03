package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/adis/criver/internal/platform"
	"github.com/adis/criver/internal/tui"
)

func main() {
	plat := platform.Detect()
	p := tea.NewProgram(tui.NewModel(plat), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
