package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/adiis/criver/internal/platform"
	"github.com/adiis/criver/internal/tui"
)

func main() {
	plat := platform.Detect()
	p := tea.NewProgram(tui.NewModel(plat), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
