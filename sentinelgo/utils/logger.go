package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os" // Required for os.Exit in Fatal, though currently commented out.
	"sync"
	"time"
)

// LogLevel defines the severity level for log messages.
type LogLevel int

// Constants for predefined log levels.
// These determine the verbosity and importance of log messages.
const (
	LevelDebug LogLevel = iota // Debug: Detailed information for diagnosing problems.
	LevelInfo                  // Info: General operational information.
	LevelWarn                  // Warn: Potential issues or unusual occurrences.
	LevelError                 // Error: Errors that occurred but allow the application to continue.
	LevelFatal                 // Fatal: Severe errors causing the application to terminate.
)

// levelToString maps LogLevel enum values to their string representations (e.g., "INFO", "ERROR").
var levelToString = map[LogLevel]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
	LevelFatal: "FATAL",
}

// stringToLevel maps string representations of log levels to LogLevel enum values.
// This is useful for parsing log levels from configuration.
var stringToLevel = map[string]LogLevel{
	"DEBUG": LevelDebug,
	"INFO":  LevelInfo,
	"WARN":  LevelWarn,
	"ERROR": LevelError,
	"FATAL": LevelFatal,
}

// LogEntry represents a single structured log record.
// It's designed to be marshaled into JSON format for logging.
// Fields are tagged with `json:"...,omitempty"` to exclude them from JSON output if they are zero-valued.
type LogEntry struct {
	Timestamp       string                 `json:"timestamp"`                      // ISO 8601 format (UTC) timestamp of the log event.
	Level           string                 `json:"level"`                          // Severity level of the log (e.g., "INFO", "ERROR").
	Message         string                 `json:"message"`                        // The main log message.
	SessionID       string                 `json:"session_id,omitempty"`         // ID of the reporting session, if applicable.
	ReportURL       string                 `json:"report_url,omitempty"`         // Target URL of the report, if applicable.
	Proxy           string                 `json:"proxy,omitempty"`              // Proxy used for the request, if applicable.
	UserAgent       string                 `json:"user_agent,omitempty"`         // User-Agent string used for the request.
	RequestMethod   string                 `json:"request_method,omitempty"`     // HTTP request method (e.g., "POST", "GET").
	RequestHeaders  http.Header            `json:"request_headers,omitempty"`    // HTTP headers sent with the request.
	RequestBody     string                 `json:"request_body,omitempty"`       // Body of the HTTP request (may be summarized or omitted for large/sensitive data).
	ResponseStatus  int                    `json:"response_status,omitempty"`    // HTTP status code received in the response.
	ResponseHeaders http.Header            `json:"response_headers,omitempty"`   // HTTP headers received in the response.
	ResponseBody    string                 `json:"response_body,omitempty"`      // Body of the HTTP response (may be summarized or omitted).
	Outcome         string                 `json:"outcome,omitempty"`            // Result of an operation (e.g., "accepted", "failed", "retrying").
	LogID           string                 `json:"log_id,omitempty"`             // Log ID from an external service (e.g., TikTok response header).
	Error           string                 `json:"error,omitempty"`              // Error message if an error occurred.
	AdditionalData  map[string]interface{} `json:"additional_data,omitempty"`    // A map for any other contextual data relevant to the log entry.
}

// Logger provides a structured JSON logger that writes log entries to an io.Writer.
// It supports different log levels and ensures thread-safe write operations.
type Logger struct {
	writer   io.Writer // Destination for log output (e.g., os.Stdout, a file).
	minLevel LogLevel  // Minimum log level to output; messages below this level are suppressed.
	mu       sync.Mutex // Mutex to ensure thread-safe writes to the writer.
}

// NewLogger creates and returns a new Logger instance.
//
// Parameters:
//   - writer: The io.Writer where log entries will be written (e.g., os.Stdout, a file).
//   - minLevelStr: The minimum log level as a string (e.g., "INFO", "DEBUG").
//     If an invalid string is provided, it defaults to LevelInfo.
func NewLogger(writer io.Writer, minLevelStr string) *Logger {
	level, ok := stringToLevel[strings.ToUpper(minLevelStr)] // Ensure case-insensitivity for level string.
	if !ok {
		level = LevelInfo // Default to INFO if the provided string is invalid.
	}
	return &Logger{
		writer:   writer,
		minLevel: level,
	}
}

// Log writes a LogEntry at the specified LogLevel if the level is at or above the Logger's minimum level.
// The LogEntry is augmented with a timestamp and string representation of the level before being
// marshaled to JSON and written to the Logger's io.Writer.
// Writes are thread-safe. If JSON marshaling fails, a fallback plain text error is logged.
// If writing to the primary writer fails, the error is written to io.Discard.
func (l *Logger) Log(level LogLevel, entry LogEntry) {
	if level < l.minLevel {
		return // Suppress messages below the minimum level.
	}

	// Populate standard fields.
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano) // High-precision timestamp.
	entry.Level = levelToString[level]

	l.mu.Lock() // Ensure thread-safe write to the output.
	defer l.mu.Unlock()

	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to a simple error log if marshaling fails, to avoid losing error information.
		// This fallback log is also JSON-like for some consistency.
		fmt.Fprintf(l.writer, "{\"timestamp\":\"%s\",\"level\":\"ERROR\",\"message\":\"Failed to marshal log entry\",\"original_level\":\"%s\",\"error\":\"%s\"}\n",
			time.Now().UTC().Format(time.RFC3339Nano), entry.Level, err.Error())
		return
	}

	// Write the JSON data followed by a newline (JSON Lines format).
	if _, err = fmt.Fprintln(l.writer, string(jsonData)); err != nil {
		// If writing to the primary writer fails, attempt to write an error message to io.Discard
		// to avoid crashing or polluting stderr directly from library code.
		fmt.Fprintf(io.Discard, "{\"timestamp\":\"%s\",\"level\":\"ERROR\",\"message\":\"Failed to write log entry to primary writer\",\"error\":\"%s\"}\n",
			time.Now().UTC().Format(time.RFC3339Nano), err.Error())
	}
}

// Debug logs a message at LevelDebug.
func (l *Logger) Debug(entry LogEntry) {
	l.Log(LevelDebug, entry)
}

// Info logs a message at LevelInfo.
func (l *Logger) Info(entry LogEntry) {
	l.Log(LevelInfo, entry)
}

// Warn logs a message at LevelWarn.
func (l *Logger) Warn(entry LogEntry) {
	l.Log(LevelWarn, entry)
}

// Error logs a message at LevelError.
func (l *Logger) Error(entry LogEntry) {
	l.Log(LevelError, entry)
}

// Fatal logs a message at LevelFatal.
// IMPORTANT: In its current commented-out form, it logs as LevelError to prevent os.Exit(1)
// from being called directly within library code, which is generally safer.
// If true fatal behavior (program termination) is required, the os.Exit(1) call should be
// managed by the main application after logging, or this method should be uncommented carefully.
func (l *Logger) Fatal(entry LogEntry) {
	entry.Message = fmt.Sprintf("FATAL: %s", entry.Message) // Prepend "FATAL:" to the message for emphasis.
	l.Log(LevelFatal, entry) // Log with LevelFatal, which will be written if minLevel allows.
	// If os.Exit is truly needed here (use with caution in library code):
	// os.Exit(1)
}

// CreateLogEntry is a helper function to quickly create a basic LogEntry struct.
// This is useful for simple logging scenarios where only a few fields are needed.
//
// Parameters:
//   - message: The main log message.
//   - reportURL: The target URL of the report, if applicable. Can be empty.
//   - proxy: The proxy used for the request, if applicable. Can be empty.
//   - outcome: The result of an operation (e.g., "success", "failure"). Can be empty.
//   - err: An error object, if an error occurred. Its message will be stored. Can be nil.
//
// Returns:
//   A LogEntry struct populated with the provided fields.
func CreateLogEntry(message string, reportURL string, proxy string, outcome string, err error) LogEntry {
	entry := LogEntry{
		Message:   message,
		ReportURL: reportURL,
		Proxy:     proxy,
		Outcome:   outcome,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	return entry
}
