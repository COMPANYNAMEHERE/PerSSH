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

	os.WriteFile("perssh_alive.txt", []byte("I AM ALIVE"), 0644)



	// Initialize logger
	logger, err := utils.NewLogger("system.log", "audit.log")
	if err != nil {
		fmt.Printf("Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	os.WriteFile("perssh_alive_2.txt", []byte("LOGGER OK"), 0644)
	logger.System("Application started")

	devMode := flag.Bool("dev", false, "Enable local mock mode for testing")
	flag.Parse()

	// Create model and set dev mode if flag is present
	model := tui.NewModel(logger)
	if *devMode {
		model.DevMode = true
	}
	os.WriteFile("perssh_alive_3.txt", []byte("MODEL OK"), 0644)

	// Create and start TUI
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		os.WriteFile("perssh_alive_error.txt", []byte(err.Error()), 0644)
		logger.Error("TUI Error: %v", err)
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
	os.WriteFile("perssh_alive_4.txt", []byte("DONE"), 0644)
}
