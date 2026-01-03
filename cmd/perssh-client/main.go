package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/tui"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/utils"
)

func main() {
	// Initialize logger
	logger, err := utils.NewLogger("system.log", "audit.log")
	if err != nil {
		fmt.Printf("Error initializing logger: %v\n", err)
		os.Exit(1)
	}

	devMode := flag.Bool("dev", false, "Enable local mock mode for testing")
	flag.Parse()

	// Create model and set dev mode if flag is present
	model := tui.NewModel(logger)
	if *devMode {
		model.DevMode = true
	}

	// Create and start TUI
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		logger.Error("TUI Error: %v", err)
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
