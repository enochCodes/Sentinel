package session

import (
	"fmt"
	"sync"
	"time"
	"github.com/google/uuid" // For session ID

	"sentinelgo/report"
	// "sentinelgo/utils" // Not importing utils here to avoid circular deps if utils imports session
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
	Level   string    // e.g., "INFO", "ERROR", "WARN"
	Message string
	Timestamp time.Time // Added timestamp for TUI display
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

// ReportJob defines a single reporting task.
type ReportJob struct {
	ID          string
	TargetURL   string
	Reason      string
	Status      string
	LogID       string
	Error       string
	StartTime   time.Time
	EndTime     time.Time
	Attempts    int
}

// Session manages a collection of report jobs and their execution.
type Session struct {
	ID                string
	State             SessionState
	Reporter          *report.Reporter
	Jobs              []*ReportJob
	CurrentJobIndex   int
	ProcessedJobs     int
	SuccessfulReports int
	FailedReports     int
	StartTime         time.Time
	EndTime           time.Time
	ProxiesUsed       map[string]int

	LogChannel        chan LogUpdate
	controlChannel    chan string

	wg                sync.WaitGroup
	mu                sync.Mutex
}

// NewSession creates a new reporting session.
func NewSession(reporter *report.Reporter, initialJobsData []struct{URL string; Reason string}) *Session {
	jobs := make([]*ReportJob, len(initialJobsData))
	for i, ij := range initialJobsData {
		jobs[i] = &ReportJob{
			ID: uuid.NewString(),
			TargetURL: ij.URL,
			Reason: ij.Reason,
			Status: "pending",
		}
	}

	return &Session{
		ID:             uuid.NewString(),
		State:          Idle,
		Reporter:       reporter,
		Jobs:           jobs,
		ProxiesUsed:    make(map[string]int),
		LogChannel:     make(chan LogUpdate, 100),
		controlChannel: make(chan string, 10),
	}
}

func (s *Session) sendLog(level string, message string) {
	// Helper to ensure sending to LogChannel doesn't block indefinitely if it's full,
	// though a buffered channel makes this less likely for typical TUI updates.
	// A select with a default case could make it non-blocking, but might drop messages.
	// For now, assume TUI consumes fast enough or buffer is sufficient.
	s.LogChannel <- LogUpdate{Level: level, Message: message, Timestamp: time.Now()}
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
	s.EndTime = time.Time{} // Clear end time if restarting
	if s.CurrentJobIndex >= len(s.Jobs) || s.State == Completed || s.State == Aborted {
		s.CurrentJobIndex = 0
		s.ProcessedJobs = 0
		s.SuccessfulReports = 0
		s.FailedReports = 0
		for _, job := range s.Jobs { // Reset job statuses
			job.Status = "pending"
			job.Error = ""
			job.LogID = ""
			job.Attempts = 0
		}
	}
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runLoop()
	s.sendLog(LogLevelUpdateInfo, fmt.Sprintf("Session %s started.", s.ID))
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

		finalState := s.State // Current state before this defer
		if finalState != Aborted && finalState != Failed {
			if s.CurrentJobIndex >= len(s.Jobs) {
				s.State = Completed
				s.sendLog(LogLevelUpdateInfo, "Session completed: All jobs processed.")
			} else if finalState != Paused { // Not all jobs done, not paused -> stopped
				s.State = Stopped
				s.sendLog(LogLevelUpdateWarn, "Session stopped before completion.")
			}
		} else if finalState == Aborted {
			s.sendLog(LogLevelUpdateWarn, "Session processing aborted by user.")
		}

		s.EndTime = time.Now()
		s.mu.Unlock()
		// Close LogChannel signals TUI that no more logs for this session.
		// Ensure this is the only place it's closed for a given session instance.
		close(s.LogChannel)
	}()

	for {
		s.mu.Lock()
		if s.CurrentJobIndex >= len(s.Jobs) {
			s.mu.Unlock()
			break
		}
		currentState := s.State
		currentJob := s.Jobs[s.CurrentJobIndex]
		s.mu.Unlock()

		select {
		case cmd := <-s.controlChannel:
			s.mu.Lock()
			switch cmd {
			case "pause":
				if s.State == Running { // Only pause if running
					s.State = Paused
					s.sendLog(LogLevelUpdateWarn, "Session paused.")
				}
				s.mu.Unlock()
				// Block for resume or abort
				for pausedCmd := range s.controlChannel {
					s.mu.Lock()
					if pausedCmd == "resume" {
						if s.State == Paused { // Only resume if actually paused
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
				// If controlChannel closed while paused
				s.mu.Lock()
				if s.State == Paused { // If still paused (e.g. chan closed)
					s.State = Aborted
				}
				s.mu.Unlock()
				if s.GetState() == Aborted { return }
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
			if currentState == Paused {
				 time.Sleep(100 * time.Millisecond)
			}
			continue
		}
		s.mu.Unlock()

		currentJob.Status = "processing"
		currentJob.StartTime = time.Now()
		currentJob.Attempts++
		s.sendLog(LogLevelUpdateInfo, fmt.Sprintf("Job %d/%d: %s -> Sending...", s.CurrentJobIndex+1, len(s.Jobs), currentJob.TargetURL))

		reportErr := s.Reporter.SendReport(currentJob.TargetURL, currentJob.Reason, s.ID)
		currentJob.EndTime = time.Now()

		s.mu.Lock()
		if reportErr != nil {
			currentJob.Status = "failed"
			currentJob.Error = reportErr.Error()
			s.FailedReports++
			s.sendLog(LogLevelUpdateError, fmt.Sprintf("Job %d/%d: %s -> Failed: %s", s.CurrentJobIndex+1, len(s.Jobs), currentJob.TargetURL, reportErr.Error()))
		} else {
			currentJob.Status = "success"
			s.SuccessfulReports++
			s.sendLog(LogLevelUpdateInfo, fmt.Sprintf("Job %d/%d: %s -> Success.", s.CurrentJobIndex+1, len(s.Jobs), currentJob.TargetURL))
		}
		s.ProcessedJobs++
		s.CurrentJobIndex++
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
		// If Idle, runLoop hasn't started. LogChannel might be open but no sender.
		// TUI should handle this. If Abort is called multiple times, this prevents error/panic.
		if s.State == Idle && s.LogChannel != nil {
			// It's a bit aggressive to close it here as TUI might be setting up listener.
			// Consider a more graceful shutdown signal if needed for Idle state.
		}
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
		return fmt.Errorf("timeout waiting for session to abort")
	}
	return nil
}

func (s *Session) GetSummary() string {
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
	return fmt.Sprintf("ID: %s | State: %s | Jobs: %d/%d | Success: %d | Failed: %d | Duration: %s",
		s.ID, s.State.String(), s.ProcessedJobs, len(s.Jobs), s.SuccessfulReports, s.FailedReports, duration.Round(time.Second).String())
}

func (s *Session) GetState() (SessionState, int, int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State, len(s.Jobs), s.ProcessedJobs, s.SuccessfulReports, s.FailedReports
}

// CloseLogChannel is intended to be called by the owner of the session (e.g. TUI)
// when it's completely done with the session and its logs, especially if the session
// was never started or aborted early. The runLoop's defer is the primary closer when active.
func (s *Session) EnsureLogChannelClosed() {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.LogChannel != nil {
        // How to safely close a channel that might be written to or closed by another goroutine?
        // 1. Use a sync.Once for the actual close operation.
        // 2. Send a special "please close" message to runLoop (but runLoop might be done).
        // 3. Have a dedicated channel for closing signals.
        // For now, the runLoop's defer is the main closer. This method is more a hint
        // that TUI is done, primarily for Idle sessions where runLoop didn't start.
        if s.State == Idle || s.State == Completed || s.State == Aborted || s.State == Failed {
            // Attempt to close only if we are sure no one else will write to it.
            // This is still risky. A select with a default case on send is safer.
            // The most robust is that the TUI just stops reading when the session is terminal.
            // And runLoop's defer always closes it.
            // If runLoop didn't start (e.g. session created then immediately discarded),
            // the channel would be GC'd. If TUI listened, it would block.
            // To prevent TUI block on Idle sessions, TUI should timeout its read or
            // NewSession should not create LogChannel until Start().
            // For now, let's assume TUI handles closed channel reads.
        }
    }
}
