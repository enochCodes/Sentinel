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
// It now carries the structured LogUpdate from the session package.
type sessionLogMsg struct{ update session.LogUpdate }

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
	reasonInput      string
	logMessages      []string // Stores fully styled log strings for direct rendering
	inputFocus       int
	sessionStatus    string
}

// NewInitialModel creates the initial TUI model.
func NewInitialModel(cfg *config.AppConfig, logger *utils.Logger) Model {
	m := Model{
		activeTab:        TargetInputTab,
		tabsDisplayNames: tabNames,
		appConfig:        cfg,
		logger:           logger,
		logMessages:      []string{LogTimestampStyle.Render(time.Now().Format("15:04:05")) + " " + LogLevelInfoStyle.Render(LogPrefixInfo + " TUI Initialized. Welcome to SentinelGo!")},
		inputFocus:       0,
	}

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
			// This logging won't go to TUI's m.logMessages directly without a tea.Cmd/tea.Msg mechanism
			// It will go to the file logger used by BatchCheckProxies' fmt.Printf.
			// To update TUI: send a message back to tea.Program.Send(proxiesCheckedMsg{})
			// Note: This BatchCheckProxies directly logs to os.Stdout.
			// For TUI integration, it would ideally send tea.Msg back to the TUI.
			m.proxyManager.BatchCheckProxies(m.proxyManager.GetAllProxies(), checkTimeout, concurrency)
			m.logger.Info(utils.LogEntry{Message: "Initial batch proxy check completed."})
		}()
	}

	dummyAnalyzer := ai.NewDummyAnalyzer(logger)
	m.reporter = report.NewReporter(cfg, m.proxyManager, logger, dummyAnalyzer)
	m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogPrefixInfo+" Reporter initialized with Dummy AI Analyzer."))
	m.sessionStatus = SubtleTextStyle.Render("Session: Idle")

	return m
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
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	// Ensure m.err is cleared on any key press or new action, if desired.
	// This simple approach clears it on any KeyMsg. More sophisticated error handling might be needed.
	// if _, ok := msg.(tea.KeyMsg); ok {
	// 	m.err = nil
	// }


	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionLogMsg:
		logEntry := msg.update
		var styledLog string
		var logStyle lipgloss.Style
		var prefix string

		switch logEntry.Level {
		case session.LogLevelUpdateInfo:
			logStyle = LogLevelInfoStyle
			prefix = LogPrefixInfo
		case session.LogLevelUpdateError:
			logStyle = LogLevelErrorStyle
			prefix = LogPrefixError
		case session.LogLevelUpdateWarn:
			logStyle = LogLevelWarnStyle
			prefix = LogPrefixWarn
		case session.LogLevelUpdateDebug:
			logStyle = LogLevelDebugStyle
			prefix = LogPrefixDebug
		default:
			logStyle = NormalTextStyle
			prefix = "[???]"
		}

		timestampStr := LogTimestampStyle.Render(logEntry.Timestamp.Format("15:04:05.000"))
		// Apply the main level style to the prefix and the message for consistent coloring
		styledMsgPart := logStyle.Render(prefix + " " +logEntry.Message)
		styledLog = fmt.Sprintf("%s %s", timestampStr, styledMsgPart)

		m.logMessages = append(m.logMessages, styledLog)

		if logEntry.Message == "Session log channel closed by sender." {
			if m.session != nil {
				sState, total, processed, ok, fail := m.session.GetState()
				m.sessionStatus = fmt.Sprintf("Session: %s | Total: %d | Done: %d | OK: %s | Fail: %s",
					sState.String(), total, processed,
					SuccessTextStyle.Render(fmt.Sprintf("%d", ok)),
					ErrorTextStyle.Render(fmt.Sprintf("%d", fail)))
			} else {
				m.sessionStatus = ErrorTextStyle.Render("Session: ERROR - Log channel closed but session is nil")
			}
			return m, nil // Stop listening
		}
		cmds = append(cmds, m.listenForSessionLogsCmd())

	case tea.KeyMsg:
		m.err = nil // Clear previous error on new key press
		if m.session != nil {
			sState, _, _, _, _ := m.session.GetState()
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

		switch msg.String() {
		case "ctrl+c", "q":
			if m.session != nil {
				sState, _, _, _, _ := m.session.GetState()
				if sState == session.Running || sState == session.Paused {
					m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixWarn+" Session active. Press 'a' to abort session first, or Ctrl+C again to force quit."))
					if msg.String() == "q" && (m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.reasonInput == "")) {
						// return m, tea.Quit
					}
				} else if m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.reasonInput == "") {
					return m, tea.Quit
				}
			} else if m.activeTab != TargetInputTab || (m.targetURLInput == "" && m.reasonInput == "") {
				return m, tea.Quit
			}

			if m.activeTab == TargetInputTab && msg.String() == "q" {
				if m.inputFocus == 0 {
					m.targetURLInput += msg.String()
				} else {
					m.reasonInput += msg.String()
				}
			} else if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}

		case "ctrl+n":
			m.activeTab = (m.activeTab + 1) % numTabs
		case "ctrl+p":
			m.activeTab = (m.activeTab - 1 + numTabs) % numTabs

		case "tab":
			if m.activeTab == TargetInputTab {
				m.inputFocus = (m.inputFocus + 1) % 2
			}
		case "enter":
			if m.activeTab == TargetInputTab {
				if m.targetURLInput != "" && m.reasonInput != "" {
					currentSessionState := session.Idle
					if m.session != nil {
						currentSessionState, _,_,_,_ = m.session.GetState()
					}

					if currentSessionState == session.Running || currentSessionState == session.Paused {
						m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+" A session is already active. Abort or wait."))
						m.err = fmt.Errorf("session already active")
					} else {
						jobs := []struct{URL string; Reason string}{{URL: m.targetURLInput, Reason: m.reasonInput}}
						if m.session == nil || (currentSessionState != session.Idle && currentSessionState != session.Stopped) {
						    m.session = session.NewSession(m.reporter, jobs)
                        } else {
                            m.session.Jobs = make([]*session.ReportJob, len(jobs))
                            for i, ij := range jobs {
                                m.session.Jobs[i] = &session.ReportJob{
                                    ID: uuid.NewString(),
                                    TargetURL: ij.URL,
                                    Reason: ij.Reason,
                                    Status: "pending",
                                }
                            }
                        }
						m.err = m.session.Start()
						if m.err != nil {
							m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+fmt.Sprintf(" Error starting session: %v", m.err)))
						} else {
							m.logMessages = append(m.logMessages, LogLevelInfoStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixInfo+" New session started."))
							cmds = append(cmds, m.listenForSessionLogsCmd())
						}
						m.targetURLInput = ""
						m.reasonInput = ""
						m.inputFocus = 0
					}
				} else {
					m.logMessages = append(m.logMessages, ErrorTextStyle.Render(LogTimestampStyle.Render(time.Now().Format("15:04:05.000"))+" "+LogPrefixError+" Target URL and Reason cannot be empty."))
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

	if m.session != nil {
		sState, totalJobs, processed, successful, failed := m.session.GetState()
		m.sessionStatus = fmt.Sprintf("Session: %s | Total: %d | Done: %d | OK: %s | Fail: %s",
			sState.String(), totalJobs, processed,
			SuccessTextStyle.Render(fmt.Sprintf("%d", successful)),
			ErrorTextStyle.Render(fmt.Sprintf("%d", failed)))
	} else {
		m.sessionStatus = SubtleTextStyle.Render("Session: Idle")
	}

	return m, tea.Batch(cmds...)
}

// renderHeader creates the header view.
func (m Model) renderHeader() string {
	title := HeaderStyle.Render("SentinelGo++ Cyber Operations Platform")
	return title
}

// renderFooter creates the footer view with help text and errors.
func (m Model) renderFooter() string {
	helpKeyStyle := HelpTextStyle.Copy().Bold(true) // For key part of help

	helpParts := []string{
		helpKeyStyle.Render("Ctrl+C:") + HelpTextStyle.Render(" Quit"),
		helpKeyStyle.Render("Ctrl+N/P:") + HelpTextStyle.Render(" Nav Tabs"),
	}
	if m.session != nil {
		sState, _, _, _, _ := m.session.GetState()
		if sState == session.Running || sState == session.Paused {
			sessionHelp := fmt.Sprintf("%sP:%s Pause | %sR:%s Resume | %sA:%s Abort",
				helpKeyStyle.Render(""), HelpTextStyle.Render(""), // Symbols already in styles/consts
				helpKeyStyle.Render(""), HelpTextStyle.Render(""),
				helpKeyStyle.Render(""), HelpTextStyle.Render(""))
			// Using symbols directly in the string for P/R/A might be better:
			sessionHelp = fmt.Sprintf("P:%sPause | R:%sResume | A:%sAbort",
				HelpTextStyle.Render(""), HelpTextStyle.Render(""), HelpTextStyle.Render(""))
            sessionHelp = lipgloss.JoinHorizontal(lipgloss.Left,
                HelpTextStyle.Render(SymbolPointer+" Session: "),
                helpKeyStyle.Render("P "), HelpTextStyle.Render("Pause | "),
                helpKeyStyle.Render("R "), HelpTextStyle.Render("Resume | "),
                helpKeyStyle.Render("A "), HelpTextStyle.Render("Abort"),
            )

			helpParts = append(helpParts, sessionHelp)
		}
	}
	help := lipgloss.JoinHorizontal(lipgloss.Left, helpParts...)


	var footerElements []string
	if m.err != nil {
		// ErrorMessageStyle should already include SymbolFailure if desired
		errorMsg := ErrorMessageStyle.Render(SymbolFailure + " Error: " + m.err.Error())
		footerElements = append(footerElements, errorMsg)
	}
	footerElements = append(footerElements, HelpTextStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, helpParts...)))

	return lipgloss.NewStyle().PaddingTop(1).Render(
		lipgloss.JoinVertical(lipgloss.Left, footerElements...),
	)
}

// renderTabBar creates the tab bar view.
func (m Model) renderTabBar() string {
	var renderedTabs []string
	for i, name := range m.tabsDisplayNames {
		style := TabStyle
		prefix := SymbolNotFocused
		if Tab(i) == m.activeTab {
			style = ActiveTabStyle
			prefix = SymbolActiveTab
		}
		renderedTabs = append(renderedTabs, style.Render(prefix+" "+name))
	}
	tabBarContainerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).BorderTop(false).BorderLeft(false).BorderRight(false).
		BorderForeground(BorderColor).PaddingBottom(0)

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

	keyStyle := NormalTextStyle.Copy().Bold(true)
	subKeyStyle := NormalTextStyle.Copy() // No need for extra padding left, handled by indent string
	valueStyle := InfoTextStyle.Copy()
	indent := "  "

	renderKV := func(key string, value interface{}, isSensitive bool, currentIndent string, styleForKey lipgloss.Style) string {
		valStr := fmt.Sprintf("%v", value)
		if isSensitive {
			if len(valStr) > 8 {
				valStr = valStr[:4] + strings.Repeat("*", len(valStr)-8) + valStr[len(valStr)-4:]
			} else if len(valStr) > 0 {
				valStr = strings.Repeat("*", len(valStr))
			}
		}
		return currentIndent + styleForKey.Render(key+": ") + valueStyle.Render(valStr)
	}

	content.WriteString(renderKV("Max Retries", m.appConfig.MaxRetries, false, SymbolInfo+" ", keyStyle) + "\n")
	content.WriteString(renderKV("Risk Threshold", fmt.Sprintf("%.2f%%", m.appConfig.RiskThreshold), false, SymbolInfo+" ", keyStyle) + "\n\n")

	content.WriteString(keyStyle.Render(SymbolInfo+" Default Headers:") + "\n")
	if len(m.appConfig.DefaultHeaders) > 0 {
		for k, v := range m.appConfig.DefaultHeaders {
			content.WriteString(renderKV(k, v, false, indent+SymbolListSubItem+" ", subKeyStyle) + "\n")
		}
	} else {
		content.WriteString(indent + SubtleTextStyle.Render("No default headers configured.") + "\n")
	}
	content.WriteString("\n")

	content.WriteString(keyStyle.Render(SymbolInfo+" Custom Cookies:") + "\n")
	if len(m.appConfig.CustomCookies) > 0 {
		for i, cookie := range m.appConfig.CustomCookies {
			// Applying style directly to the cookie header line
			cookieHeader := NormalTextStyle.Render(fmt.Sprintf("%s Cookie #%d (%s):", indent+SymbolListSubItem, i+1, keyStyle.Render(cookie.Name)))
			content.WriteString(cookieHeader + "\n")

			cookieIndent := indent + indent + SymbolListSubItem + " "
			content.WriteString(renderKV("Value", cookie.Value, true, cookieIndent, subKeyStyle) + "\n")
			if cookie.Domain != "" {
				content.WriteString(renderKV("Domain", cookie.Domain, false, cookieIndent, subKeyStyle) + "\n")
			}
			if cookie.Path != "" {
				content.WriteString(renderKV("Path", cookie.Path, false, cookieIndent, subKeyStyle) + "\n")
			}
			if !cookie.Expires.IsZero() {
                 content.WriteString(renderKV("Expires", cookie.Expires.Format(time.RFC1123), false, cookieIndent, subKeyStyle) + "\n")
            }
             if cookie.HttpOnly {
                 content.WriteString(renderKV("HttpOnly", cookie.HttpOnly, false, cookieIndent, subKeyStyle) + "\n")
            }
            if cookie.Secure {
                 content.WriteString(renderKV("Secure", cookie.Secure, false, cookieIndent, subKeyStyle) + "\n")
            }
		}
	} else {
		content.WriteString(indent + SubtleTextStyle.Render("No custom cookies configured.") + "\n")
	}
	content.WriteString("\n")

	content.WriteString(keyStyle.Render(SymbolInfo+" API Keys:") + "\n")
	if len(m.appConfig.APIKeys) > 0 {
		for k, v := range m.appConfig.APIKeys {
			content.WriteString(renderKV(k, v, true, indent+SymbolListSubItem+" ", subKeyStyle) + "\n")
		}
	} else {
		content.WriteString(indent + SubtleTextStyle.Render("No API keys configured.") + "\n")
	}

	content.WriteString(HelpTextStyle.Render("\n\n(These settings are currently read-only in the TUI.)"))
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

		reasonLabel := NormalTextStyle.Render(SymbolInputMarker + " Report Reason")
		var reasonInputView string
		reasonInputDisplay := m.reasonInput
		if m.inputFocus == 1 {
			reasonInputDisplay += "_"
			reasonInputView = FocusedInputStyle.Render(SymbolFocused + " " + reasonInputDisplay)
		} else {
			reasonInputView = BlurredInputStyle.Render(SymbolNotFocused + " " + reasonInputDisplay)
		}
		currentTabView.WriteString(reasonLabel + "\n" + reasonInputView + "\n\n")

		helpText := "Tab: Switch Fields | Enter: Submit Report"
		if m.session != nil {
			sState,_,_,_,_ := m.session.GetState()
			if sState == session.Running || sState == session.Paused {
				helpTextParts := []string{
					HelpTextStyle.Render("Tab: Switch | Enter: Submit"),
					HelpTextStyle.Render(SymbolPointer+" Session:"),
					HelpTextStyle.Bold(true).Render("P"), HelpTextStyle.Render(" Pause | "),
					HelpTextStyle.Bold(true).Render("R"), HelpTextStyle.Render(" Resume | "),
					HelpTextStyle.Bold(true).Render("A"), HelpTextStyle.Render(" Abort"),
				}
				helpText = lipgloss.JoinHorizontal(lipgloss.Left, helpTextParts...)
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
				switch p.HealthStatus {
				case "healthy":
					healthyCount++
				case "unknown":
					unknownCount++
				}
			}
			unhealthyCount := totalProxies - healthyCount - unknownCount

			statsStyle := NormalTextStyle.Copy().PaddingBottom(0)

			currentTabView.WriteString(statsStyle.Render(
				fmt.Sprintf("%s Total Proxies: %s", SymbolInfo, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", totalProxies))),
			) + "\n")
			currentTabView.WriteString(statsStyle.Render(
				fmt.Sprintf("%s Healthy:       %s", SymbolSuccess, SuccessTextStyle.Render(fmt.Sprintf("%d", healthyCount))),
			) + "\n")
			currentTabView.WriteString(statsStyle.Render(
				fmt.Sprintf("%s Unhealthy:     %s", SymbolFailure, ErrorTextStyle.Render(fmt.Sprintf("%d", unhealthyCount))),
			) + "\n")
			if unknownCount > 0 {
				currentTabView.WriteString(statsStyle.Render(
					fmt.Sprintf("%s Unknown:       %s", SymbolWarning, WarningTextStyle.Render(fmt.Sprintf("%d", unknownCount))),
				) + "\n")
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
		if maxLogsToShow < 1 { maxLogsToShow = 5 }

		displayLogs := m.logMessages
		if len(m.logMessages) > maxLogsToShow {
			displayLogs = m.logMessages[len(m.logMessages)-maxLogsToShow:]
		}
		for _, styledMsg := range displayLogs {
			currentTabView.WriteString(styledMsg + "\n")
		}
	case LogReviewTab:
		currentTabView.WriteString(HeaderStyle.Render(SymbolListItem+" Log Review & Export") + "\n\n")

		mainMessage := InfoTextStyle.Render(
			fmt.Sprintf("%s Log review and advanced export functionalities are currently under development.", SymbolInfo),
		)
		additionalNote := HelpTextStyle.Render(
			fmt.Sprintf("All detailed session logs are being saved in JSON lines format to %s.",
				lipgloss.NewStyle().Foreground(PrimaryGreen).Render("sentinelgo_session.log"),
			),
		)
		comingSoon := SubtleTextStyle.Render("\n(More features coming soon!)")

		currentTabView.WriteString(lipgloss.NewStyle().PaddingBottom(1).Render(mainMessage) + "\n")
		currentTabView.WriteString(lipgloss.NewStyle().PaddingBottom(1).Render(additionalNote) + "\n")
		currentTabView.WriteString(comingSoon + "\n")

	}

	footerView := m.renderFooter()

	contentHeight := m.height - lipgloss.Height(headerView) - lipgloss.Height(tabBarView) - lipgloss.Height(footerView) - BoxStyle.GetVerticalPadding()
	if contentHeight < 0 { contentHeight = 0 }

	contentBoxStyle := BoxStyle.Copy().MaxHeight(contentHeight).MaxWidth(m.width - BoxStyle.GetHorizontalBorderSize())

	finalView := lipgloss.JoinVertical(
		lipgloss.Left,
		headerView,
		tabBarView,
		contentBoxStyle.Render(currentTabView.String()),
		footerView,
	)

	return AppStyle.Width(m.width).Height(m.height).Render(finalView)
}
