package main

import (
	"fmt"
	"os"

	"sentinelgo/config"
	"sentinelgo/tui" // New TUI package
	"sentinelgo/utils" // Logger

	"fmt"
	"os"

	"sentinelgo/config"
	"sentinelgo/tui" // New TUI package
	"sentinelgo/utils" // Logger

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev" // Default version, will be overridden by LDFLAGS

func main() {
	fmt.Printf("SentinelGo version %s\n", version) // Print version at startup

	// 1. Load Application Configuration
	// Assuming sentinel.yaml is in a 'config' directory relative to where the binary is run,
	// or in the project root 'sentinelgo/config/sentinel.yaml' during development.
	// For robustness, consider allowing config path via CLI flag or environment variable.
	appCfg, err := config.LoadAppConfig("config/sentinel.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading application config: %v. Proceeding with defaults.\n", err)
		// If LoadAppConfig returns defaults on error, this might be fine.
		// Otherwise, handle critical config errors more gracefully or exit.
		// For now, we assume LoadAppConfig provides a usable default config.
	}
    if appCfg == nil { // Ensure appCfg is not nil if LoadAppConfig can return nil on error
        fmt.Fprintf(os.Stderr, "Critical error: AppConfig is nil after loading. Exiting.\n")
        // Create a minimal default config if absolutely necessary to prevent nil pointer dereferences
        appCfg = &config.AppConfig{
            MaxRetries: 3, // Sensible default
            RiskThreshold: 75.0, // Sensible default
            DefaultHeaders: make(map[string]string),
            APIKeys: make(map[string]string),
        }
    }


	// 2. Initialize Logger
	// For TUI, logging to stdout might interfere. A log file is better.
	// Or, a custom io.Writer that sends log messages to the TUI model via tea.Cmd.
	// For now, let's use a file, and also prepare for TUI log display.
	logFile, err := os.OpenFile("sentinelgo_session.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not open log file: %v. Logging to stdout.\n", err)
		// Fallback to stdout if file logging fails
		// logger = utils.NewLogger(os.Stdout, "INFO") // Or "DEBUG" for more verbosity
	}
	// defer logFile.Close() // This defer will run when main exits.

	// In a real TUI app, you might have a logger that can also send messages
	// to the TUI's log view. For now, the TUI model has its own `logMessages` slice.
	// The global logger passed here can be used by backend components.
	var appLogger *utils.Logger
	if logFile != nil {
		appLogger = utils.NewLogger(logFile, "INFO") // Default log level
	} else {
		appLogger = utils.NewLogger(os.Stdout, "INFO")
	}

	appLogger.Info(utils.LogEntry{Message: "SentinelGo application starting..."})


	// 3. Create Initial TUI Model
	// Pass the loaded config and logger to the TUI model.
	initialModel := tui.NewInitialModel(appCfg, appLogger)

	// 4. Create and Run Bubble Tea Program
	p := tea.NewProgram(initialModel, tea.WithAltScreen()) // Use AltScreen for better TUI experience

	// Add a final log message upon clean exit or error
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()


	if _, err := p.Run(); err != nil {
		appLogger.Error(utils.LogEntry{Message: "TUI program exited with error", Error: err.Error()})
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
    appLogger.Info(utils.LogEntry{Message: "SentinelGo application exited cleanly."})
}
