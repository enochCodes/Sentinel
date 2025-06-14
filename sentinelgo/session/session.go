package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"sentinelgo/sentinelgo/report"
)

// LogLevelUpdate defines the severity level for LogUpdate messages sent to the TUI.
const (
	LogLevelUpdateInfo  = "INFO"  // Informational messages.
	LogLevelUpdateError = "ERROR" // Error messages.
	LogLevelUpdateWarn  = "WARN"  // Warning messages.
	LogLevelUpdateDebug = "DEBUG" // Debug messages (if session needs to send verbose updates).
)

// LogUpdate is the structured message sent over the Session's LogChannel.
// It provides real-time updates to listeners (e.g., the TUI) about the session's progress and status.
type LogUpdate struct {
	Level     string    // Severity level of the log update (e.g., LogLevelUpdateInfo).
	Message   string    // The content of the log message.
	Timestamp time.Time // Timestamp when the log update was generated.
}

// SessionState defines the possible operational states of a reporting session.
type SessionState int

// Constants defining the specific states a Session can be in.
const (
	Idle      SessionState = iota // Session created but not yet started.
	Running                       // Session is actively processing reports.
	Paused                        // Session is temporarily paused by user command.
	Stopping                      // Session is in the process of stopping due to an abort command.
	Stopped                       // Session was stopped before completing all reports (e.g., by abort, or if loop exited unexpectedly without completion).
	Completed                     // Session successfully processed all its reports.
	Aborted                       // Session was explicitly aborted by user command, not all reports may have been processed.
	Failed                        // Session encountered a critical internal error (e.g., panic in runLoop).
)

// String returns a human-readable string representation of the SessionState.
func (s SessionState) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Running:
		return "Running"
	case Paused:
		return "Paused"
	case Stopping:
		return "Stopping"
	case Stopped:
		return "Stopped"
	case Completed:
		return "Completed"
	case Aborted:
		return "Aborted"
	case Failed:
		return "Failed (Internal Error)"
	default:
		return fmt.Sprintf("Unknown State (%d)", s)
	}
}

// ReportJob represents a single report attempt within a session.
// Since a session now targets one URL for N reports, each of these N reports is a ReportJob.
type ReportJob struct {
	ID           string    // Unique identifier for this specific report job.
	ReportNumber int       // 1-based sequence number of this report within the session (e.g., 1 of N).
	Status       string    // Current status of this job (e.g., "pending", "processing", "success", "failed").
	LogID        string    // Log identifier received from the target platform's response (if any). TODO: reporter.SendReport needs to return this.
	Error        string    // Error message if this specific report job failed.
	StartTime    time.Time // Timestamp when processing for this job started.
	EndTime      time.Time // Timestamp when processing for this job ended.
}

// Session manages the overall process of sending a configured number of reports
// to a single target URL. It handles state (running, paused, etc.), tracks progress,
// and communicates updates via its LogChannel.
type Session struct {
	ID       string           // Unique identifier for the session.
	State    SessionState     // Current operational state of the session.
	Reporter *report.Reporter // The reporter instance used to send individual reports.

	TargetURL        string       // The URL targeted by this session.
	NumReportsToSend int          // Total number of reports to send in this session.
	Jobs             []*ReportJob // Slice holding each of the N report jobs.

	ReportsAttemptedCount int // How many reports (0 to N-1 index) have begun processing.
	SuccessfulReports     int // Count of successfully sent reports.
	FailedReports         int // Count of failed report attempts.

	StartTime   time.Time      // Timestamp when the session was started.
	EndTime     time.Time      // Timestamp when the session concluded (completed, aborted, or failed).
	ProxiesUsed map[string]int // TODO: Track proxy usage statistics.

	LogChannel     chan LogUpdate // Channel for sending LogUpdate messages to listeners (e.g., TUI).
	controlChannel chan string    // Internal channel for control commands (pause, resume, abort).

	wg sync.WaitGroup // Used to wait for the main runLoop goroutine to finish.
	mu sync.Mutex     // Protects concurrent access to shared fields (State, counts, etc.).
}

// NewSession creates a new reporting session configured to send `numReportsToSend`
// reports to the specified `targetURL` using the provided `reporter`.
// The session starts in the Idle state.
func NewSession(reporter *report.Reporter, targetURL string, numReportsToSend int) *Session {
	if numReportsToSend <= 0 {
		numReportsToSend = 1 // Ensure at least one report is attempted.
	}
	jobs := make([]*ReportJob, numReportsToSend)
	for i := 0; i < numReportsToSend; i++ {
		jobs[i] = &ReportJob{
			ID:           uuid.NewString(),
			ReportNumber: i + 1, // 1-based for user display
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
		LogChannel:       make(chan LogUpdate, 100), // Buffered channel for TUI updates.
		controlChannel:   make(chan string, 10),     // Buffered for control commands.
	}
}

// sendLog is an internal helper to send a LogUpdate to the LogChannel.
// It includes a timeout to prevent blocking indefinitely if the channel is full.
func (s *Session) sendLog(level string, message string) {
	select {
	case s.LogChannel <- LogUpdate{Level: level, Message: message, Timestamp: time.Now()}:
	case <-time.After(1 * time.Second): // Timeout to prevent deadlock.
		// Fallback if TUI is not consuming logs (e.g., during shutdown or if TUI is frozen).
		fmt.Printf("LogChannel Send Timeout: [%s] %s\n", level, message)
	}
}

// Start initiates the session's reporting process in a new goroutine.
// It returns an error if the session is not in a startable state (Idle, Stopped, Completed, Aborted).
// If restarting a session, its progress counters and job statuses are reset.
func (s *Session) Start() error {
	s.mu.Lock()
	// Allow starting from Idle or any terminal/stopped state (which implies a restart).
	if s.State != Idle && s.State != Stopped && s.State != Completed && s.State != Aborted && s.State != Failed {
		s.mu.Unlock()
		return fmt.Errorf("session cannot be started from its current state: %s", s.State)
	}

	s.State = Running
	s.StartTime = time.Now()
	s.EndTime = time.Time{} // Clear EndTime if this is a restart.

	// Reset counters and job statuses if this is a fresh start or a restart.
	s.ReportsAttemptedCount = 0
	s.SuccessfulReports = 0
	s.FailedReports = 0
	for i := 0; i < s.NumReportsToSend; i++ {
		// Ensure Jobs slice is not nil and element exists (should be guaranteed by NewSession)
		if i < len(s.Jobs) && s.Jobs[i] != nil {
			s.Jobs[i].Status = "pending"
			s.Jobs[i].Error = ""
			s.Jobs[i].LogID = ""
		}
	}
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runLoop()
	s.sendLog(LogLevelUpdateInfo, fmt.Sprintf("Session %s started: %d reports to %s.", s.ID, s.NumReportsToSend, s.TargetURL))
	return nil
}

// runLoop is the core goroutine where reports are sent one by one.
// It handles state changes, control commands, and updates progress.
// This function calls `defer s.wg.Done()` and `defer close(s.LogChannel)`.
func (s *Session) runLoop() {
	defer s.wg.Done() // Signal that this goroutine has finished.
	defer func() {    // This deferred function handles cleanup and final state setting.
		s.mu.Lock()
		if r := recover(); r != nil { // Panic recovery.
			s.sendLog(LogLevelUpdateError, fmt.Sprintf("FATAL: Session runLoop panicked: %v", r))
			s.State = Failed
		}

		// Determine final state if not already Aborted or Failed.
		currentLockedState := s.State
		if currentLockedState != Aborted && currentLockedState != Failed {
			if s.ReportsAttemptedCount >= s.NumReportsToSend {
				s.State = Completed
				s.sendLog(LogLevelUpdateInfo, "Session completed: All reports processed.")
			} else if currentLockedState != Paused { // Not all jobs done, not paused -> implies stopped early.
				s.State = Stopped
				s.sendLog(LogLevelUpdateWarn, "Session stopped before completing all reports.")
			}
			// If it was Paused and the loop exited (e.g., control channel closed externally), it remains Paused.
			// An explicit Abort call would then be needed to terminate it fully.
		} else if currentLockedState == Aborted { // If it was Aborted.
			s.sendLog(LogLevelUpdateWarn, "Session processing was aborted.")
		}

		if s.EndTime.IsZero() {
			s.EndTime = time.Now()
		} // Set end time if not already set (e.g., by Abort).
		s.mu.Unlock()
		close(s.LogChannel) // Signal to listeners that no more logs will come from this session.
	}()

	for { // Loop for each report to be sent.
		s.mu.Lock()
		// Check if all reports have been attempted or if a terminal state was reached.
		if s.ReportsAttemptedCount >= s.NumReportsToSend || (s.State != Running && s.State != Paused) {
			s.mu.Unlock()
			break
		}
		currentState := s.State // Capture current state under lock.
		currentJob := s.Jobs[s.ReportsAttemptedCount]
		s.mu.Unlock()

		// Handle control commands (Pause, Resume, Abort).
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
				// Inner loop to block while paused, waiting for "resume" or "abort".
				for pausedCmd := range s.controlChannel {
					s.mu.Lock()
					if pausedCmd == "resume" {
						if s.State == Paused {
							s.State = Running
							s.sendLog(LogLevelUpdateWarn, "Session resumed.")
						}
						s.mu.Unlock()
						break // Exit pause-wait loop, continue outer report loop.
					} else if pausedCmd == "abort" {
						s.State = Aborted // Set final state.
						s.mu.Unlock()
						return // Exit runLoop entirely.
					}
					s.sendLog(LogLevelUpdateWarn, fmt.Sprintf("Invalid command '%s' while session is paused.", pausedCmd))
					s.mu.Unlock()
				}
				// If controlChannel was closed while paused.
				s.mu.Lock()
				if s.State == Paused { // If still paused (e.g., channel closed externally).
					s.State = Aborted // Treat as an abort.
				}
				s.mu.Unlock()
				if s.GetStateValue() == Aborted {
					return
				} // Exit runLoop if aborted.
				continue // Re-evaluate main loop condition.
			case "abort":
				s.State = Aborted // Set final state.
				s.mu.Unlock()
				return // Exit runLoop entirely.
			default: // Unknown command.
				s.sendLog(LogLevelUpdateWarn, fmt.Sprintf("Unknown control command received: %s", cmd))
				s.mu.Unlock()
			}
			continue // After handling a control command, re-evaluate main loop.
		default:
			// No control message, proceed.
		}

		s.mu.Lock()
		// Re-check state after potential control message processing or if it changed.
		if s.State != Running {
			s.mu.Unlock()
			if currentState == Paused { // If it was paused, sleep briefly before re-checking.
				time.Sleep(100 * time.Millisecond)
			}
			continue // Re-evaluate main loop condition (e.g. might be paused or aborted).
		}
		s.mu.Unlock() // Unlock before blocking on SendReport.

		// Process the current report job.
		currentJob.Status = "processing"
		currentJob.StartTime = time.Now()
		s.sendLog(LogLevelUpdateInfo, fmt.Sprintf("Report %d/%d to %s -> Sending...", currentJob.ReportNumber, s.NumReportsToSend, s.TargetURL))

		// This is a blocking call. Reporter.SendReport handles its own retries.
		reportErr := s.Reporter.SendReport(s.TargetURL, s.ID) // Reason is no longer passed.
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
			// TODO: currentJob.LogID = ... // Reporter.SendReport needs to return this.
		}
		s.ReportsAttemptedCount++
		s.mu.Unlock()
	}
}

// Pause sends a command to the runLoop to pause the session.
// Returns an error if the session is not currently running.
func (s *Session) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != Running {
		return fmt.Errorf("session is not running, cannot pause (current state: %s)", s.State)
	}
	s.controlChannel <- "pause"
	return nil
}

// Resume sends a command to the runLoop to resume a paused session.
// Returns an error if the session is not currently paused.
func (s *Session) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != Paused {
		return fmt.Errorf("session is not paused, cannot resume (current state: %s)", s.State)
	}
	s.controlChannel <- "resume"
	return nil
}

// Abort signals the session to stop processing reports and clean up.
// It waits for the runLoop goroutine to complete, with a timeout.
// Returns an error if the session is in a state that cannot be aborted or if timeout occurs.
func (s *Session) Abort() error {
	s.mu.Lock()
	// Check if session is in a state where abort is meaningful or possible.
	if s.State == Completed || s.State == Aborted || s.State == Failed || s.State == Idle {
		s.mu.Unlock()
		return nil // Nothing to abort or already done.
	}

	isAlreadyStopping := (s.State == Stopping || s.State == Aborted) // Aborted also implies stopping is done.
	s.State = Stopping                                               // Indicate intent to stop. runLoop will set final Aborted state.
	s.mu.Unlock()

	if !isAlreadyStopping {
		s.sendLog(LogLevelUpdateWarn, "Abort signal sent to session.")
		s.controlChannel <- "abort"
	}

	// Wait for runLoop goroutine to finish, with a timeout.
	waitTimeout := time.NewTimer(10 * time.Second)
	defer waitTimeout.Stop()
	done := make(chan struct{})
	go func() {
		s.wg.Wait() // Wait for s.wg.Done() in runLoop's defer.
		close(done)
	}()

	select {
	case <-done: // runLoop completed.
		s.mu.Lock()
		s.State = Aborted // Ensure final state is Aborted.
		if s.EndTime.IsZero() {
			s.EndTime = time.Now()
		}
		s.mu.Unlock()
	case <-waitTimeout.C: // Timeout waiting for runLoop.
		s.sendLog(LogLevelUpdateError, "Timeout waiting for session to abort; runLoop may be stuck.")
		// State remains Stopping if runLoop is stuck.
		return fmt.Errorf("timeout waiting for session to abort")
	}
	return nil
}

// GetStateValue returns the current operational state of the session (thread-safe).
func (s *Session) GetStateValue() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State
}

// GetSummary provides a human-readable string summary of the session's status and progress.
// Suitable for display in logs or TUI status lines.
func (s *Session) GetSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var duration time.Duration
	if !s.StartTime.IsZero() {
		if s.EndTime.IsZero() || s.State == Running || s.State == Paused { // Ongoing session.
			duration = time.Since(s.StartTime)
		} else { // Session has a definitive end time.
			duration = s.EndTime.Sub(s.StartTime)
		}
	}
	return fmt.Sprintf("ID: %s | State: %s | Target: %s | Reports: %d/%d | Success: %d | Fail: %d | Duration: %s",
		s.ID, s.State.String(), s.TargetURL, s.ReportsAttemptedCount, s.NumReportsToSend, s.SuccessfulReports, s.FailedReports, duration.Round(time.Second).String())
}

// GetStats returns key statistics about the session in a thread-safe manner.
// This includes its current state, target URL, counts for total reports to send,
// reports attempted, successful reports, and failed reports.
func (s *Session) GetStats() (currentState SessionState, targetURL string, numToSend int, attempted int, successful int, failed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State, s.TargetURL, s.NumReportsToSend, s.ReportsAttemptedCount, s.SuccessfulReports, s.FailedReports
}

// EnsureLogChannelClosed can be called by the TUI if it needs to signal it's done with this session object
// particularly if the session was never started. The primary mechanism for channel closure is the
// defer function in runLoop.
func (s *Session) EnsureLogChannelClosed() {
	// The runLoop's defer close(s.LogChannel) is the primary mechanism.
	// This function is more of a conceptual placeholder. In a robust system with multiple
	// consumers or more complex lifecycle, channel closure needs careful design, often
	// involving context cancellation or sync.Once for the close operation.
	// For this application, TUI should gracefully handle reading from a closed channel
	// when listenForSessionLogsCmd's <-s.LogChannel returns !ok.
}
