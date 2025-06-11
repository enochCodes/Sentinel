package tui

import (
	"fmt"
	"sentinelgo/sentinelgo/ai" // Import AI package
	"sentinelgo/sentinelgo/config"
	"sentinelgo/sentinelgo/proxy"   // Import proxy package
	"sentinelgo/sentinelgo/report"  // Import report package
	"sentinelgo/sentinelgo/session" // Import session package
	"sentinelgo/sentinelgo/utils"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Tab defines the different views/tabs in the TUI.
type Tab int

const (
	TargetInputTab Tab = iota
	ProxyMgmtTab
	SettingsTab
	LiveSessionLogsTab
	LogReviewTab
	numTabs // Keep this last to count number of tabs
)

var tabNames = []string{
	"Target Input",
	"Proxy Management",
	"Settings",
	"Live Session Logs",
	"Log Review & Export",
}

// sessionLogMsg is a tea.Msg to send log messages from session to TUI.
type sessionLogMsg string

// sessionStateChangeMsg is a tea.Msg to indicate session state has changed.
type sessionStateChangeMsg session.SessionState

// Model holds the state of the TUI.
type Model struct {
	activeTab      Tab
	width          int
	height         int
	err            error
	appConfig      *config.AppConfig
	logger         *utils.Logger // Main app logger (file)
	proxyManager   *proxy.ProxyManager
	reporter       *report.Reporter
	session        *session.Session
	targetURLInput string
	reasonInput    string
	logMessages    []string // For Live Session Logs view
	inputFocus     int      // 0 for URL, 1 for Reason
	sessionStatus  string   // Display current session status string
}

// NewInitialModel creates the initial TUI model.
func NewInitialModel(cfg *config.AppConfig, logger *utils.Logger) Model {
	m := Model{
		activeTab:   TargetInputTab,
		appConfig:   cfg,
		logger:      logger,
		logMessages: []string{"TUI Initialized. Welcome to sentinelgo/sentinelgo!"},
		inputFocus:  0, // Default focus to URL input
	}

	// Initialize Proxy Manager
	// TODO: Make proxy source path configurable (e.g., from appConfig or default)
	proxySourcePath := "config/proxies.csv"    // Or proxies.json
	if cfg.DefaultHeaders["ProxyFile"] != "" { // Example: allow setting via config
		proxySourcePath = cfg.DefaultHeaders["ProxyFile"]
	}

	initialProxies, err := proxy.LoadProxies(proxySourcePath)
	if err != nil {
		m.logMessages = append(m.logMessages, fmt.Sprintf("Error loading proxies from %s: %v", proxySourcePath, err))
		m.err = fmt.Errorf("failed to load proxies: %w", err)
		// Proceed with an empty proxy manager or handle error more gracefully
		initialProxies = []*proxy.ProxyInfo{}
	} else {
		m.logMessages = append(m.logMessages, fmt.Sprintf("Loaded %d proxies from %s.", len(initialProxies), proxySourcePath))
	}

	// TODO: Make strategy and healthyOnly configurable
	m.proxyManager = proxy.NewProxyManager(initialProxies, "round-robin", true)
	m.logMessages = append(m.logMessages, "Proxy manager initialized.")

	// Perform initial batch proxy check asynchronously.
	// For a real TUI, you'd send a tea.Cmd that wraps this and returns a tea.Msg on completion.
	// For simplicity here, just a goroutine. TUI won't get live updates from this specific check easily.
	if len(m.proxyManager.GetAllProxies()) > 0 {
		m.logMessages = append(m.logMessages, "Starting initial proxy health check...")
		go func() {
			// TODO: Make checkTimeout and concurrency configurable
			// Use a less aggressive default for initial check if many proxies
			checkTimeout := 10 * time.Second
			concurrency := 5
			proxy.BatchCheckProxies(m.proxyManager.GetAllProxies(), checkTimeout, concurrency)
			// This logging won't go to TUI's m.logMessages directly without a tea.Cmd/tea.Msg mechanism
			// It will go to the file logger used by BatchCheckProxies' fmt.Printf.
			// To update TUI: send a message back to tea.Program.Send(proxiesCheckedMsg{})
			m.logger.Info(utils.LogEntry{Message: "Initial batch proxy check completed."})
			// For TUI update, one might do: teaProgram.Send(proxiesCheckedMsg("Batch proxy check complete!"))
		}()
	}

	// Initialize Reporter
	dummyAnalyzer := ai.NewDummyAnalyzer(logger)                                // Create DummyAnalyzer instance
	m.reporter = report.NewReporter(cfg, m.proxyManager, logger, dummyAnalyzer) // Pass analyzer to Reporter
	m.logMessages = append(m.logMessages, "Reporter initialized with Dummy AI Analyzer.")

	return m
}

// listenForSessionLogsCmd waits for messages from the session's LogChannel.
func (m *Model) listenForSessionLogsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.session == nil || m.session.LogChannel == nil {
			return nil // Or an error message
		}
		logMsg, ok := <-m.session.LogChannel
		if !ok { // Channel closed
			// Optionally send a specific message indicating channel closure or session end
			return sessionLogMsg("Session log channel closed.")
		}
		return sessionLogMsg(logMsg)
	}
}

// Init is the first command that is run when the program starts.
func (m Model) Init() tea.Cmd {
	return nil // No initial command for now; proxy checks are fire-and-forget goroutine
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionLogMsg: // Message from session.LogChannel
		m.logMessages = append(m.logMessages, string(msg))
		if m.session != nil && m.session.GetState() != session.Running && m.session.GetState() != session.Paused {
			// If session ended/aborted/completed, stop listening.
			// However, runLoop might send a few more messages before channel truly closes.
			// For robustness, listen until channel is confirmed closed by listenForSessionLogsCmd returning specific msg or nil.
			if string(msg) == "Session log channel closed." || m.session.GetState() == session.Completed || m.session.GetState() == session.Aborted || m.session.GetState() == session.Failed {
				m.sessionStatus = fmt.Sprintf("Session: %s", m.session.GetState().String())
				return m, nil // Stop listening
			}
		}
		// Keep listening if session is active or messages are still coming
		cmds = append(cmds, m.listenForSessionLogsCmd())

	case tea.KeyMsg:
		// Global keybindings (like Ctrl+C, q for quit, tab navigation)
		// Specific keybindings if a session is active (p, r, a)
		if m.session != nil && (m.session.GetState() == session.Running || m.session.GetState() == session.Paused) {
			switch msg.String() {
			case "p": // Pause session
				if m.session.GetState() == session.Running {
					err := m.session.Pause()
					if err != nil {
						m.err = err
					} else {
						m.logMessages = append(m.logMessages, "Pause command sent.")
						m.sessionStatus = "Session: Pausing..."
					}
				}
			case "r": // Resume session
				if m.session.GetState() == session.Paused {
					err := m.session.Resume()
					if err != nil {
						m.err = err
					} else {
						m.logMessages = append(m.logMessages, "Resume command sent.")
						m.sessionStatus = "Session: Resuming..."
					}
				}
			case "a": // Abort session
				err := m.session.Abort() // Abort can be called even if stopping
				if err != nil {
					m.err = err
				} else {
					m.logMessages = append(m.logMessages, "Abort command sent.")
					m.sessionStatus = "Session: Aborting..."
				}
				// After abort, session.LogChannel might close. listenForSessionLogsCmd handles this.
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			// If a session is running, consider aborting it first or asking for confirmation.
			if m.session != nil && (m.session.GetState() == session.Running || m.session.GetState() == session.Paused) {
				m.logMessages = append(m.logMessages, "Session active. Press 'a' to abort session first, or Ctrl+C again to force quit.")
				// Optionally, implement a more robust confirmation or force quit mechanism.
				// For now, this makes 'q' and 'ctrl+c' less destructive if session is active.
				// Returning m, tea.Quit here would be the old behavior.
				// To implement "Ctrl+C again", you'd need to track ctrlCpresses.
				// For now, let 'a' be the primary way to stop a session.
				// If q is pressed when not typing, and no session, then quit.
				if msg.String() == "q" && (m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.reasonInput == "")) {
					return m, tea.Quit
				}

			} else if m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.reasonInput == "") {
				return m, tea.Quit
			}
			// If in input tab and typing 'q', treat as input.
			if m.activeTab == TargetInputTab && msg.String() == "q" {
				if m.inputFocus == 0 {
					m.targetURLInput += msg.String()
				} else {
					m.reasonInput += msg.String()
				}
			}

		case "ctrl+n":
			m.activeTab = (m.activeTab + 1) % numTabs
		case "ctrl+p":
			m.activeTab = (m.activeTab - 1 + numTabs) % numTabs

		case "tab":
			if m.activeTab == TargetInputTab {
				m.inputFocus = (m.inputFocus + 1) % 2 // Toggle focus
			}
		case "enter":
			if m.activeTab == TargetInputTab {
				if m.targetURLInput != "" && m.reasonInput != "" {
					if m.session != nil && (m.session.GetState() == session.Running || m.session.GetState() == session.Paused) {
						m.logMessages = append(m.logMessages, "A session is already active. Abort or wait for completion.")
						m.err = fmt.Errorf("session already active")
					} else {
						jobs := []struct {
							URL    string
							Reason string
						}{{URL: m.targetURLInput, Reason: m.reasonInput}}
						m.session = session.NewSession(m.reporter, jobs)
						m.err = m.session.Start()
						if m.err != nil {
							m.logMessages = append(m.logMessages, fmt.Sprintf("Error starting session: %v", m.err))
							m.sessionStatus = fmt.Sprintf("Session: Error - %v", m.err)
						} else {
							m.logMessages = append(m.logMessages, "New session started.")
							m.sessionStatus = "Session: Starting..."
							// Start listening for logs from this new session.
							cmds = append(cmds, m.listenForSessionLogsCmd())
						}
						m.targetURLInput = "" // Clear inputs after starting
						m.reasonInput = ""
						m.inputFocus = 0
					}
				} else {
					m.logMessages = append(m.logMessages, "Target URL and Reason cannot be empty.")
					m.err = fmt.Errorf("target URL and Reason cannot be empty")
				}
			}

		case "backspace":
			if m.activeTab == TargetInputTab {
				if m.inputFocus == 0 && len(m.targetURLInput) > 0 {
					m.targetURLInput = m.targetURLInput[:len(m.targetURLInput)-1]
				} else if m.inputFocus == 1 && len(m.reasonInput) > 0 {
					m.reasonInput = m.reasonInput[:len(m.reasonInput)-1]
				}
			}

		default:
			if m.activeTab == TargetInputTab && msg.Type == tea.KeyRunes && !strings.Contains(msg.String(), "ctrl+") {
				if m.inputFocus == 0 {
					m.targetURLInput += msg.String()
				} else {
					m.reasonInput += msg.String()
				}
			}
		}
	}

	// Update session status string if session exists
	if m.session != nil {
		sState := m.session.GetState()
		m.sessionStatus = fmt.Sprintf("Session: %s (%d/%d jobs)", sState.String(), m.session.ProcessedJobs, len(m.session.Jobs))
		// If state indicates termination, but we're still getting messages, ensure we keep listening
		if (sState == session.Running || sState == session.Paused || sState == session.Stopping) && len(cmds) == 0 {
			// Add listener if not already added by sessionLogMsg itself
			// This ensures that if Update is called for other reasons (e.g. WindowSizeMsg), listener is re-added.
			// However, this might lead to multiple listeners if not careful.
			// A better approach is to have a flag in Model `isListening` or ensure `listenForSessionLogsCmd` is idempotent / self-terminating.
			// For now, rely on sessionLogMsg to re-trigger listening.
		}
	} else {
		m.sessionStatus = "Session: Idle"
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI.
func (m Model) View() string {
	var s strings.Builder

	// Tab Bar
	var renderedTabs []string
	for i, name := range tabNames {
		if Tab(i) == m.activeTab {
			renderedTabs = append(renderedTabs, fmt.Sprintf("[%s]", name))
		} else {
			renderedTabs = append(renderedTabs, name)
		}
	}
	s.WriteString(strings.Join(renderedTabs, " | ") + "\n")
	s.WriteString(m.sessionStatus + "\n\n") // Display session status

	// Content based on active tab
	switch m.activeTab {
	case TargetInputTab:
		s.WriteString("Target URL:\n")
		if m.inputFocus == 0 {
			s.WriteString("> ")
		} else {
			s.WriteString("  ")
		}
		s.WriteString(m.targetURLInput + "\n\n")
		s.WriteString("Reason:\n")
		if m.inputFocus == 1 {
			s.WriteString("> ")
		} else {
			s.WriteString("  ")
		}
		s.WriteString(m.reasonInput + "\n\n")
		s.WriteString("(Ctrl+N/P Tabs | Tab Key Inputs | Enter Submit)\n")
		if m.session != nil && (m.session.GetState() == session.Running || m.session.GetState() == session.Paused) {
			s.WriteString("(P Pause | R Resume | A Abort Session)\n")
		}

	case ProxyMgmtTab:
		s.WriteString("Proxy Management - Coming Soon\n")
		if m.proxyManager != nil {
			s.WriteString(fmt.Sprintf("Loaded proxies: %d\n", len(m.proxyManager.GetAllProxies())))
			// Display some proxy stats, e.g., healthy count
			healthyCount := 0
			for _, p := range m.proxyManager.GetAllProxies() {
				if p.HealthStatus == "healthy" {
					healthyCount++
				}
			}
			s.WriteString(fmt.Sprintf("Healthy proxies: %d\n", healthyCount))
		}

	case SettingsTab:
		s.WriteString("Settings - Coming Soon\n")
		if m.appConfig != nil {
			s.WriteString(fmt.Sprintf("Max Retries from Config: %d\n", m.appConfig.MaxRetries))
			s.WriteString(fmt.Sprintf("Risk Threshold from Config: %.2f\n", m.appConfig.RiskThreshold))
		}

	case LiveSessionLogsTab:
		s.WriteString("Live Session Logs (from session.LogChannel & TUI direct):\n")
		maxLogsToShow := m.height - 7 // Adjusted for tab bar, status, and other lines
		if maxLogsToShow < 1 {
			maxLogsToShow = 1
		}
		start := 0
		if len(m.logMessages) > maxLogsToShow {
			start = len(m.logMessages) - maxLogsToShow
		}
		for _, msg := range m.logMessages[start:] {
			s.WriteString(msg + "\n")
		}
	case LogReviewTab:
		s.WriteString("Log Review & Export - Coming Soon\n")
		s.WriteString("Full structured logs are in 'sentinelgo/sentinelgo_session.log'.\n")
	}

	// Error display
	if m.err != nil {
		s.WriteString("\n\nError: " + m.err.Error())
		// m.err = nil // Clear error after displaying once, or require explicit clear command
	}

	s.WriteString(fmt.Sprintf("\n\nWindow Size: %d x %d. Press 'q' to consider quitting.", m.width, m.height))
	return s.String()
}
