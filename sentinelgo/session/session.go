package session

import (
	"fmt"
	"sync"
	"time"
	"github.com/google/uuid"

	"sentinelgo/report"
)

// LogLevelUpdate defines the severity level for LogUpdate messages.
const (
	LogLevelUpdateInfo  = "INFO"
	LogLevelUpdateError = "ERROR"
	LogLevelUpdateWarn  = "WARN"
	LogLevelUpdateDebug = "DEBUG"
)

// LogUpdate is the structured message sent over LogChannel for TUI updates.
type LogUpdate struct {
	Level     string
	Message   string
	Timestamp time.Time
}

// SessionState defines the possible states of a reporting session.
type SessionState int

const (
	Idle SessionState = iota
	Running
	Paused
	Stopping
	Stopped
	Completed
	Aborted
	Failed
)

// String returns the string representation of SessionState.
func (s SessionState) String() string {
	switch s {
	case Idle: return "Idle"
	case Running: return "Running"
	case Paused: return "Paused"
	case Stopping: return "Stopping"
	case Stopped: return "Stopped"
	case Completed: return "Completed"
	case Aborted: return "Aborted"
	case Failed: return "Failed (Internal Error)"
	default: return fmt.Sprintf("Unknown State (%d)", s)
	}
}

// ReportJob defines a single reporting task attempt.
type ReportJob struct {
	ID            string
	ReportNumber  int      // 1-based index for this report in the session
	Status        string   // e.g., "pending", "processing", "success", "failed"
	LogID         string   // From report response, if any (TODO: reporter needs to return this)
	Error         string
	StartTime     time.Time
	EndTime       time.Time
	// Attempts field from previous might be redundant if Reporter handles retries for a single SendReport call
}

// Session manages a reporting session to a single target URL for a number of reports.
type Session struct {
	ID                string
	State             SessionState
	Reporter          *report.Reporter

	TargetURL         string
	NumReportsToSend  int
	Jobs              []*ReportJob // Stores each of the N reports

	ReportsAttemptedCount int // Number of reports (0 to N-1) for which processing has started
	SuccessfulReports   int
	FailedReports       int

	StartTime         time.Time
	EndTime           time.Time
	ProxiesUsed       map[string]int // TODO: Populate this

	LogChannel        chan LogUpdate
	controlChannel    chan string

	wg                sync.WaitGroup
	mu                sync.Mutex
}

// NewSession creates a new reporting session for a single target URL and a specified number of reports.
func NewSession(reporter *report.Reporter, targetURL string, numReportsToSend int) *Session {
	jobs := make([]*ReportJob, numReportsToSend)
	for i := 0; i < numReportsToSend; i++ {
		jobs[i] = &ReportJob{
			ID:           uuid.NewString(),
			ReportNumber: i + 1,
			Status:       "pending",
		}
	}

	return &Session{
		ID:               uuid.NewString(),
		State:            Idle,
		Reporter:         reporter,
		TargetURL:        targetURL,
		NumReportsToSend: numReportsToSend,
		Jobs:             jobs,
		ProxiesUsed:      make(map[string]int),
		LogChannel:       make(chan LogUpdate, 100),
		controlChannel:   make(chan string, 10),
	}
}

func (s *Session) sendLog(level string, message string) {
	// Non-blocking send attempt with timeout to prevent deadlock if channel is full and no receiver.
	// This is a safety measure; ideally, TUI consumes fast enough or buffer is sufficient.
	select {
	case s.LogChannel <- LogUpdate{Level: level, Message: message, Timestamp: time.Now()}:
	case <-time.After(1 * time.Second): // Timeout if LogChannel is blocked
		fmt.Printf("LogChannel send timeout: %s: %s\n", level, message) // Fallback log
	}
}

// Start begins processing the report jobs in a new goroutine.
func (s *Session) Start() error {
	s.mu.Lock()
	if s.State != Idle && s.State != Stopped && s.State != Completed && s.State != Aborted {
		s.mu.Unlock()
		return fmt.Errorf("session cannot be started from state: %s", s.State)
	}

	s.State = Running
	s.StartTime = time.Now()
	s.EndTime = time.Time{}

	// Reset counters and job statuses if this is a restart of a completed/aborted session
	if s.ReportsAttemptedCount >= s.NumReportsToSend || s.State == Completed || s.State == Aborted {
		s.ReportsAttemptedCount = 0
		s.SuccessfulReports = 0
		s.FailedReports = 0
		for i := 0; i < s.NumReportsToSend; i++ {
			s.Jobs[i].Status = "pending"
			s.Jobs[i].Error = ""
			s.Jobs[i].LogID = ""
		}
	}
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runLoop()
	s.sendLog(LogLevelUpdateInfo, fmt.Sprintf("Session %s started for %d reports to %s.", s.ID, s.NumReportsToSend, s.TargetURL))
	return nil
}

// runLoop is the main goroutine for processing report jobs.
func (s *Session) runLoop() {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		if r := recover(); r != nil {
			s.sendLog(LogLevelUpdateError, fmt.Sprintf("FATAL: Session runLoop panicked: %v", r))
			s.State = Failed
		}

		finalState := s.State
		if finalState != Aborted && finalState != Failed {
			if s.ReportsAttemptedCount >= s.NumReportsToSend {
				s.State = Completed
				s.sendLog(LogLevelUpdateInfo, "Session completed: All reports processed.")
			} else if finalState != Paused {
				s.State = Stopped
				s.sendLog(LogLevelUpdateWarn, "Session stopped before completion.")
			}
		} else if finalState == Aborted {
			s.sendLog(LogLevelUpdateWarn, "Session processing aborted by user.")
		}

		s.EndTime = time.Now()
		s.mu.Unlock()
		close(s.LogChannel)
	}()

	for s.ReportsAttemptedCount < s.NumReportsToSend {
		s.mu.Lock()
		currentState := s.State
		currentJob := s.Jobs[s.ReportsAttemptedCount]
		s.mu.Unlock()

		select {
		case cmd := <-s.controlChannel:
			s.mu.Lock()
			switch cmd {
			case "pause":
				if s.State == Running {
					s.State = Paused
					s.sendLog(LogLevelUpdateWarn, "Session paused.")
				}
				s.mu.Unlock()
				for pausedCmd := range s.controlChannel { // Block here
					s.mu.Lock()
					if pausedCmd == "resume" {
						if s.State == Paused {
							s.State = Running
							s.sendLog(LogLevelUpdateWarn, "Session resumed.")
						}
						s.mu.Unlock()
						break
					} else if pausedCmd == "abort" {
						s.State = Aborted
						s.mu.Unlock()
						return
					}
					s.sendLog(LogLevelUpdateWarn, fmt.Sprintf("Invalid cmd '%s' while paused.", pausedCmd))
					s.mu.Unlock()
				}
				s.mu.Lock() // Re-lock after breaking from inner pause loop
				if s.State == Paused { // If controlChannel closed while paused
					s.State = Aborted
				}
				s.mu.Unlock()
				if s.GetStateValue() == Aborted { return } // Use GetStateValue for thread-safe read
				continue
			case "abort":
				s.State = Aborted
				s.mu.Unlock()
				return
			default:
				s.sendLog(LogLevelUpdateWarn, fmt.Sprintf("Unknown control command: %s", cmd))
				s.mu.Unlock()
			}
		default:
			// No control message
		}

		s.mu.Lock()
		if s.State != Running {
			s.mu.Unlock()
			if currentState == Paused { // Use captured state for this check
				 time.Sleep(100 * time.Millisecond)
			}
			continue
		}
		s.mu.Unlock()

		currentJob.Status = "processing"
		currentJob.StartTime = time.Now()
		s.sendLog(LogLevelUpdateInfo, fmt.Sprintf("Report %d/%d to %s -> Sending...", currentJob.ReportNumber, s.NumReportsToSend, s.TargetURL))

		// SendReport's second argument (reportReason) is now effectively unused by the reporter.
		// The session ID is passed for logging context within the reporter.
		reportErr := s.Reporter.SendReport(s.TargetURL, "" /* reason no longer used */, s.ID)
		currentJob.EndTime = time.Now()

		s.mu.Lock()
		if reportErr != nil {
			currentJob.Status = "failed"
			currentJob.Error = reportErr.Error()
			s.FailedReports++
			s.sendLog(LogLevelUpdateError, fmt.Sprintf("Report %d/%d to %s -> Failed: %s", currentJob.ReportNumber, s.NumReportsToSend, s.TargetURL, reportErr.Error()))
		} else {
			currentJob.Status = "success"
			s.SuccessfulReports++
			s.sendLog(LogLevelUpdateInfo, fmt.Sprintf("Report %d/%d to %s -> Success.", currentJob.ReportNumber, s.NumReportsToSend, s.TargetURL))
			// TODO: currentJob.LogID = reporter needs to return this value from SendReport
		}
		s.ReportsAttemptedCount++
		s.mu.Unlock()
	}
}

func (s *Session) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != Running {
		return fmt.Errorf("session not running (state: %s)", s.State)
	}
	s.controlChannel <- "pause"
	return nil
}

func (s *Session) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != Paused {
		return fmt.Errorf("session not paused (state: %s)", s.State)
	}
	s.controlChannel <- "resume"
	return nil
}

func (s *Session) Abort() error {
	s.mu.Lock()
	if s.State == Completed || s.State == Aborted || s.State == Failed || s.State == Idle {
		s.mu.Unlock()
		return nil
	}

	isAlreadyStoppingOrAborting := (s.State == Stopping || s.State == Aborted)
	s.State = Stopping
	s.mu.Unlock()

	if !isAlreadyStoppingOrAborting {
		s.sendLog(LogLevelUpdateWarn, "Abort signal sent to session.")
		s.controlChannel <- "abort"
	}

	waitTimeout := time.NewTimer(10 * time.Second)
	defer waitTimeout.Stop()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.mu.Lock()
		s.State = Aborted
		if s.EndTime.IsZero() { s.EndTime = time.Now() }
		s.mu.Unlock()
	case <-waitTimeout.C:
		s.sendLog(LogLevelUpdateError, "Timeout waiting for session to abort.")
		// State might remain Stopping if runLoop is stuck
		return fmt.Errorf("timeout waiting for session to abort")
	}
	return nil
}

// GetStateValue is a thread-safe way to get the current state.
func (s *Session) GetStateValue() SessionState {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.State
}


// GetSummary provides a string summary of the session.
func (s *Session) GetSummary() string { // For TUI display, perhaps not for detailed logging
	s.mu.Lock()
	defer s.mu.Unlock()
	var duration time.Duration
	if !s.StartTime.IsZero() {
		if s.EndTime.IsZero() || s.State == Running || s.State == Paused {
			duration = time.Since(s.StartTime)
		} else {
			duration = s.EndTime.Sub(s.StartTime)
		}
	}
	return fmt.Sprintf("ID: %s | State: %s | Target: %s | Reports: %d/%d | Success: %d | Fail: %d | Duration: %s",
		s.ID, s.State.String(), s.TargetURL, s.ReportsAttemptedCount, s.NumReportsToSend, s.SuccessfulReports, s.FailedReports, duration.Round(time.Second).String())
}

// GetStats returns current session statistics.
func (s *Session) GetStats() (currentState SessionState, targetURL string, numToSend int, attempted int, successful int, failed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State, s.TargetURL, s.NumReportsToSend, s.ReportsAttemptedCount, s.SuccessfulReports, s.FailedReports
}

// EnsureLogChannelClosed is a helper that might be called by TUI when it's certain it's done with a session.
// The primary close is in runLoop's defer.
func (s *Session) EnsureLogChannelClosed() {
    // This method is tricky because of potential race conditions on closing LogChannel.
    // The runLoop's defer func is the most reliable place to close LogChannel.
    // TUI should handle reads from a closed LogChannel gracefully.
}
