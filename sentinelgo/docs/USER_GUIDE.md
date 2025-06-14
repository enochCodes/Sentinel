# SentinelGo++ User Guide

Welcome to the SentinelGo++ User Guide. This guide will help you understand how to install, configure, and use the application.

## Table of Contents

*   [Installation](#installation)
*   [Configuration](#configuration)
*   [Running SentinelGo++](#running-sentinelgo)
*   [Navigating the Terminal User Interface (TUI)](#navigating-the-terminal-user-interface-tui)
    *   [Global Keybindings](#global-keybindings)
    *   [Splash Screen](#splash-screen)
    *   [Main Tabs Overview](#main-tabs-overview)
*   [Using Core Features](#using-core-features)
    *   [Target Input Tab](#target-input-tab)
    *   [Live Session Logs Tab](#live-session-logs-tab)
    *   [Proxy Management Tab](#proxy-management-tab)
    *   [Settings Tab (Editable)](#settings-tab-editable)
    *   [Log Review + Export Tab](#log-review--export-tab)
*   [Understanding Proxies](#understanding-proxies) (Placeholder)
*   [Troubleshooting](#troubleshooting) (Placeholder)
*   [Advanced Usage](#advanced-usage) (Placeholder)

## Installation
*(Refer to the main [README.md](../README.md#installation) for detailed installation instructions.)*

## Configuration
*(Refer to the [CONFIGURATION.md](./CONFIGURATION.md) for details on all settings in `config/sentinel.yaml`.)*

## Running SentinelGo++
Once installed (or built using `make build`), you can run the application from the `sentinelgo` directory:
*   If installed to your PATH: `sentinelgo`
*   If built with `make build`: `./build/sentinelgo`

This will launch the Terminal User Interface.

## Navigating the Terminal User Interface (TUI)

### Global Keybindings
*   **Ctrl+C**: Quit the application from (almost) any screen. If a session is active, it might prompt to abort first or require a second Ctrl+C.
*   **q**: Quit the application (usually when not actively typing in an input field or when a session is not running).
*   **Ctrl+N**: Navigate to the Next Tab (cycles through tabs).
*   **Ctrl+P**: Navigate to the Previous Tab (cycles through tabs).
*   *(Context-specific keybindings are displayed in the footer area of the TUI.)*

### Splash Screen
Upon startup, a large ASCII art logo and version information are displayed. Press `Enter` to continue to the main interface.

### Main Tabs Overview
The TUI is organized into several tabs for different functionalities:

*   **Target Input**: Specify the target URL and the number of reports you wish to send for a new reporting session.
*   **Proxy Management**: View the status of your loaded proxy pool (total, healthy, unhealthy, unknown).
*   **Settings**: View and edit application settings like `Max Retries` and `AI Risk Threshold`. Changes can be saved to `config/sentinel.yaml`.
*   **Live Session Logs**: Monitor real-time, styled log messages from active reporting sessions, including progress and outcomes.
*   **Log Review + Export**: (Placeholder) Intended for future features to review past session logs from the `sentinelgo_session.log` file and export them.

## Using Core Features

### Target Input Tab
1.  **Focus**: This tab usually opens by default. The `>` symbol or a highlighted border indicates the active input field.
2.  **Target URL**: Type or paste the full URL for the report request. This is the endpoint that will receive the POST requests.
3.  **Number of Reports**: Enter the total number of times you want the report to be sent. Only numeric digits are accepted here.
4.  **Switch Input Fields**: Press `Tab` to switch focus between "Target URL" and "Number of Reports".
5.  **Submit**: With both fields filled appropriately, press `Enter` to start a new reporting session.
    *   The system will validate inputs (URL not empty, Number of Reports > 0). Errors will be shown in the footer.
    *   If valid, a new session starts, and you'll see updates in the "Live Session Logs" tab and the session status bar.
    *   The Target URL field will be cleared after submission. "Number of Reports" defaults to "1".

### Live Session Logs Tab
*   Displays real-time status updates from any ongoing reporting session.
*   Messages are prefixed with a timestamp and log level (e.g., `[INF]`, `[ERR]`), and styled with colors for readability.
*   You can monitor the progress of reports being sent (e.g., "Report X of N -> Sending..."), successes, and failures.
*   **Session Controls (when a session is active and this tab is not focused on an input):**
    *   `P`: Pause the current reporting session (pauses between report sends).
    *   `R`: Resume a paused session.
    *   `A`: Abort (cancel) the current session. The session will attempt to stop gracefully.

### Proxy Management Tab
*   Provides an overview of your proxy pool:
    *   **Total Proxies**: Number of proxies loaded from your source file (e.g., `config/proxies.csv`).
    *   **Healthy**: Number of proxies currently marked as "healthy" by health checks.
    *   **Unhealthy**: Number of proxies marked as "unhealthy".
    *   **Unknown**: Number of proxies whose health status is not yet determined or has expired.
*   An informational message indicates that initial health checks run in the background.
*   *(Future enhancements: list individual proxies, trigger manual health checks, import/export proxy lists.)*

### Settings Tab (Editable)
1.  **Navigation**: Use `Arrow Up` and `Arrow Down` keys to highlight different settings. The selected setting is prefixed with `▸`.
2.  **Edit**: Press `Enter` on a highlighted setting to enter "edit mode". The current value will appear in an input field with a `_` cursor.
3.  **Modify Value**: Type the new value. For numeric fields like "Max Retries", only digits will be accepted. For "Risk Threshold", numbers and a decimal point.
4.  **Confirm Edit**: Press `Enter` again. The input will be validated:
    *   If valid, the setting is updated in the application's current memory. A log message confirms the local update and reminds you to save.
    *   If invalid (e.g., non-numeric for "Max Retries"), an error message appears in the footer.
5.  **Cancel Edit**: Press `Esc` while in edit mode to discard changes and revert to the setting's previous value.
6.  **Save Settings**: Press `Ctrl+S` to save all current in-memory setting changes to the `config/sentinel.yaml` file. A confirmation or error message will be logged.
7.  **Reload Settings**: Press `Ctrl+R` to discard any unsaved in-memory changes and reload all settings from `config/sentinel.yaml`. The view will update to reflect the loaded values.

### Log Review + Export Tab
*   This section is currently a placeholder for future development.
*   All detailed, structured session logs are automatically saved in JSON lines format to the `sentinelgo_session.log` file in the directory where the application is run. This file can be reviewed manually or processed by other tools.

## Understanding Proxies
*(Placeholder: This section will explain the importance of using proxies for anonymity and avoiding rate limits, types of proxies, and tips for sourcing good proxy lists.)*

## Troubleshooting
*(Placeholder: This section will list common issues, such_as proxy errors, configuration problems, or TUI display glitches, along with potential solutions.)*

## Advanced Usage
*(Placeholder: This section might cover topics like advanced configuration not exposed in the TUI, interpreting structured logs, or potential for scripting interactions if the tool evolves to support CLI operations alongside the TUI.)*
