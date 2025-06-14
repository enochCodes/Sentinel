# SentinelGo++ Architecture

This document provides an overview of the SentinelGo++ project structure and the roles of its main components.

## Directory Structure Overview

```
sentinelgo/
├── cmd/sentinelgo/   # Main application entry point (main.go) & initial setup.
├── tui/              # Terminal User Interface logic (Bubble Tea model, views, styles).
├── report/           # Core reporting engine (sending reports, retry logic).
├── proxy/            # Proxy management (loading, health checks, rotation strategies).
├── config/           # Configuration loading (sentinel.yaml) and state management (state.json).
├── session/          # Session management (controls report execution flow, state).
├── ai/               # AI module integration (analyzer interface, dummy analyzer).
├── utils/            # Utility functions (structured logger, etc.).
├── assets/           # Static assets (e.g., placeholder for future themes, images if any).
├── docs/             # Project documentation.
# Note: The root main.go was part of an earlier structure and has been consolidated into cmd/sentinelgo/main.go
├── go.mod            # Go module definition.
├── go.sum            # Go module checksums.
├── Makefile          # Build, test, install scripts.
├── README.md         # Main project README.
└── LICENSE           # Project license.
```

## Core Modules

### 1. `cmd/sentinelgo`
*   **Responsibility:** Application startup, command-line argument parsing (if any), initialization of global components (logger, config), and launching the TUI.
*   **Key files:** `main.go`

### 2. `config`
*   **Responsibility:** Loading application configuration from `sentinel.yaml`, managing application state (e.g., `~/.sentinel/state.json`), and providing access to configuration values. Also handles saving configuration.
*   **Key files:** `config.go`

### 3. `tui`
*   **Responsibility:** Managing the terminal user interface using the Bubble Tea library. Handles user input, displays information, and orchestrates interaction with backend components.
*   **Key files:** `model.go` (main TUI model, update, view), `styles.go` (lipgloss styling).

### 4. `proxy`
*   **Responsibility:** Loading proxies from various sources (CSV, JSON), performing health checks, and implementing proxy rotation strategies.
*   **Key files:** `loader.go`, `health.go`, `strategy.go`

### 5. `report`
*   **Responsibility:** Sending individual report requests to the target URL. Handles HTTP communication, retries, and integration with the AI analyzer.
*   **Key files:** `reporter.go`

### 6. `session`
*   **Responsibility:** Managing a reporting session, which involves sending a specified number of reports to a target URL. Controls the flow (start, pause, resume, abort) and tracks progress.
*   **Key files:** `session.go`

### 7. `ai`
*   **Responsibility:** Provides an interface for content analysis. Includes a dummy analyzer for placeholder functionality, allowing for future integration of actual AI models.
*   **Key files:** `analyzer.go`

### 8. `utils`
*   **Responsibility:** Contains shared utility functions, most notably the structured JSON logger.
*   **Key files:** `logger.go`

## Data Flow (Simplified Example: Starting a Session)

1.  User inputs Target URL and Number of Reports in **TUI** (`tui/model.go`).
2.  TUI validates input and on submission, creates a new **Session** (`session/session.go`) instance, passing it a **Reporter** instance.
3.  The **Session** manager's `Start()` method is called, launching its `runLoop()` in a goroutine.
4.  For each report to be sent (up to "Number of Reports"):
    a.  **Session**'s `runLoop` logs intent to send "Report X of N".
    b.  It calls `s.Reporter.SendReport(s.TargetURL, s.ID)`.
5.  Inside `Reporter.SendReport()`:
    a.  A proxy is requested from the **ProxyManager** (`proxy/strategy.go`).
    b.  An HTTP request is constructed (currently POST with nil body). Headers and cookies from **AppConfig** (`config/config.go`) are applied.
    c.  The request is sent. Retries are handled internally by `SendReport` up to `AppConfig.MaxRetries`.
    d.  If successful and an **AIAnalyzer** (`ai/analyzer.go`) is configured, the response content (simulated for now) is passed to `AIAnalyzer.Analyze()`.
    e.  The outcome (success/failure, AI results) is logged using the **Logger** (`utils/logger.go`).
6.  The **Session** updates its internal counters (successful/failed reports) based on the error returned by `Reporter.SendReport()`.
7.  The **Session** sends status updates (e.g., "Report X of N success/failure") to the **TUI** via its `LogChannel`.
8.  The **TUI** receives these updates and displays them in the "Live Session Logs" tab.
9.  All components access shared configuration settings via the `AppConfig` struct, which is initially loaded by `cmd/sentinelgo/main.go` and passed down.

*(This is a high-level overview and can be expanded with more diagrams and details regarding specific interactions, error handling, and data persistence.)*
