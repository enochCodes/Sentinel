package main

import (
	"bufio" // For waiting for Enter key
	"fmt"
	"net/http"
	"os"

	"sentinelgo/sentinelgo/config" // Application configuration management.
	"sentinelgo/sentinelgo/tui"    // Terminal User Interface logic.
	"sentinelgo/sentinelgo/utils"  // Utility functions, including the structured logger.

	tea "github.com/charmbracelet/bubbletea" // Bubble Tea TUI framework.
)

// appLogo is the ASCII art logo displayed at application startup.
// It is defined as a multi-line string.
const appLogo = `
                                              ..:::------:::..
                                     .:-=+*##%%%###**%%*%*%%%##*+=-:.
                                .: =*#%%+#%#+%%%+++**+=*=*==#=+%%%%%%%#*=:.
                             :+#%%%**%#==***#%%%%%%%%%%%%%%##%%%*++***#%%%#+:
                         .-*%#%+*##=+*#%%%%%%**#%#*+#####%%%%%%%%%%#+*#*-=###%*-.
                       -*%%%%=***##%%%#**-+%%-+===++=++++==+-*%==#%%%%%%+%*=+%%%%*-
                    .=*#%%%%%*#%%%%++=--==+#%##%%%%%%%%%%%%##%#*#+==+##%%%%%*%%%%%#*=.
                  .+%*-+%#*=:+%%*-*+++#%%%%%%###############%%%%%%###==++#%%+:=*#%+-*%+.
                .+%%=::=+-::=%%%%+#%%%%##***
                **##############*****##%%%%++#%%%=::-+=::=%%+.
               =+#%+::-::--+%%%%%%%##***##########################***##%%%%%%%+--::-::+%#+=
             .**:=%*::-:::=====+#***##################################***#+=====:::-::*%=:**.
            -%#:::*#=:-:::::--+**########################################**+--:::::-:=#*:::#%-
           +%%+::---:-=-::+***##############################################***+::-=-:---::+%%+
          *%=*%=::::::::-+**##################################################**+-::::::::=%*=%*
         *%+:-#%=:--==+*#*####################################################*#*#*+==--:=%#-:+%*
        +%%-::-+-+#*+=-+*#*=*+**##########################################**==*##*+-=+*#+-+-::-%%+
       -%%%=::-::--:::=####++#*++*################*====*################+=--=*#####=:::--::-::=%%%-
      .%+#%%+-::::--++######*+*#*++*#############*:=#*=:##############*-::-*########++--::::-+%%#+%.
      *%-:+%%+-***+=*#########*++**++*###########*-=====############*+-==*###########*=+***-+%%+:-%*
     :%#:::=+-=+=-:=*###########*++**+++##########*+::*###########+=-=*##############*=:-=+=-+=:::#%:
     +%%-::::--::-+*##############*++**+++*#*=-============-=##*+==+*#################*+-::--::::-%%+
     %*#%+-:-:--++*****##############*+***+++--=--=:==:=--=--*+=+*#################*****++--:-:-+%#*%
    .%+:+%%+:*#*=:=--=======+++=+++++==--++===---:------:=--=====++==+++++=+++=======--=:=*#*:+%%+:+%.
    -%*::-+=-*=::-+*--++++++++=-=+++==+**=-=------------------=-=+*+==+++=-=++++++++--*+-::=*-=+-::*%:
    -%#-::::::::-#*##--=++++++++=+++==*##===--------==--:-----===##*==+++=++++++++=--##*#-::::::::-#%-
    -%# #+-:::-=-=%*###*+--+++++++==+++++*==-----:-======-:-----==*+++++==+++++++--+*###*%=-=-:::-+#%%-
    :*#%%%%--#+::**#####*==-=++++++===+=+==--------====--------==+=+===++++++=-==*#####**::+#--%%%%#*:
    .#--=*#=-=:::#*########+==-===+==++=====------------------=====++==+===-==+########*#:::=-=#*=--#.
     #*:::---:::+***##########**++++--*++*==------------------==*+++--++++**##########***+:::---:::*#
     =%*=-::::-*#--+#################=::-==-=--=-:=----=:-=--=-==-::+#################+--#*-::::-=*%=
     .%#####*:*%-::+*################*:::-==+++=-==:==:==-==++==:::-#################*+::-%*:*#####%.
      +%--+*#=--:::+**###############*-:--:+###**+=-=--++**++**+=-.-+###############**+:::--=#*+--%+
       ##-:::-::::+=-+*#############*==++--=+*####+:+..+####*==+*+=-=##############*+-=+::::-:::-##
       :%%+--::::*#::-**#########*+==+*##*=-----=-:=+.---===---==+=-:-+###########**-::#*::::--+%%:
        -%######-+=:::*#*######*+==*#######+=-----=**.:-=====+**+*+-:::-+########*#*:::=+-######%-
         =%=--===:--:--=***##*+=+*###########**+=--==..:=+**########*=-::-++*##***=--:--:===--=%=
          =%+-::::::-#=::+#*==+*#################**=::+*##############*=:=#*+#*#+::=#-::::::-+%=
           -%%*+++*+-*=:::=+=====*##################**##################==+****+:::=*-+*+++*%%-
            :#%%##*+-:-::-::-----:=###################################*-=-=-=-::::::-*%%%%%%#.
            ::---:::-------::::::::-############**********############-.::::::--------:---==
            .:-::::::--::::::------::-+***++==--------------==++***+-:.:----::::::::-----:-:
             .:-====++----------::::::::--:::::::--::::::-::::::--::::::::--------------:-:
               =######+-:::::-----------::::::---::::=::::---:::::------------:::::=#****:
                :+***++==++++*=:-:::::---==+++**+:--=-==-:-**++==---:::::-:-*++++==#####-
                  ::::::-*####*---==++**########+::-=-==::-#######**++==--=*#####+-=+++.
                    ..:::-======*################=:---:-::+**############*=+**##+::::.
                          .:::::+########***+==---::::::::::---==++***###=:::--:.
                            .....:-====---:::::::::::::::::::::::::::---::::::.
                                   .::::::::::...         ...::::::::::
                                     ....                         ....
`

// version is the application's version string.
// It defaults to "dev" and can be overridden during the build process using ldflags
// (e.g., `go build -ldflags="-X main.version=1.0.0"`).
var version = "dev"

// main is the entry point for the SentinelGo application.
// It handles initial setup including:
// - Displaying an ASCII art logo and version information.
// - Loading application configuration from `config/sentinel.yaml`.
// - Initializing a structured logger (output to `sentinelgo_session.log`).
// - Creating the initial model for the Terminal User Interface (TUI).
// - Starting and running the Bubble Tea TUI program.
// It exits with status 1 if TUI initialization or execution fails.
func main() {
	// Initial splash screen: Clear screen, print logo, version, and wait for Enter.
	fmt.Print("[H[2J") // ANSI escape sequence to clear the terminal screen.
	fmt.Println(appLogo)
	fmt.Printf("SentinelGo version %s\n", version)
	fmt.Print("\nPress Enter to continue...")
	_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n') // Wait for Enter key press.
	fmt.Print("[H[2J")                             // Clear screen again before starting the TUI.

	// 1. Load Application Configuration
	// Attempts to load from "config/sentinel.yaml".
	// If loading fails or file not found, proceeds with default values defined in config.LoadAppConfig.
	appCfg, err := config.LoadAppConfig("config/sentinel.yaml")
	if err != nil {
		// Log to Stderr as the main logger might not be set up or might be file-based.
		fmt.Fprintf(os.Stderr, "Warning: Error loading application config: %v. Proceeding with defaults.\n", err)
	}
	if appCfg == nil { // Should only happen if LoadAppConfig has a bug and returns nil, nil
		fmt.Fprintf(os.Stderr, "Critical error: AppConfig is nil after attempting to load. Using minimal fallback defaults.\n")
		appCfg = &config.AppConfig{ // Provide minimal essential defaults.
			MaxRetries:     3,
			RiskThreshold:  75.0,
			DefaultHeaders: make(map[string]string),
			APIKeys:        make(map[string]string),
			CustomCookies:  []http.Cookie{},
		}
	}

	// 2. Initialize Logger
	// Logs to "sentinelgo_session.log". Falls back to Stderr if file cannot be opened.
	logFile, logFileErr := os.OpenFile("sentinelgo_session.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	var appLogger *utils.Logger
	if logFileErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not open log file 'sentinelgo_session.log': %v. Logging to Stderr for this session.\n", logFileErr)
		appLogger = utils.NewLogger(os.Stderr, "INFO") // Default to INFO level for Stderr fallback.
	} else {
		appLogger = utils.NewLogger(logFile, "INFO") // Default to INFO level for file logger.
		defer func() {                               // Ensure log file is closed on exit if successfully opened.
			appLogger.Info(utils.LogEntry{Message: "SentinelGo application shutting down. Closing log file."})
			logFile.Close()
		}()
	}

	appLogger.Info(utils.LogEntry{Message: "SentinelGo application TUI starting..."})

	// 3. Create Initial TUI Model
	// The TUI model is initialized with the loaded (or default) application configuration and the logger.
	initialModel := tui.NewInitialModel(appCfg, appLogger)

	// 4. Create and Run Bubble Tea Program
	// Uses tea.WithAltScreen() to enable alternate screen buffer for a cleaner TUI experience.
	program := tea.NewProgram(initialModel, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		// Log fatal error from TUI and exit.
		appLogger.Error(utils.LogEntry{Message: "TUI program exited with error", Error: err.Error()})
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1) // Exit with error code.
	}
	appLogger.Info(utils.LogEntry{Message: "SentinelGo TUI exited cleanly."})
}
