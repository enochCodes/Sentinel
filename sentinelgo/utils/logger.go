package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// LogLevel type for defining log levels
type LogLevel int

// Log levels constants
const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal // Will os.Exit(1)
)

// levelToString maps LogLevel to string representation
var levelToString = map[LogLevel]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
	LevelFatal: "FATAL",
}

// stringToLevel maps string representation to LogLevel
var stringToLevel = map[string]LogLevel{
	"DEBUG": LevelDebug,
	"INFO":  LevelInfo,
	"WARN":  LevelWarn,
	"ERROR": LevelError,
	"FATAL": LevelFatal,
}

// LogEntry represents a single structured log record.
type LogEntry struct {
	Timestamp       string                 `json:"timestamp"`
	Level           string                 `json:"level"`
	Message         string                 `json:"message"`
	SessionID       string                 `json:"session_id,omitempty"`
	ReportURL       string                 `json:"report_url,omitempty"`
	Proxy           string                 `json:"proxy,omitempty"`
	UserAgent       string                 `json:"user_agent,omitempty"`
	RequestMethod   string                 `json:"request_method,omitempty"`
	RequestHeaders  http.Header            `json:"request_headers,omitempty"`
	RequestBody     string                 `json:"request_body,omitempty"` // Consider summarizing for large bodies
	ResponseStatus  int                    `json:"response_status,omitempty"`
	ResponseHeaders http.Header            `json:"response_headers,omitempty"`
	ResponseBody    string                 `json:"response_body,omitempty"` // Consider summarizing
	Outcome         string                 `json:"outcome,omitempty"`
	LogID           string                 `json:"log_id,omitempty"` // e.g., from TikTok response
	Error           string                 `json:"error,omitempty"`
	AdditionalData  map[string]interface{} `json:"additional_data,omitempty"`
}

// Logger provides structured JSON logging.
type Logger struct {
	writer   io.Writer
	minLevel LogLevel
	mu       sync.Mutex
}

// NewLogger creates a new Logger instance.
// minLevelStr should be one of "DEBUG", "INFO", "WARN", "ERROR", "FATAL".
func NewLogger(writer io.Writer, minLevelStr string) *Logger {
	level, ok := stringToLevel[minLevelStr]
	if !ok {
		level = LevelInfo // Default to INFO if invalid level string is provided
	}
	return &Logger{
		writer:   writer,
		minLevel: level,
	}
}

// Log creates a new log entry with the given level and entry details.
func (l *Logger) Log(level LogLevel, entry LogEntry) {
	if level < l.minLevel {
		return
	}

	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano) // Use RFC3339Nano for more precision
	entry.Level = levelToString[level]

	l.mu.Lock()
	defer l.mu.Unlock()

	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to plain text logging if JSON marshaling fails
		fmt.Fprintf(l.writer, "{\"timestamp\":\"%s\",\"level\":\"ERROR\",\"message\":\"Failed to marshal log entry\",\"error\":\"%s\"}\n", time.Now().UTC().Format(time.RFC3339Nano), err.Error())
		return
	}

	_, err = fmt.Fprintln(l.writer, string(jsonData)) // Fprintln adds a newline
	if err != nil {
		// If writing to the primary writer fails, try to write error to stderr
		fmt.Fprintf(io.Discard, "Failed to write log entry: %v\n", err) // Changed from os.Stderr to io.Discard
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

// Fatal logs a message at LevelFatal and then calls os.Exit(1).
// For now, we will just log as error, as os.Exit behavior might be too abrupt for the tool.
// Actual os.Exit can be added if specifically required later.
func (l *Logger) Fatal(entry LogEntry) {
    entry.Message = fmt.Sprintf("FATAL: %s", entry.Message) // Prepend FATAL to message
	l.Log(LevelError, entry) // Log as LevelError to avoid os.Exit in library code
	// If os.Exit is truly needed:
	// l.Log(LevelFatal, entry)
	// os.Exit(1)
}

// Helper to create a basic entry - can be expanded
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
