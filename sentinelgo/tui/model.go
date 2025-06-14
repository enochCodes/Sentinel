package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss" // Import lipgloss
	"net/http"                         // For http.Cookie if AppConfig is detailed
	"strconv"                          // For parsing numReportsInput & settings
	"strings"
	"time" // For BatchCheckProxies delay example
	// "github.com/google/uuid" // No longer needed here directly

	"sentinelgo/ai"      // Import AI package
	"sentinelgo/config"
	"sentinelgo/proxy"   // Import proxy package
	"sentinelgo/report"  // Import report package
	"sentinelgo/session" // Import session package
	"sentinelgo/utils"

	"github.com/charmbracelet/bubbletea"
)

// Tab defines the different views/tabs in the TUI.
type Tab int

// Constants defining the available tabs in the TUI.
const (
	TargetInputTab Tab = iota // Tab for inputting target URL and number of reports.
	ProxyMgmtTab              // Tab for managing and viewing proxy status.
	SettingsTab               // Tab for viewing and editing application settings.
	LiveSessionLogsTab        // Tab for viewing live logs from an active reporting session.
	LogReviewTab              // Tab for reviewing past logs (placeholder).
	numTabs                   // Internal counter for the number of tabs.
)

// tabNames provides a user-friendly string representation for each Tab.
var tabNames = []string{
	"Target Input",
	"Proxy Management",
	"Settings",
	"Live Session Logs",
	"Log Review & Export",
}

// sessionLogMsg is a tea.Msg used to send log updates from a running session.Session
// to the TUI's Update method. It wraps a session.LogUpdate struct.
type sessionLogMsg struct{ update session.LogUpdate }

// EditableSettingEntry defines the structure for a setting that can be
// displayed and potentially edited in the Settings tab.
type EditableSettingEntry struct {
	Name         string      // User-friendly display name for the setting (e.g., "Max Retries").
	Path         string      // A unique identifier or path for the setting, used internally to map to AppConfig fields (e.g., "MaxRetries").
	Type         string      // Data type of the setting ("int", "float", "string"), used for validation and input handling.
	CurrentValue interface{} // The current value of the setting, retrieved from AppConfig.
	IsSensitive  bool        // Flag indicating if the value should be masked when displayed (e.g., API keys).
}

// Model is the central struct for the Bubble Tea TUI application.
// It holds all state related to the UI, application configuration,
// backend components (like reporter, proxy manager), and active session.
type Model struct {
	activeTab        Tab      // Currently active tab in the TUI.
	tabsDisplayNames []string // Slice of display names for tabs, used in rendering the tab bar.
	width            int      // Current width of the terminal window.
	height           int      // Current height of the terminal window.
	err              error    // Holds the last error encountered, displayed in the footer.

	appConfig    *config.AppConfig   // Pointer to the application's configuration.
	logger       *utils.Logger       // Pointer to the global structured logger.
	proxyManager *proxy.ProxyManager // Manages the pool of proxies.
	reporter     *report.Reporter    // Handles sending individual reports.
	session      *session.Session    // Pointer to the currently active reporting session (nil if no session is active).

	// Fields for the "Target Input" tab
	targetURLInput  string // Buffer for the target URL input.
	numReportsInput string // Buffer for the number of reports input (stored as string for text input).

	logMessages   []string // Slice of styled strings for display in the "Live Session Logs" tab.
	inputFocus    int      // Determines which input field has focus (0 for URL, 1 for NumReports on TargetInputTab; index on SettingsTab).
	sessionStatus string   // A styled string representing the current session status, displayed below the tab bar.

	// State fields for the "Settings" tab
	editableSettings  []EditableSettingEntry // List of settings that can be edited.
	settingsFocusIndex int                  // Index of the currently selected setting in the editableSettings slice.
	editingSetting    bool                 // True if the user is currently editing a setting's value.
	currentEditValue  string               // Buffer for the value being typed during a setting edit.
	originalEditValue interface{}          // Stores the original value of a setting before editing, for cancellation.
	editingSettingPath string               // The 'Path' of the setting currently being edited.
}

// NewInitialModel creates the initial state of the TUI Model.
// It initializes the application configuration, logger, proxy manager, reporter,
// and sets up default values for UI state (e.g., active tab, input fields).
//
// Parameters:
//   - cfg: A pointer to the loaded AppConfig.
//   - logger: A pointer to the application's global logger.
//
// Returns:
//   The fully initialized Model struct.
func NewInitialModel(cfg *config.AppConfig, logger *utils.Logger) Model {
	m := Model{
		activeTab:        TargetInputTab,
		tabsDisplayNames: tabNames,
		appConfig:        cfg,
		logger:           logger,
		logMessages:      []string{LogTimestampStyle.Render(time.Now().Format("15:04:05.000")) + " " + LogLevelInfoStyle.Render(LogPrefixInfo + " TUI Initialized. Welcome to SentinelGo!")},
		inputFocus:       0, // Default focus to the first input field on the active tab.
		numReportsInput:  "1", // Default value for number of reports.
	}

	m.populateEditableSettings() // Initialize the list of editable settings.

	// Initialize proxy manager
	proxySourcePath := "config/proxies.csv" // Default path
	if cfg != nil && cfg.DefaultHeaders["ProxyFile"] != "" { // Allow overriding via app config
		proxySourcePath = cfg.DefaultHeaders["ProxyFile"]
	}
	initialProxies, err := proxy.LoadProxies(proxySourcePath)
	if err != nil {
		// Log error to TUI and potentially to file logger via m.err or direct log
		m.logMessages = append(m.logMessages, LogLevelErrorStyle.Render(LogPrefixError+fmt.Sprintf(" Error loading proxies from %s: %v", proxySourcePath, err)))
		m.err = fmt.Errorf("failed to load proxies: %w", err) // Set error for display in footer
		initialProxies = []*proxy.ProxyInfo{} // Proceed with empty list
	} else {
		m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+fmt.Sprintf(" Loaded %d proxies from %s.", len(initialProxies), proxySourcePath)))
	}
	m.proxyManager = proxy.NewProxyManager(initialProxies, proxy.StrategyRoundRobin, true) // Default strategy
	m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+" Proxy manager initialized."))

	// Asynchronously start initial proxy health check if proxies are loaded.
	if len(m.proxyManager.GetAllProxies()) > 0 {
		m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+" Starting initial proxy health check (background)..."))
		go func() { // Fire-and-forget goroutine for initial check.
			// TODO: Consider a mechanism (tea.Cmd) to send a message back to TUI upon completion for status update.
			checkTimeout := 10 * time.Second // Configurable?
			concurrency := 5                // Configurable?
			m.proxyManager.BatchCheckProxies(m.proxyManager.GetAllProxies(), checkTimeout, concurrency)
			// Log completion to the file logger.
			m.logger.Info(utils.LogEntry{Message: "Initial batch proxy health check completed."})
		}()
	}

	// Initialize AI Analyzer (dummy for now) and Reporter.
	dummyAnalyzer := ai.NewDummyAnalyzer(logger)
	m.reporter = report.NewReporter(cfg, m.proxyManager, logger, dummyAnalyzer)
	m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+" Reporter initialized with Dummy AI Analyzer."))
	m.sessionStatus = SubtleTextStyle.Render("Session: Idle") // Initial session status.

	return m
}

// populateEditableSettings initializes or refreshes the list of settings that can be edited in the UI.
// It reads directly from `m.appConfig`. This should be called when `m.appConfig` is loaded or reloaded.
func (m *Model) populateEditableSettings() {
	if m.appConfig == nil { // Should not happen if NewInitialModel ensures appConfig is not nil
		m.editableSettings = []EditableSettingEntry{}
		m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogPrefixError+" Cannot populate settings: AppConfig is nil."))
		return
	}
	m.editableSettings = []EditableSettingEntry{
		{Name: "Max Retries", Path: "MaxRetries", Type: "int", CurrentValue: m.appConfig.MaxRetries},
		{Name: "Risk Threshold (%)", Path: "RiskThreshold", Type: "float", CurrentValue: m.appConfig.RiskThreshold},
		// Example for a string setting (if one were added to AppConfig directly)
		// {Name: "Default User Agent", Path: "DefaultHeaders.User-Agent", Type: "string", CurrentValue: m.appConfig.DefaultHeaders["User-Agent"]},
		// Note: Editing nested map/slice values like DefaultHeaders or APIKeys directly through this simple list
		// would require more complex Path handling and reflection/type assertions in Update logic.
		// For now, only top-level simple fields are handled.
	}
}

// listenForSessionLogsCmd returns a tea.Cmd that listens for the next LogUpdate
// from the active session's LogChannel. If the channel is closed or the session is nil,
// it sends a specific sessionLogMsg to indicate this.
func (m *Model) listenForSessionLogsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.session == nil || m.session.LogChannel == nil {
			// This indicates an issue, possibly session ended abruptly or was not set up.
			return sessionLogMsg{update: session.LogUpdate{Level: session.LogLevelUpdateError, Message: "TUI Error: Session or its LogChannel is nil.", Timestamp: time.Now()}}
		}
		logUpdate, ok := <-m.session.LogChannel // Blocking read from the channel.
		if !ok { // Channel has been closed by the sender (session.runLoop's defer).
			return sessionLogMsg{update: session.LogUpdate{Level: session.LogLevelUpdateWarn, Message: "Session log channel closed by sender.", Timestamp: time.Now()}}
		}
		return sessionLogMsg{update: logUpdate} // Send the received LogUpdate.
	}
}

// Init is called by Bubble Tea when the program starts.
// It can return an initial command to be executed.
func (m Model) Init() tea.Cmd { return nil } // No initial command needed for now.

// Update is the main message handling function for the TUI.
// It processes incoming tea.Msg types (like key presses, window size changes, custom messages)
// and updates the Model accordingly. It can also return a tea.Cmd for further actions.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	m.err = nil // Clear previous general error on any new message or action.

	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // Handle terminal window resize events.
		m.width = msg.Width
		m.height = msg.Height

	case sessionLogMsg: // Handle log updates from the active session.
		logEntry := msg.update
		var styledLog string
		var logStyle lipgloss.Style
		var prefix string

		// Determine style and prefix based on log level.
		switch logEntry.Level {
		case session.LogLevelUpdateInfo: logStyle, prefix = LogLevelInfoStyle, LogPrefixInfo
		case session.LogLevelUpdateError: logStyle, prefix = LogLevelErrorStyle, LogPrefixError
		case session.LogLevelUpdateWarn: logStyle, prefix = LogLevelWarnStyle, LogPrefixWarn
		case session.LogLevelUpdateDebug: logStyle, prefix = LogLevelDebugStyle, LogPrefixDebug
		default: logStyle, prefix = NormalTextStyle, "[???]" // Fallback for unknown levels.
		}

		timestampStr := LogTimestampStyle.Render(logEntry.Timestamp.Format("15:04:05.000")) // Format timestamp.
		styledMsgPart := logStyle.Render(prefix + " " + logEntry.Message) // Style prefix and message.
		styledLog = fmt.Sprintf("%s %s", timestampStr, styledMsgPart) // Combine parts.

		m.logMessages = append(m.logMessages, styledLog) // Add to TUI log display buffer.

		// If the message indicates the log channel was closed, stop listening.
		if logEntry.Message == "Session log channel closed by sender." {
			if m.session != nil { // Update final session status.
				sState, _, numToSend, attempted, ok, fail := m.session.GetStats()
				m.sessionStatus = fmt.Sprintf("Session: %s | Target: %s | Reports: %d/%d | OK: %s | Fail: %s",
					sState.String(), m.session.TargetURL, attempted, numToSend,
					SuccessTextStyle.Render(fmt.Sprintf("%d", ok)), ErrorTextStyle.Render(fmt.Sprintf("%d", fail)))
			} else { // Should ideally not happen if channel belonged to a session.
				m.sessionStatus = ErrorTextStyle.Render("Session: ERROR - Log channel closed but session is nil")
			}
			return m, nil // No further command; stop listening.
		}
		// Continue listening for more log messages from the session.
		cmds = append(cmds, m.listenForSessionLogsCmd())

	case tea.KeyMsg: // Handle keyboard input.
		// Settings tab edit mode has priority for key handling.
		if m.activeTab == SettingsTab && m.editingSetting {
			switch msg.String() {
			case "enter": // Confirm edit.
				if m.settingsFocusIndex < 0 || m.settingsFocusIndex >= len(m.editableSettings) {
					m.err = fmt.Errorf("internal error: invalid settingsFocusIndex %d", m.settingsFocusIndex)
					m.editingSetting = false
					break
				}
				settingToEdit := &m.editableSettings[m.settingsFocusIndex]
				isValid := true
				var parseErr error

				// Validate and convert based on setting type.
				switch settingToEdit.Type {
				case "int":
					val, errConv := strconv.Atoi(m.currentEditValue)
					if errConv != nil { parseError = fmt.Errorf("invalid integer value: %w", errConv); isValid = false
					} else { // Apply change to AppConfig.
						if settingToEdit.Path == "MaxRetries" { m.appConfig.MaxRetries = val }
						settingToEdit.CurrentValue = val // Update UI model.
					}
				case "float":
					val, errConv := strconv.ParseFloat(m.currentEditValue, 64)
					if errConv != nil { parseError = fmt.Errorf("invalid float value: %w", errConv); isValid = false
					} else { // Apply change to AppConfig.
						if settingToEdit.Path == "RiskThreshold" { m.appConfig.RiskThreshold = val }
						settingToEdit.CurrentValue = val // Update UI model.
					}
				// Add case "string" here if string settings become editable.
				}

				if !isValid { m.err = parseError
				} else {
					m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixInfo+fmt.Sprintf(" Setting '%s' updated locally to '%s'. Use Ctrl+S to save.", settingToEdit.Name, m.currentEditValue)))
				}
				m.editingSetting = false // Exit edit mode.
			case "esc": // Cancel edit.
				m.editingSetting = false
				m.logMessages = append(m.logMessages, LogLevelWarnStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixWarn+" Edit cancelled for '"+m.editableSettings[m.settingsFocusIndex].Name+"'."))
			case "backspace": // Handle backspace.
				if len(m.currentEditValue) > 0 { m.currentEditValue = m.currentEditValue[:len(m.currentEditValue)-1] }
			default: // Append typed characters.
				if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace { // Allow spaces for potential future string settings.
					// TODO: Implement live input restrictions for int/float types if desired.
					m.currentEditValue += msg.String()
				}
			}
		} else { // Not editing a setting, or not on Settings tab.
			// Session control keybindings (P, R, A) if a session is active.
			if m.session != nil {
				sState, _, _, _, _, _ := m.session.GetStats()
				if sState == session.Running || sState == session.Paused {
					tsNow := LogTimestampStyle.Render(time.Now().Format("15:04:05.000")) + " "
					switch msg.String() {
					case "p": if sState == session.Running { if err := m.session.Pause(); err != nil { m.err = err } else { m.logMessages = append(m.logMessages, LogLevelWarnStyle.Render(tsNow+LogPrefixWarn+" Pause command sent.")) }}
					case "r": if sState == session.Paused { if err := m.session.Resume(); err != nil { m.err = err } else { m.logMessages = append(m.logMessages, LogLevelWarnStyle.Render(tsNow+LogPrefixWarn+" Resume command sent.")) }}
					case "a": if err := m.session.Abort(); err != nil { m.err = err } else { m.logMessages = append(m.logMessages, LogLevelWarnStyle.Render(tsNow+LogPrefixWarn+" Abort command sent.")) }}
				}
			}

			// Global keybindings.
			switch msg.String() {
			case "ctrl+c", "q": // Quit logic.
				if m.session != nil {
					sState, _, _, _, _, _ := m.session.GetStats()
					if sState == session.Running || sState == session.Paused { // If session active, warn before quit.
						m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixWarn+" Session active. Press 'a' to abort, or Ctrl+C again to force quit."))
						if msg.String() == "q" && (m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.numReportsInput == "")) { /* no quit on 'q' if session active */ }
						else if msg.String() == "ctrl+c" { return m, tea.Quit } // Force quit on Ctrl+C
						break // Do not fall through to other quit conditions if session is active
					}
				}
				// If no active session or not typing in target input, allow 'q' to quit.
				if m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.numReportsInput == "") { return m, tea.Quit }
				// If on target input tab and press 'q', treat as input unless fields are empty.
				if m.activeTab == TargetInputTab && msg.String() == "q" { if m.inputFocus == 0 { m.targetURLInput += msg.String() } }


			case "ctrl+n": // Next tab.
				m.activeTab = (m.activeTab + 1) % numTabs; m.editingSetting = false; m.settingsFocusIndex = 0
			case "ctrl+p": // Previous tab.
				m.activeTab = (m.activeTab - 1 + numTabs) % numTabs; m.editingSetting = false; m.settingsFocusIndex = 0

			case "ctrl+s": // Save settings (only if on SettingsTab).
				if m.activeTab == SettingsTab {
					err := config.SaveAppConfig("config/sentinel.yaml", m.appConfig)
					ts := LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))
					if err != nil {
						m.err = fmt.Errorf("failed to save config: %w", err)
						m.logMessages = append(m.logMessages, ErrorTextStyle.Render(ts+" "+LogPrefixError+" Failed to save settings: "+err.Error()))
					} else {
						m.logMessages = append(m.logMessages, SuccessTextStyle.Render(ts+" "+LogPrefixInfo+" Settings saved to config/sentinel.yaml."))
					}
				}
            case "ctrl+r": // Reload settings (only if on SettingsTab).
                if m.activeTab == SettingsTab {
                    newCfg, err := config.LoadAppConfig("config/sentinel.yaml")
                    ts := LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))
                    if err != nil {
                        m.err = fmt.Errorf("failed to reload config: %w", err)
                        m.logMessages = append(m.logMessages, ErrorTextStyle.Render(ts+" "+LogPrefixError+" Failed to reload settings: "+err.Error()))
                    } else {
                        m.appConfig = newCfg
                        m.populateEditableSettings() // Refresh UI list with new values.
                        m.logMessages = append(m.logMessages, SuccessTextStyle.Render(ts+" "+LogPrefixInfo+" Settings reloaded from config/sentinel.yaml."))
                    }
                }

			// Tab-specific keybindings (when not editing settings).
			default:
				if m.activeTab == TargetInputTab { // Input handling for TargetInputTab.
					switch msg.String() {
					case "tab": m.inputFocus = (m.inputFocus + 1) % 2 // Cycle focus: 0 for URL, 1 for NumReports.
					case "enter": // Submit action for TargetInputTab.
						numReportsInt, errConv := strconv.Atoi(m.numReportsInput)
						if errConv != nil || numReportsInt <= 0 {
							m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+" Number of reports must be a positive integer."))
							m.err = fmt.Errorf("invalid number of reports: '%s'", m.numReportsInput)
						} else if m.targetURLInput == "" {
							m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+" Target URL cannot be empty."))
							m.err = fmt.Errorf("target URL cannot be empty")
						} else { // Valid inputs, proceed to session logic.
							currentSessionState := session.Idle
							if m.session != nil { currentSessionState, _,_,_,_,_ = m.session.GetStats() }
							if currentSessionState == session.Running || currentSessionState == session.Paused {
								m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+" A session is already active. Abort or wait for completion."))
								m.err = fmt.Errorf("session already active")
							} else { // Okay to start a new session.
								m.session = session.NewSession(m.reporter, m.targetURLInput, numReportsInt)
								m.err = m.session.Start() // Start the session.
								if m.err != nil { // Handle error from session.Start().
									m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+fmt.Sprintf(" Error starting session: %v", m.err)))
								} else { // Session started successfully.
									m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixInfo+fmt.Sprintf(" New session started for %d reports to %s.", numReportsInt, m.targetURLInput)))
									cmds = append(cmds, m.listenForSessionLogsCmd()) // Start listening for logs.
								}
								m.targetURLInput = ""      // Clear target URL input.
								m.inputFocus = 0           // Reset focus to URL input.
							}
						}
					case "backspace": // Handle backspace for TargetInputTab.
						if m.inputFocus == 0 && len(m.targetURLInput) > 0 { m.targetURLInput = m.targetURLInput[:len(m.targetURLInput)-1] }
						if m.inputFocus == 1 && len(m.numReportsInput) > 0 { m.numReportsInput = m.numReportsInput[:len(m.numReportsInput)-1] }
					default: // Character input for TargetInputTab.
						if msg.Type == tea.KeyRunes && !strings.Contains(msg.String(), "ctrl+") { // Ignore control sequences.
							runeStr := msg.String()
							if m.inputFocus == 0 { m.targetURLInput += runeStr } // Append to URL input.
							if m.inputFocus == 1 { // Append to NumReports input, filtering for digits.
								for _, r := range runeStr { if r >= '0' && r <= '9' { m.numReportsInput += string(r) } }
							}
						}
					}
				} else if m.activeTab == SettingsTab { // Navigation/activation for SettingsTab (when not editing).
					switch msg.String() {
					case "up", "k": if m.settingsFocusIndex > 0 { m.settingsFocusIndex-- }
					case "down", "j": if m.settingsFocusIndex < len(m.editableSettings)-1 { m.settingsFocusIndex++ }
					case "enter": // Enter edit mode for selected setting.
						if m.settingsFocusIndex < len(m.editableSettings) {
							m.editingSetting = true
							m.editingSettingPath = m.editableSettings[m.settingsFocusIndex].Path
							m.originalEditValue = m.editableSettings[m.settingsFocusIndex].CurrentValue
							m.currentEditValue = fmt.Sprintf("%v", m.originalEditValue)
							m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixInfo+fmt.Sprintf(" Editing '%s'...", m.editableSettings[m.settingsFocusIndex].Name)))
						}
					}
				}
			}
		}
	}

	// Update session status string for display.
	if m.session != nil {
		sState, _, numToSend, attempted, successful, failed := m.session.GetStats()
		targetStr := m.session.TargetURL
		if len(targetStr) > 30 { targetStr = targetStr[:27]+"..."} // Truncate long URLs
		m.sessionStatus = fmt.Sprintf("Session: %s | Target: %s | Reports: %d/%d | OK: %s | Fail: %s",
			sState.String(), targetStr, attempted, numToSend,
			SuccessTextStyle.Render(fmt.Sprintf("%d", successful)), ErrorTextStyle.Render(fmt.Sprintf("%d", failed)))
	} else {
		m.sessionStatus = SubtleTextStyle.Render("Session: Idle")
	}
	return m, tea.Batch(cmds...)
}

// renderHeader, renderFooter, renderTabBar, renderSettingsView, View remain here...
// (Make sure to include the full, correct versions of these functions from the previous step's final file content)
func (m Model) renderHeader() string {
	title := HeaderStyle.Render("SentinelGo++ Cyber Operations Platform")
	return title
}
func (m Model) renderFooter() string {
	helpKeyStyle := HelpTextStyle.Copy().Bold(true)
	var helpParts []string
	if m.activeTab == SettingsTab {
		if m.editingSetting {
			helpParts = append(helpParts, helpKeyStyle.Render("Enter:")+HelpTextStyle.Render(" Confirm | "), helpKeyStyle.Render("Esc:")+HelpTextStyle.Render(" Cancel"))
		} else {
			helpParts = append(helpParts, helpKeyStyle.Render("↑/↓:")+HelpTextStyle.Render(" Nav | "), helpKeyStyle.Render("Enter:")+HelpTextStyle.Render(" Edit | "), helpKeyStyle.Render("Ctrl+S:")+HelpTextStyle.Render(" Save | "), helpKeyStyle.Render("Ctrl+R:")+HelpTextStyle.Render(" Reload"))
		}
	} else {
		helpParts = append(helpParts, helpKeyStyle.Render("Ctrl+N/P:")+HelpTextStyle.Render(" Nav Tabs"))
		if m.session != nil {
			sState, _, _, _, _, _ := m.session.GetStats()
			if sState == session.Running || sState == session.Paused {
				sessionHelp := lipgloss.JoinHorizontal(lipgloss.Left,
					HelpTextStyle.Render(SymbolPointer+" Session: "),
					helpKeyStyle.Render("P "), HelpTextStyle.Render("Pause | "),
					helpKeyStyle.Render("R "), HelpTextStyle.Render("Resume | "),
					helpKeyStyle.Render("A "), HelpTextStyle.Render("Abort"),
				)
				helpParts = append(helpParts, sessionHelp)
			}
		}
	}
	helpParts = append(helpParts, []string{helpKeyStyle.Render("Ctrl+C:") + HelpTextStyle.Render(" Quit")}...)
	helpFullString := strings.Join(helpParts, HelpTextStyle.Render(" | "))
	var footerElements []string
	if m.err != nil {
		errorMsg := ErrorMessageStyle.Render(SymbolFailure + " Error: " + m.err.Error())
		footerElements = append(footerElements, errorMsg)
	}
	footerElements = append(footerElements, HelpTextStyle.Render(helpFullString))
	return lipgloss.NewStyle().PaddingTop(1).Render(lipgloss.JoinVertical(lipgloss.Left, footerElements...))
}
func (m Model) renderTabBar() string {
	var renderedTabs []string
	for i, name := range m.tabsDisplayNames {
		style, prefix := TabStyle, SymbolNotFocused
		if Tab(i) == m.activeTab { style, prefix = ActiveTabStyle, SymbolActiveTab }
		renderedTabs = append(renderedTabs, style.Render(prefix+" "+name))
	}
	tabBarContainerStyle := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderTop(false).BorderLeft(false).BorderRight(false).BorderForeground(BorderColor).PaddingBottom(0)
	return tabBarContainerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Bottom, renderedTabs...))
}
func (m Model) renderSettingsView() string {
	var content strings.Builder
	content.WriteString(HeaderStyle.Render(SymbolListItem+" Application Configuration") + "\n\n")

	if m.appConfig == nil {
		content.WriteString(WarningTextStyle.Render(SymbolWarning + " Application configuration not loaded."))
		return content.String()
	}

	for i, setting := range m.editableSettings {
		lineStyle := NormalTextStyle
		focusMarker := SymbolNotFocused + " "
		if !m.editingSetting && i == m.settingsFocusIndex {
			lineStyle = ActiveTabStyle.Copy().BorderStyle(lipgloss.HiddenBorder())
			focusMarker = SymbolFocused + " "
		}

		keyStr := keyStyle.Render(setting.Name + ":") // keyStyle already bolded
		var valueStr string
		if m.editingSetting && i == m.settingsFocusIndex {
			valueStr = FocusedInputStyle.Render(m.currentEditValue + "_")
		} else {
			currentValDisplay := fmt.Sprintf("%v", setting.CurrentValue)
			if setting.Type == "float" {
				if fVal, ok := setting.CurrentValue.(float64); ok {
					currentValDisplay = fmt.Sprintf("%.2f", fVal)
					if setting.Path == "RiskThreshold" { currentValDisplay += "%" }
				} else {
					currentValDisplay = ErrorTextStyle.Render("N/A (float expected)")
				}
			}
			if setting.IsSensitive { // Masking logic
				if len(currentValDisplay) > 8 { currentValDisplay = currentValDisplay[:4] + strings.Repeat("*", len(currentValDisplay)-8) + currentValDisplay[len(currentValDisplay)-4:]
				} else if len(currentValDisplay) > 0 { currentValDisplay = strings.Repeat("*", len(currentValDisplay)) }
			}
			valueStr = valueStyle.Render(currentValDisplay)
		}

		content.WriteString(lineStyle.Render(focusMarker + keyStr + " " + valueStr) + "\n")
	}

	content.WriteString(HelpTextStyle.Render("\n\n(Navigate with ↑/↓, Enter to Edit/Confirm, Esc to Cancel. Ctrl+S to Save, Ctrl+R to Reload from file.)"))
	return content.String()
}
func (m Model) View() string {
	headerView := m.renderHeader()
	tabBarView := m.renderTabBar()
	var currentTabView strings.Builder
	currentTabView.WriteString(NormalTextStyle.Render(m.sessionStatus + "\n"))

	switch m.activeTab {
	case TargetInputTab:
		currentTabView.WriteString(HeaderStyle.Render(SymbolListItem+" Target Input") + "\n")
		urlLabel := NormalTextStyle.Render(SymbolInputMarker + " Target URL")
		var urlInputView string
		urlInputDisplay := m.targetURLInput
		if m.inputFocus == 0 { urlInputDisplay += "_"; urlInputView = FocusedInputStyle.Render(SymbolFocused + " " + urlInputDisplay)
		} else { urlInputView = BlurredInputStyle.Render(SymbolNotFocused + " " + urlInputDisplay) }
		currentTabView.WriteString(urlLabel + "\n" + urlInputView + "\n\n")
		numReportsLabel := NormalTextStyle.Render(SymbolInputMarker + " Number of Reports")
		var numReportsInputView string
		numReportsInputDisplay := m.numReportsInput
		if m.inputFocus == 1 { numReportsInputDisplay += "_"; numReportsInputView = FocusedInputStyle.Render(SymbolFocused + " " + numReportsInputDisplay)
		} else { numReportsInputView = BlurredInputStyle.Render(SymbolNotFocused + " " + numReportsInputDisplay) }
		currentTabView.WriteString(numReportsLabel + "\n" + numReportsInputView + "\n\n")
		helpText := "Tab: Switch Fields | Enter: Submit Report"
		if m.session != nil {
			sState,_,_,_,_,_ := m.session.GetStats()
			if sState == session.Running || sState == session.Paused {
				sessionHelp := lipgloss.JoinHorizontal(lipgloss.Left, HelpTextStyle.Render(SymbolPointer+" Session: "), helpKeyStyle.Bold(true).Render("P "), HelpTextStyle.Render("Pause | "), helpKeyStyle.Bold(true).Render("R "), HelpTextStyle.Render("Resume | "), helpKeyStyle.Bold(true).Render("A "), HelpTextStyle.Render("Abort"))
				helpText += " | " + sessionHelp
			}
		}
		currentTabView.WriteString(HelpTextStyle.Render(helpText) + "\n")
	case ProxyMgmtTab:
		currentTabView.WriteString(HeaderStyle.Render(SymbolListItem+" Proxy Pool Status") + "\n\n")
		if m.proxyManager != nil {
			allProxies := m.proxyManager.GetAllProxies(); totalProxies := len(allProxies); healthyCount := 0; unknownCount := 0
			for _, p := range allProxies { if p.HealthStatus == "healthy" { healthyCount++ } else if p.HealthStatus == "unknown" { unknownCount++ } }
			unhealthyCount := totalProxies - healthyCount - unknownCount
			statsStyle := NormalTextStyle.Copy().PaddingBottom(0)
			currentTabView.WriteString(statsStyle.Render(fmt.Sprintf("%s Total Proxies: %s", SymbolInfo, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", totalProxies)))) + "\n")
			currentTabView.WriteString(statsStyle.Render(fmt.Sprintf("%s Healthy:       %s", SymbolSuccess, SuccessTextStyle.Render(fmt.Sprintf("%d", healthyCount)))) + "\n")
			currentTabView.WriteString(statsStyle.Render(fmt.Sprintf("%s Unhealthy:     %s", SymbolFailure, ErrorTextStyle.Render(fmt.Sprintf("%d", unhealthyCount)))) + "\n")
			if unknownCount > 0 { currentTabView.WriteString(statsStyle.Render(fmt.Sprintf("%s Unknown:       %s", SymbolWarning, WarningTextStyle.Render(fmt.Sprintf("%d", unknownCount)))) + "\n") }
			currentTabView.WriteString("\n" + SubtleTextStyle.Render(SymbolInfo+" Initial health checks run in background. Statuses update over time.") + "\n")
		} else { currentTabView.WriteString(WarningTextStyle.Render(SymbolWarning+" Proxy Manager not initialized.") + "\n") }
		currentTabView.WriteString(HelpTextStyle.Render("\n(Detailed proxy list, manual checks, and import/export coming soon...)"))
	case SettingsTab: currentTabView.WriteString(m.renderSettingsView())
	case LiveSessionLogsTab:
		currentTabView.WriteString(HeaderStyle.Render(SymbolListItem+" Live Session Logs") + "\n")
		maxLogsToShow := m.height - 12; if maxLogsToShow < 1 { maxLogsToShow = 5 }
		displayLogs := m.logMessages; if len(m.logMessages) > maxLogsToShow { displayLogs = m.logMessages[len(m.logMessages)-maxLogsToShow:] }
		for _, styledMsg := range displayLogs { currentTabView.WriteString(styledMsg + "\n") }
	case LogReviewTab:
		currentTabView.WriteString(HeaderStyle.Render(SymbolListItem+" Log Review & Export") + "\n\n")
		mainMessage := InfoTextStyle.Render(fmt.Sprintf("%s Log review and advanced export functionalities are currently under development.", SymbolInfo))
		additionalNote := HelpTextStyle.Render(fmt.Sprintf("All detailed session logs are being saved in JSON lines format to %s.", lipgloss.NewStyle().Foreground(PrimaryGreen).Render("sentinelgo_session.log")))
		comingSoon := SubtleTextStyle.Render("\n(More features coming soon!)")
		currentTabView.WriteString(lipgloss.NewStyle().PaddingBottom(1).Render(mainMessage) + "\n")
		currentTabView.WriteString(lipgloss.NewStyle().PaddingBottom(1).Render(additionalNote) + "\n")
		currentTabView.WriteString(comingSoon + "\n")
	}
	footerView := m.renderFooter()
	contentHeight := m.height - lipgloss.Height(headerView) - lipgloss.Height(tabBarView) - lipgloss.Height(footerView) - BoxStyle.GetVerticalPadding()
	if contentHeight < 0 { contentHeight = 0 }
	contentBoxStyle := BoxStyle.Copy().MaxHeight(contentHeight).MaxWidth(m.width - BoxStyle.GetHorizontalBorderSize())
	finalView := lipgloss.JoinVertical(lipgloss.Left, headerView, tabBarView, contentBoxStyle.Render(currentTabView.String()), footerView)
	return AppStyle.Width(m.width).Height(m.height).Render(finalView)
}
