# SentinelGo++ Cyber Operations Platform

```text

                                              ..:::------:::..
                                     .:-=+*##%%%###**%%*%*%%%##*+=-:.
                                .: =*#%%+#%#+%%%+++**+=*=*==#=+%%%%%%%#*=:.
                             :+#%%%**%#==***#%%%%%%%%%%%%%%##%%%*++***#%%%#+:
                         .-*%#%+*##=+*#%%%%%%**#%#*+#####%%%%%%%%%%#+*#*-=###%*-.
                       -*%%%%=***##%%%#**-+%%-+===++=++++==+-*%==#%%%%%%+%*=+%%%%*-
                    .=*#%%%%%*#%%%%++=--==+#%##%%%%%%%%%%%%##%#*#+==+##%%%%%*%%%%%#*=.
                  .+%*-+%#*=:+%%*-*+++#%%%%%%###############%%%%%%###==++#%%+:=*#%+-*%+.
                .+%%=::=+-::=%%%%+#%%%%##*****##############*****##%%%%++#%%%=::-+=::=%%+.
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
```

## Overview

SentinelGo++ is an interactive, modular, AI-ready terminal-based tool designed for cybersecurity operations. It specializes in monitoring and facilitating the reporting of content on various platforms, with an initial focus on services like TikTok. The application emphasizes a professional user experience through a keyboard-driven Terminal User Interface (TUI) and is built for extensibility.

## Core Features

*   **Interactive Terminal UI (TUI):** Built with Bubble Tea, providing a responsive, keyboard-driven interface.
*   **Tabbed Layout:**
    *   **Target Input:** Specify target URL and number of reports.
    *   **Proxy Management:** View proxy pool status (loaded, healthy, unhealthy, unknown).
    *   **Settings:** View and edit application settings (e.g., Max Retries, AI Risk Threshold). Changes can be saved to `config/sentinel.yaml`.
    *   **Live Session Logs:** Real-time, styled log output for ongoing reporting sessions.
    *   **Log Review + Export:** (Placeholder) Future home for reviewing and exporting detailed logs.
*   **Session Control:** Start, pause, resume, and abort reporting sessions directly from the TUI.
*   **Advanced Proxy Management:**
    *   Load proxies from CSV or JSON files.
    *   Automatic proxy health checks (initial checks run in the background).
    *   Proxy rotation strategies (currently round-robin, random).
*   **Configuration:**
    *   Centralized `config/sentinel.yaml` for persistent application settings.
    *   Key settings like `MaxRetries` and `RiskThreshold` are editable via the "Settings" tab in the TUI.
*   **Secure Structured Logging:** Session activities and important events are logged in JSON lines format to `sentinelgo_session.log`. TUI also provides a live, styled view of session events.
*   **AI Module Hook:** Features a pluggable `ContentAnalyzer` interface. A `DummyAnalyzer` is currently implemented for placeholder AI content assessment, checking against a configurable `RiskThreshold`.
*   **Customizable Reporting:** Users can specify the target URL and the number of reports to send for each session.

## Installation

### From Source

Requires Go 1.20+

```sh
git clone https://github.com/your-username/sentinelgo.git # Replace with actual URL when available
cd sentinelgo/sentinelgo
# (Note: if you cloned into 'sentinelgo' and the module is also 'sentinelgo', cd into the inner 'sentinelgo' or adjust paths)
go install ./cmd/sentinelgo
```
This will install `sentinelgo` to your `$GOPATH/bin` or `$GOBIN` (if set). Ensure this directory is in your system's `PATH`.

### Using Makefile

```sh
git clone https://github.com/your-username/sentinelgo.git # Replace with actual URL
cd sentinelgo/sentinelgo
make build  # Creates executable in ./build/
# OR
make install # Installs to $GOPATH/bin or $GOBIN
```

## Basic Usage

1.  Run the application (after installation):
    ```sh
    sentinelgo
    ```
2.  The application will first display a splash screen with the SentinelGo++ logo. Press `Enter` to continue to the TUI.
3.  **TUI Navigation:**
    *   `Ctrl+N`: Navigate to the next tab.
    *   `Ctrl+P`: Navigate to the previous tab.
    *   `Arrow Keys (Up/Down)`: Navigate lists or selectable items within a tab (e.g., in Settings).
    *   `Enter`:
        *   On "Target Input" tab: Submits the target URL and number of reports to start a new session.
        *   On "Settings" tab: Activates edit mode for the selected setting, or confirms an edit.
    *   `Tab`: Switch focus between input fields (e.g., in "Target Input" tab).
    *   `Esc`: Cancel current edit (e.g., in Settings tab).
    *   `Ctrl+C` or `q` (in non-input contexts): Quit the application.
    *   Session specific: `P` to Pause, `R` to Resume, `A` to Abort an active session (when focus is not on an input field).

## Configuration

SentinelGo++ uses a `config/sentinel.yaml` file for its core configuration. Key settings include:

*   `maxretries`: Default number of times the reporter will retry sending a single report if it fails.
*   `riskthreshold`: A percentage (0-100) used by the AI module. Content scoring above this threshold might trigger specific actions or logs.
*   `defaultheaders`: A map of HTTP headers (e.g., `User-Agent`) to be used for outgoing requests.
*   `customcookies`: A list of cookies (name, value, path, domain) to be added to requests.
*   `apikeys`: A map for storing API keys for various services (e.g., `virustotal`).

Some of these settings (`MaxRetries`, `RiskThreshold`) can be viewed and edited live from the "Settings" tab within the TUI. Changes can be saved back to `config/sentinel.yaml` using `Ctrl+S` on that tab. `Ctrl+R` reloads the configuration from the file.

The application also uses `config/proxies.csv` (or a JSON equivalent) by default to load proxies, though this path might be configurable in future versions or via `sentinel.yaml`.

## Build Instructions

The provided `Makefile` (in the `sentinelgo/` directory) simplifies common development tasks:

*   `make build`: Compiles the application and places the executable in `sentinelgo/build/sentinelgo`. Includes version information via ldflags.
*   `make install`: Installs the application to your Go binary path.
*   `make test`: Runs all unit tests in the project.
*   `make clean`: Removes build artifacts.
*   Cross-compilation: The Makefile includes placeholder targets for building for different operating systems:
    *   `make build-linux`
    *   `make build-windows`
    *   `make build-mac`

## Contributing

Contributions are welcome! Please refer to `CONTRIBUTING.md` (to be created) for guidelines on how to contribute to the SentinelGo++ project.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
