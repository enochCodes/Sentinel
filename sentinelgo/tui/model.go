package tui

import (
	"fmt"
	"strconv" // For parsing numReportsInput & settings
	"strings"
	"time" // For BatchCheckProxies delay example

	"github.com/charmbracelet/lipgloss" // Import lipgloss
	// For http.Cookie if AppConfig is detailed

	// "github.com/google/uuid" // No longer needed here directly

	"sentinelgo/sentinelgo/ai" // Import AI package
	"sentinelgo/sentinelgo/config"
	"sentinelgo/sentinelgo/proxy"   // Import proxy package
	"sentinelgo/sentinelgo/report"  // Import report package
	"sentinelgo/sentinelgo/session" // Import session package
	"sentinelgo/sentinelgo/utils"

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
type sessionLogMsg struct{ update session.LogUpdate }

// EditableSettingEntry defines a setting that can be modified in the TUI.
type EditableSettingEntry struct {
	Name         string      // Display name, e.g., "Max Retries"
	Path         string      // Path to identify the field in AppConfig, e.g., "MaxRetries"
	Type         string      // "int", "float", "string" (for now)
	CurrentValue interface{} // The actual current value from m.appConfig
	IsSensitive  bool
}

// Model holds the state of the TUI.
type Model struct {
	activeTab        Tab
	tabsDisplayNames []string
	width            int
	height           int
	err              error
	appConfig        *config.AppConfig
	logger           *utils.Logger
	proxyManager     *proxy.ProxyManager
	reporter         *report.Reporter
	session          *session.Session
	targetURLInput   string
	numReportsInput  string
	logMessages      []string
	inputFocus       int // TargetInputTab: 0 for URL, 1 for NumReports. SettingsTab: index of editableSettings.
	sessionStatus    string

	// Settings Tab State
	editableSettings   []EditableSettingEntry
	settingsFocusIndex int         // Which setting item is currently highlighted/selected
	editingSetting     bool        // True if currently in "edit mode" for a setting
	currentEditValue   string      // Buffer to hold the text being typed for the setting
	originalEditValue  interface{} // To store the original value if user cancels edit
	editingSettingPath string      // Path of the setting being edited
}

// NewInitialModel creates the initial TUI model.
func NewInitialModel(cfg *config.AppConfig, logger *utils.Logger) Model {
	// logo
	// sleep 3 sec
	m := Model{
		activeTab:        TargetInputTab,
		tabsDisplayNames: tabNames,
		appConfig:        cfg,
		logger:           logger,
		logMessages:      []string{LogTimestampStyle.Render(time.Now().Format("15:04:05")) + " " + LogLevelInfoStyle.Render(LogPrefixInfo+" TUI Initialized. Welcome to SentinelGo!")},
		inputFocus:       0,
		numReportsInput:  "1",
	}

	// Initialize editableSettings (simple version for now)
	m.populateEditableSettings()

	proxySourcePath := "config/proxies.csv"
	if cfg != nil && cfg.DefaultHeaders["ProxyFile"] != "" {
		proxySourcePath = cfg.DefaultHeaders["ProxyFile"]
	}
	initialProxies, err := proxy.LoadProxies(proxySourcePath)
	if err != nil {
		m.logMessages = append(m.logMessages, LogLevelErrorStyle.Render(LogPrefixError+fmt.Sprintf(" Error loading proxies from %s: %v", proxySourcePath, err)))
		m.err = fmt.Errorf("failed to load proxies: %w", err)
		initialProxies = []*proxy.ProxyInfo{}
	} else {
		m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+fmt.Sprintf(" Loaded %d proxies from %s.", len(initialProxies), proxySourcePath)))
	}
	m.proxyManager = proxy.NewProxyManager(initialProxies, "round-robin", true)
	m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+" Proxy manager initialized."))
	if len(m.proxyManager.GetAllProxies()) > 0 {
		m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+" Starting initial proxy health check..."))
		go func() {
			checkTimeout := 10 * time.Second
			concurrency := 5
			proxy.BatchCheckProxies(m.proxyManager.GetAllProxies(), checkTimeout, concurrency)
			m.logger.Info(utils.LogEntry{Message: "Initial batch proxy check completed."})
		}()
	}
	dummyAnalyzer := ai.NewDummyAnalyzer(logger)
	m.reporter = report.NewReporter(cfg, m.proxyManager, logger, dummyAnalyzer)
	m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+" Reporter initialized with Dummy AI Analyzer."))
	m.sessionStatus = SubtleTextStyle.Render("Session: Idle")
	return m
}

func (m *Model) populateEditableSettings() {
	if m.appConfig == nil {
		m.editableSettings = []EditableSettingEntry{}
		return
	}
	m.editableSettings = []EditableSettingEntry{
		{Name: "Max Retries", Path: "MaxRetries", Type: "int", CurrentValue: m.appConfig.MaxRetries},
		{Name: "Risk Threshold (%)", Path: "RiskThreshold", Type: "float", CurrentValue: m.appConfig.RiskThreshold},
		// TODO: Add more settings later, e.g., DefaultHeaders["User-Agent"]
	}
}

// listenForSessionLogsCmd waits for messages from the session's LogChannel.
func (m *Model) listenForSessionLogsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.session == nil || m.session.LogChannel == nil {
			return sessionLogMsg{update: session.LogUpdate{Level: session.LogLevelUpdateError, Message: "TUI Error: Session or LogChannel nil", Timestamp: time.Now()}}
		}
		logUpdate, ok := <-m.session.LogChannel
		if !ok {
			return sessionLogMsg{update: session.LogUpdate{Level: session.LogLevelUpdateWarn, Message: "Session log channel closed by sender.", Timestamp: time.Now()}}
		}
		return sessionLogMsg{update: logUpdate}
	}
}

// Init is the first command that is run when the program starts.
func (m Model) Init() tea.Cmd { return nil }

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	m.err = nil // Clear error on any new message/action for now

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case sessionLogMsg:
		logEntry := msg.update
		var styledLog string
		var logStyle lipgloss.Style
		var prefix string
		switch logEntry.Level {
		case session.LogLevelUpdateInfo:
			logStyle, prefix = LogLevelInfoStyle, LogPrefixInfo
		case session.LogLevelUpdateError:
			logStyle, prefix = LogLevelErrorStyle, LogPrefixError
		case session.LogLevelUpdateWarn:
			logStyle, prefix = LogLevelWarnStyle, LogPrefixWarn
		case session.LogLevelUpdateDebug:
			logStyle, prefix = LogLevelDebugStyle, LogPrefixDebug
		default:
			logStyle, prefix = NormalTextStyle, "[???]"
		}
		timestampStr := LogTimestampStyle.Render(logEntry.Timestamp.Format("15:04:05.000"))
		styledMsgPart := logStyle.Render(prefix + " " + logEntry.Message)
		styledLog = fmt.Sprintf("%s %s", timestampStr, styledMsgPart)
		m.logMessages = append(m.logMessages, styledLog)
		if logEntry.Message == "Session log channel closed by sender." {
			if m.session != nil {
				sState, _, numToSend, attempted, ok, fail := m.session.GetStats()
				m.sessionStatus = fmt.Sprintf("Session: %s | Target: %s | Reports: %d/%d | OK: %s | Fail: %s",
					sState.String(), m.session.TargetURL, attempted, numToSend,
					SuccessTextStyle.Render(fmt.Sprintf("%d", ok)), ErrorTextStyle.Render(fmt.Sprintf("%d", fail)))
			} else {
				m.sessionStatus = ErrorTextStyle.Render("Session: ERROR - Log channel closed but session is nil")
			}
			return m, nil // Stop listening
		}
		cmds = append(cmds, m.listenForSessionLogsCmd())

	case tea.KeyMsg:
		if m.activeTab == SettingsTab && m.editingSetting {
			// Handle input for settings edit mode
			switch msg.String() {
			case "enter":
				// Validate and save the edited setting
				if m.settingsFocusIndex < 0 || m.settingsFocusIndex >= len(m.editableSettings) {
					m.err = fmt.Errorf("invalid settings focus index")
					m.editingSetting = false
					break
				}
				settingToEdit := &m.editableSettings[m.settingsFocusIndex] // Use pointer
				isValid := true
				var parseError error

				switch settingToEdit.Type {
				case "int":
					val, err := strconv.Atoi(m.currentEditValue)
					if err != nil {
						parseError = fmt.Errorf("invalid integer: %w", err)
						isValid = false
					} else {
						if settingToEdit.Path == "MaxRetries" {
							m.appConfig.MaxRetries = val
						}
						settingToEdit.CurrentValue = val
					}
				case "float":
					val, err := strconv.ParseFloat(m.currentEditValue, 64)
					if err != nil {
						parseError = fmt.Errorf("invalid float: %w", err)
						isValid = false
					} else {
						if settingToEdit.Path == "RiskThreshold" {
							m.appConfig.RiskThreshold = val
						}
						settingToEdit.CurrentValue = val
					}
				case "string":
					// Add string setting updates here if any
					settingToEdit.CurrentValue = m.currentEditValue
				}

				if !isValid {
					m.err = parseError
					// Optionally revert m.currentEditValue or keep it for user to fix
				} else {
					m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixInfo+fmt.Sprintf(" Setting '%s' updated to '%s'. (Ctrl+S to save to file)", settingToEdit.Name, m.currentEditValue)))
					m.editingSetting = false
				}
			case "esc":
				m.editingSetting = false
				m.logMessages = append(m.logMessages, LogLevelWarnStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixWarn+" Edit cancelled for '"+m.editableSettings[m.settingsFocusIndex].Name+"'."))
			case "backspace":
				if len(m.currentEditValue) > 0 {
					m.currentEditValue = m.currentEditValue[:len(m.currentEditValue)-1]
				}
			default:
				if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace { // Allow space for strings
					// TODO: Add input restrictions based on settingToEdit.Type (e.g. digits for int/float)
					m.currentEditValue += msg.String()
				}
			}
		} else { // Not editing setting OR not in settings tab
			// Session controls (P, R, A)
			if m.session != nil {
				sState, _, _, _, _, _ := m.session.GetStats()
				if sState == session.Running || sState == session.Paused {
					switch msg.String() {
					case "p":
						if sState == session.Running {
							if err := m.session.Pause(); err != nil {
								m.err = err
							} else {
								m.logMessages = append(m.logMessages, LogLevelWarnStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixWarn+" Pause command sent."))
							}
						}
					case "r":
						if sState == session.Paused {
							if err := m.session.Resume(); err != nil {
								m.err = err
							} else {
								m.logMessages = append(m.logMessages, LogLevelWarnStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixWarn+" Resume command sent."))
							}
						}
					case "a":
						if err := m.session.Abort(); err != nil {
							m.err = err
						} else {
							m.logMessages = append(m.logMessages, LogLevelWarnStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixWarn+" Abort command sent."))
						}
					}
				}
			}

			// Global keybindings
			switch msg.String() {
			case "ctrl+c", "q":
				// Quit logic (existing)
				if m.session != nil {
					sState, _, _, _, _, _ := m.session.GetStats()
					if sState == session.Running || sState == session.Paused {
						m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixWarn+" Session active. Press 'a' to abort session first, or Ctrl+C again to force quit."))
						if msg.String() == "q" && (m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.numReportsInput == "")) {
						}
					} else if m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.numReportsInput == "") {
						return m, tea.Quit
					}
				} else if m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.numReportsInput == "") {
					return m, tea.Quit
				}
				if m.activeTab == TargetInputTab && msg.String() == "q" {
					if m.inputFocus == 0 {
						m.targetURLInput += msg.String()
					}
				} else if msg.String() == "ctrl+c" {
					return m, tea.Quit
				}

			case "ctrl+n":
				m.activeTab = (m.activeTab + 1) % numTabs
				m.editingSetting = false
				m.settingsFocusIndex = 0
			case "ctrl+p":
				m.activeTab = (m.activeTab - 1 + numTabs) % numTabs
				m.editingSetting = false
				m.settingsFocusIndex = 0

			case "ctrl+s": // Save settings (only if on settings tab)
				if m.activeTab == SettingsTab {
					err := config.SaveAppConfig("config/sentinel.yaml", m.appConfig)
					ts := LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))
					if err != nil {
						m.err = fmt.Errorf("failed to save config: %w", err)
						m.logMessages = append(m.logMessages, ErrorTextStyle.Render(ts+" "+LogPrefixError+" Failed to save settings: "+err.Error()))
					} else {
						m.logMessages = append(m.logMessages, SuccessTextStyle.Render(ts+" "+LogPrefixInfo+" Settings saved successfully to config/sentinel.yaml."))
					}
				}
			case "ctrl+r": // Reload settings (only if on settings tab)
				if m.activeTab == SettingsTab {
					newCfg, err := config.LoadAppConfig("config/sentinel.yaml")
					ts := LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))
					if err != nil {
						m.err = fmt.Errorf("failed to reload config: %w", err)
						m.logMessages = append(m.logMessages, ErrorTextStyle.Render(ts+" "+LogPrefixError+" Failed to reload settings: "+err.Error()))
					} else {
						m.appConfig = newCfg
						m.populateEditableSettings() // Refresh the list in TUI
						m.logMessages = append(m.logMessages, SuccessTextStyle.Render(ts+" "+LogPrefixInfo+" Settings reloaded from config/sentinel.yaml."))
					}
				}

			// Tab-specific keybindings
			default:
				if m.activeTab == TargetInputTab {
					switch msg.String() {
					case "tab":
						m.inputFocus = (m.inputFocus + 1) % 2
					case "enter":
						numReportsInt, err := strconv.Atoi(m.numReportsInput)
						if err != nil || numReportsInt <= 0 {
							m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+" Invalid number of reports. Must be a positive integer."))
							m.err = fmt.Errorf("invalid number of reports: %s", m.numReportsInput)
						} else if m.targetURLInput == "" { // Validation
							m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+" Target URL cannot be empty."))
							m.err = fmt.Errorf("target URL cannot be empty")
						} else {
							currentSessionState := session.Idle
							if m.session != nil {
								currentSessionState, _, _, _, _, _ = m.session.GetStats()
							}
							if currentSessionState == session.Running || currentSessionState == session.Paused {
								m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+" A session is already active. Abort or wait."))
								m.err = fmt.Errorf("session already active")
							} else {
								m.session = session.NewSession(m.reporter, m.targetURLInput, numReportsInt)
								m.err = m.session.Start()
								if m.err != nil {
									m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+fmt.Sprintf(" Error starting session: %v", m.err)))
								} else {
									m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixInfo+fmt.Sprintf(" New session started for %d reports to %s.", numReportsInt, m.targetURLInput)))
									cmds = append(cmds, m.listenForSessionLogsCmd())
								}
								m.targetURLInput = ""
								m.inputFocus = 0
							}
						}
					case "backspace":
						if m.inputFocus == 0 && len(m.targetURLInput) > 0 {
							m.targetURLInput = m.targetURLInput[:len(m.targetURLInput)-1]
						}
						if m.inputFocus == 1 && len(m.numReportsInput) > 0 {
							m.numReportsInput = m.numReportsInput[:len(m.numReportsInput)-1]
						}
					default: // Character input
						if msg.Type == tea.KeyRunes && !strings.Contains(msg.String(), "ctrl+") {
							runeStr := msg.String()
							if m.inputFocus == 0 {
								m.targetURLInput += runeStr
							}
							if m.inputFocus == 1 {
								for _, r := range runeStr {
									if r >= '0' && r <= '9' {
										m.numReportsInput += string(r)
									}
								}
							}
						}
					}
				} else if m.activeTab == SettingsTab {
					switch msg.String() {
					case "up", "k":
						if !m.editingSetting && m.settingsFocusIndex > 0 {
							m.settingsFocusIndex--
						}
					case "down", "j":
						if !m.editingSetting && m.settingsFocusIndex < len(m.editableSettings)-1 {
							m.settingsFocusIndex++
						}
					case "enter":
						if !m.editingSetting && m.settingsFocusIndex < len(m.editableSettings) {
							m.editingSetting = true
							m.editingSettingPath = m.editableSettings[m.settingsFocusIndex].Path
							m.originalEditValue = m.editableSettings[m.settingsFocusIndex].CurrentValue
							m.currentEditValue = fmt.Sprintf("%v", m.originalEditValue)
							m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixInfo+fmt.Sprintf(" Editing '%s'...", m.editableSettings[m.settingsFocusIndex].Name)))
						}
						// Esc to cancel edit is handled when m.editingSetting is true
					}
				}
			}
		}
	}

	if m.session != nil {
		sState, _, numToSend, attempted, successful, failed := m.session.GetStats()
		m.sessionStatus = fmt.Sprintf("Session: %s | Target: %s | Reports: %d/%d | OK: %s | Fail: %s",
			sState.String(), m.session.TargetURL, attempted, numToSend,
			SuccessTextStyle.Render(fmt.Sprintf("%d", successful)), ErrorTextStyle.Render(fmt.Sprintf("%d", failed)))
	} else {
		m.sessionStatus = SubtleTextStyle.Render("Session: Idle")
	}
	return m, tea.Batch(cmds...)
}

// renderHeader, renderFooter, renderTabBar remain mostly the same...
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
		errorMsg := ErrorTextStyle.Render(SymbolFailure + " Error: " + m.err.Error())
		footerElements = append(footerElements, errorMsg)
	}
	footerElements = append(footerElements, HelpTextStyle.Render(helpFullString))
	return lipgloss.NewStyle().PaddingTop(1).Render(lipgloss.JoinVertical(lipgloss.Left, footerElements...))
}
func (m Model) renderTabBar() string {
	var renderedTabs []string
	for i, name := range m.tabsDisplayNames {
		style, prefix := TabStyle, SymbolNotFocused
		if Tab(i) == m.activeTab {
			style, prefix = ActiveTabStyle, SymbolActiveTab
		}
		renderedTabs = append(renderedTabs, style.Render(prefix+" "+name))
	}
	tabBarContainerStyle := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderTop(false).BorderLeft(false).BorderRight(false).BorderForeground(BorderColor).PaddingBottom(0)
	return tabBarContainerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Bottom, renderedTabs...))
}

// renderSettingsView generates the content for the Settings tab.
func (m Model) renderSettingsView() string {
	var content strings.Builder
	content.WriteString(HeaderStyle.Render(SymbolListItem+" Application Configuration") + "\n\n")

	if m.appConfig == nil {
		content.WriteString(WarningTextStyle.Render(SymbolWarning + " Application configuration not loaded."))
		return content.String()
	}

	// Refresh editableSettings from m.appConfig before rendering
	// This ensures that reloaded values are shown.
	// A more robust way would be to only update CurrentValue on successful save/reload.
	// m.populateEditableSettings() // Call this here or ensure it's called after reload/save in Update()

	for i, setting := range m.editableSettings {
		lineStyle := NormalTextStyle
		focusMarker := SymbolNotFocused + " "
		if !m.editingSetting && i == m.settingsFocusIndex {
			lineStyle = ActiveTabStyle.Copy().BorderStyle(lipgloss.HiddenBorder()) // Use active tab style for highlighting row
			focusMarker = SymbolFocused + " "
		}

		keyStr := keyStyle.Render(setting.Name + ":")
		var valueStr string
		if m.editingSetting && i == m.settingsFocusIndex {
			valueStr = FocusedInputStyle.Render(m.currentEditValue + "_") // Cursor for editing
		} else {
			currentValDisplay := fmt.Sprintf("%v", setting.CurrentValue)
			if setting.Type == "float" { // Ensure float formatting
				currentValDisplay = fmt.Sprintf("%.2f", setting.CurrentValue.(float64))
				if setting.Path == "RiskThreshold" {
					currentValDisplay += "%"
				}
			}
			if setting.IsSensitive {
				if len(currentValDisplay) > 8 {
					currentValDisplay = currentValDisplay[:4] + strings.Repeat("*", len(currentValDisplay)-8) + currentValDisplay[len(currentValDisplay)-4:]
				} else if len(currentValDisplay) > 0 {
					currentValDisplay = strings.Repeat("*", len(currentValDisplay))
				}
			}
			valueStr = valueStyle.Render(currentValDisplay)
		}

		content.WriteString(lineStyle.Render(focusMarker+keyStr+" "+valueStr) + "\n")
	}

	content.WriteString(HelpTextStyle.Render("\n\n(Navigate with ↑/↓, Enter to Edit/Confirm, Esc to Cancel. Ctrl+S to Save, Ctrl+R to Reload from file.)"))
	return content.String()
}

// View renders the TUI.
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
		if m.inputFocus == 0 {
			urlInputDisplay += "_"
			urlInputView = FocusedInputStyle.Render(SymbolFocused + " " + urlInputDisplay)
		} else {
			urlInputView = BlurredInputStyle.Render(SymbolNotFocused + " " + urlInputDisplay)
		}
		currentTabView.WriteString(urlLabel + "\n" + urlInputView + "\n\n")
		numReportsLabel := NormalTextStyle.Render(SymbolInputMarker + " Number of Reports")
		var numReportsInputView string
		numReportsInputDisplay := m.numReportsInput
		if m.inputFocus == 1 {
			numReportsInputDisplay += "_"
			numReportsInputView = FocusedInputStyle.Render(SymbolFocused + " " + numReportsInputDisplay)
		} else {
			numReportsInputView = BlurredInputStyle.Render(SymbolNotFocused + " " + numReportsInputDisplay)
		}
		currentTabView.WriteString(numReportsLabel + "\n" + numReportsInputView + "\n\n")
		helpText := "Tab: Switch Fields | Enter: Submit Report"
		if m.session != nil {
			sState, _, _, _, _, _ := m.session.GetStats()
			if sState == session.Running || sState == session.Paused {
				sessionHelp := lipgloss.JoinHorizontal(lipgloss.Left, HelpTextStyle.Render(SymbolPointer+" Session: "), helpKeyStyle.Bold(true).Render("P "), HelpTextStyle.Render("Pause | "), helpKeyStyle.Bold(true).Render("R "), HelpTextStyle.Render("Resume | "), helpKeyStyle.Bold(true).Render("A "), HelpTextStyle.Render("Abort"))
				helpText += " | " + sessionHelp
			}
		}
		currentTabView.WriteString(HelpTextStyle.Render(helpText) + "\n")
	case ProxyMgmtTab:
		currentTabView.WriteString(HeaderStyle.Render(SymbolListItem+" Proxy Pool Status") + "\n\n")
		if m.proxyManager != nil {
			allProxies := m.proxyManager.GetAllProxies()
			totalProxies := len(allProxies)
			healthyCount := 0
			unknownCount := 0
			for _, p := range allProxies {
				if p.HealthStatus == "healthy" {
					healthyCount++
				} else if p.HealthStatus == "unknown" {
					unknownCount++
				}
			}
			unhealthyCount := totalProxies - healthyCount - unknownCount
			statsStyle := NormalTextStyle.Copy().PaddingBottom(0)
			currentTabView.WriteString(statsStyle.Render(fmt.Sprintf("%s Total Proxies: %s", SymbolInfo, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", totalProxies)))) + "\n")
			currentTabView.WriteString(statsStyle.Render(fmt.Sprintf("%s Healthy:       %s", SymbolSuccess, SuccessTextStyle.Render(fmt.Sprintf("%d", healthyCount)))) + "\n")
			currentTabView.WriteString(statsStyle.Render(fmt.Sprintf("%s Unhealthy:     %s", SymbolFailure, ErrorTextStyle.Render(fmt.Sprintf("%d", unhealthyCount)))) + "\n")
			if unknownCount > 0 {
				currentTabView.WriteString(statsStyle.Render(fmt.Sprintf("%s Unknown:       %s", SymbolWarning, WarningTextStyle.Render(fmt.Sprintf("%d", unknownCount)))) + "\n")
			}
			currentTabView.WriteString("\n" + SubtleTextStyle.Render(SymbolInfo+" Initial health checks run in background. Statuses update over time.") + "\n")
		} else {
			currentTabView.WriteString(WarningTextStyle.Render(SymbolWarning+" Proxy Manager not initialized.") + "\n")
		}
		currentTabView.WriteString(HelpTextStyle.Render("\n(Detailed proxy list, manual checks, and import/export coming soon...)"))
	case SettingsTab:
		currentTabView.WriteString(m.renderSettingsView())
	case LiveSessionLogsTab:
		currentTabView.WriteString(HeaderStyle.Render(SymbolListItem+" Live Session Logs") + "\n")
		maxLogsToShow := m.height - 12
		if maxLogsToShow < 1 {
			maxLogsToShow = 5
		}
		displayLogs := m.logMessages
		if len(m.logMessages) > maxLogsToShow {
			displayLogs = m.logMessages[len(m.logMessages)-maxLogsToShow:]
		}
		for _, styledMsg := range displayLogs {
			currentTabView.WriteString(styledMsg + "\n")
		}
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
	if contentHeight < 0 {
		contentHeight = 0
	}
	contentBoxStyle := BoxStyle.Copy().MaxHeight(contentHeight).MaxWidth(m.width - BoxStyle.GetHorizontalBorderSize())
	finalView := lipgloss.JoinVertical(lipgloss.Left, headerView, tabBarView, contentBoxStyle.Render(currentTabView.String()), footerView)
	return AppStyle.Width(m.width).Height(m.height).Render(finalView)
}
