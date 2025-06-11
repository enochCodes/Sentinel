package tui

import "github.com/charmbracelet/lipgloss"

// UI Symbols & Icons
// These constants provide a consistent set of visual cues across the TUI.
const (
	// Prompts & Pointers
	SymbolPrompt      = "❯" // Heavy right-pointing angle quotation mark
	SymbolArrowRight  = "»" // Right-pointing double angle quotation mark
	SymbolPointer     = "➢" // Three-d perispex right arrow

	// Status Indicators
	SymbolSuccess     = "✔" // Heavy Check Mark (✓ is lighter)
	SymbolFailure     = "✘" // Heavy Ballot X (✗ is lighter)
	SymbolWarning     = "⚠" // Warning Sign
	SymbolInfo        = "ℹ" // Information Source
	SymbolRunning     = "…" // Horizontal Ellipsis (simple in-progress)
	SymbolPaused      = "⏸" // Pause symbol
	SymbolStopped     = "■" // Black Square (for stopped state)
	SymbolCompleted   = "🏁" // Chequered Flag

	// List Markers
	SymbolListItem    = "▪" // Black Small Square
	SymbolListSubItem = "•" // Bullet / Black Small Circle

	// Section Headers/Dividers (Box drawing characters)
	SymbolVertLine    = "┃" // Box Drawings Heavy Vertical
	SymbolHorizLine   = "━" // Box Drawings Heavy Horizontal
	SymbolTeeRight    = "┣" // Box Drawings Heavy Vertical and Right
	SymbolTeeLeft     = "┫" // Box Drawings Heavy Vertical and Left
	SymbolTeeDown     = "┳" // Box Drawings Heavy Horizontal and Down
	SymbolTeeUp       = "┻" // Box Drawings Heavy Horizontal and Up
	SymbolCornerTL    = "┏" // Box Drawings Heavy Down and Right
	SymbolCornerTR    = "┓" // Box Drawings Heavy Down and Left
	SymbolCornerBL    = "┗" // Box Drawings Heavy Up and Right
	SymbolCornerBR    = "┛" // Box Drawings Heavy Up and Left

	// Tab UI
	SymbolTabSeparator = "|"   // Vertical Line (can be styled)
	SymbolActiveTab  = "▶" // Black Right-Pointing Triangle (or use styling only)

	// Focus/Selection
	SymbolFocused    = "▸" // Black Right-Pointing Small Triangle
	SymbolNotFocused = " " // Space for alignment

	// Input Fields
	SymbolInputMarker = ":" // For marking input lines like "URL:"

	// Log Level Prefixes (can be combined with color styles)
	LogPrefixDebug   = "[DBG]"
	LogPrefixInfo    = "[INF]"
	LogPrefixWarn    = "[WRN]"
	LogPrefixError   = "[ERR]"
	LogPrefixFatal   = "[FTL]" // Should ideally not happen if Fatal exits
	LogPrefixTrace   = "[TRC]" // For very verbose debugging, if added
)


// Color Palette (Inspired by military/terminal themes)
// Using ANSI 256 color codes for wider compatibility.
// Reference: https://jonasjacek.github.io/colors/
var (
	// Base Colors
	BaseBackground = lipgloss.Color("235") // Dark Grey, almost black
	BaseText       = lipgloss.Color("252") // Light Grey / Off-white
	SubtleGreyText = lipgloss.Color("244") // Medium Grey

	// Primary/Accent Colors (Military Greens)
	PrimaryGreen    = lipgloss.Color("71")  // A medium, slightly desaturated green (like #5F875F)
	BrightGreen     = lipgloss.Color("83")  // A brighter green for highlights (#5FAF5F)
	DarkerGreen     = lipgloss.Color("22")  // Dark green, less saturated

	// Status Colors
	SuccessColor = lipgloss.Color("77")  // Bright Green, slightly different from Primary (#5FD75F)
	ErrorColor   = lipgloss.Color("160") // Bright Red (#D70000)
	WarningColor = lipgloss.Color("220") // Bright Yellow (#FFAF00)
	InfoColor    = lipgloss.Color("75")  // Bright Cyan/Blue (#5FDFFF)

	// Border and UI Element Colors
	BorderColor         = lipgloss.Color("240") // Medium-Dark Grey
	ActiveBorderColor   = PrimaryGreen
	FocusedBorderColor  = BrightGreen
	InactiveBorderColor = lipgloss.Color("238") // Darker Grey

	// Specific UI elements
	TabSeparatorColor = lipgloss.Color("238") // Dark Grey for tab separators
)

// General Application Styles
var (
	// AppStyle sets a base background for the entire application viewport.
	// Use this as the outermost style in tea.Program options if needed, or apply to main view.
	AppStyle = lipgloss.NewStyle().
		// Background(BaseBackground). // Set globally if renderer supports it well or on main view
		Foreground(BaseText)

	// HeaderStyle for section titles within views.
	HeaderStyle = lipgloss.NewStyle().
		Foreground(PrimaryGreen).
		Bold(true).
		MarginBottom(1)

	// NormalTextStyle for general body text.
	NormalTextStyle = lipgloss.NewStyle().
		Foreground(BaseText)

	// SubtleTextStyle for less important information or disabled elements.
	SubtleTextStyle = lipgloss.NewStyle().
		Foreground(SubtleGreyText)

	// HelpTextStyle for keybinding hints and footer text.
	HelpTextStyle = lipgloss.NewStyle().
		Foreground(SubtleGreyText).
		PaddingTop(1)
)

// Tab Styles
var (
	// TabStyle is for an individual, inactive tab.
	TabStyle = lipgloss.NewStyle().
		Foreground(SubtleGreyText).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderBottom(true).BorderTop(false).BorderLeft(false).BorderRight(false). // Only bottom border
		BorderForeground(InactiveBorderColor)


	// ActiveTabStyle is for the currently selected tab.
	ActiveTabStyle = TabStyle.Copy().
		Foreground(PrimaryGreen).
		BorderForeground(ActiveBorderColor).
		Bold(true)

	// TabSeparator defines the style for the " | " between tabs.
    TabSeparator = lipgloss.NewStyle().
        Foreground(TabSeparatorColor).
        Padding(0,1)
)

// Input Field Styles
var (
	// FocusedInputStyle for text input fields that have focus.
	FocusedInputStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(FocusedBorderColor).
		Foreground(BaseText).
		Padding(0, 1)
		// Background(lipgloss.Color("237")) // Slightly lighter background for focused input

	// BlurredInputStyle for text input fields that do not have focus.
	BlurredInputStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(BorderColor).
		Foreground(SubtleGreyText). // Text is more subtle when blurred
		Padding(0, 1)
		// Background(lipgloss.Color("236"))
)

// Status Message Styles
var (
	SuccessTextStyle = lipgloss.NewStyle().
		Foreground(SuccessColor).
		Bold(true)

	ErrorTextStyle = lipgloss.NewStyle().
		Foreground(ErrorColor).
		Bold(true)

	WarningTextStyle = lipgloss.NewStyle().
		Foreground(WarningColor).
		Bold(true)

	InfoTextStyle = lipgloss.NewStyle().
		Foreground(InfoColor)
)

// Log Specific Styles
var (
	LogTimestampStyle = lipgloss.NewStyle().Foreground(SubtleGreyText)
	LogLevelDebugStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Dimmed
	LogLevelInfoStyle  = lipgloss.NewStyle().Foreground(InfoColor)
	LogLevelWarnStyle  = lipgloss.NewStyle().Foreground(WarningColor)
	LogLevelErrorStyle = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
	LogMessageStyle    = lipgloss.NewStyle().Foreground(BaseText)
	LogProxyStyle      = lipgloss.NewStyle().Foreground(PrimaryGreen)
	LogOutcomeSuccessStyle = lipgloss.NewStyle().Foreground(SuccessColor)
	LogOutcomeFailureStyle = lipgloss.NewStyle().Foreground(ErrorColor)
)

// Container/Box Styles
var (
    // General purpose box with a border
    BoxStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(BorderColor).
        Padding(1, 2) // Add some padding inside the box

	ActiveBoxStyle = BoxStyle.Copy().
		BorderForeground(ActiveBorderColor)
)
