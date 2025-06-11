package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid" // For session ID

	"sentinelgo/sentinelgo/report"
	// "sentinelgo/utils" // utils.LogEntry might be too verbose for LogChannel, using string for now
)

// SessionState defines the possible states of a reporting session.
type SessionState int

const (
	Idle SessionState = iota
	Running
	Paused
	Stopping  // Actively trying to stop workers
	Stopped   // Workers have confirmed stop. Can be resumed.
	Completed // All jobs processed.
	Aborted   // User initiated stop, not all jobs processed.
	Failed    // Session encountered a critical internal error (e.g., bad config)
)

// String returns the string representation of SessionState.
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
		return "Stopped (Can Resume)"
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

// ReportJob defines a single reporting task.
type ReportJob struct {
	ID        string // Unique ID for this job
	TargetURL string
	Reason    string
	Status    string // e.g., "pending", "processing", "success", "failed"
	LogID     string // From report response, if any
	Error     string // Error string from the reporting attempt
	StartTime time.Time
	EndTime   time.Time
	Attempts  int
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

	LogChannel     chan string
	controlChannel chan string

	wg sync.WaitGroup
	mu sync.Mutex
}

// NewSession creates a new reporting session.
func NewSession(reporter *report.Reporter, initialJobsData []struct {
	URL    string
	Reason string
}) *Session {
	jobs := make([]*ReportJob, len(initialJobsData))
	for i, ij := range initialJobsData {
		jobs[i] = &ReportJob{
			ID:        uuid.NewString(),
			TargetURL: ij.URL,
			Reason:    ij.Reason,
			Status:    "pending",
		}
	}

	return &Session{
		ID:             uuid.NewString(),
		State:          Idle,
		Reporter:       reporter,
		Jobs:           jobs,
		ProxiesUsed:    make(map[string]int),
		LogChannel:     make(chan string, 100),
		controlChannel: make(chan string, 10),
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
	if s.CurrentJobIndex >= len(s.Jobs) { // If restarting a completed/aborted session from beginning
		s.CurrentJobIndex = 0
		s.ProcessedJobs = 0
		s.SuccessfulReports = 0
		s.FailedReports = 0
		// Reset job statuses if needed
		for _, job := range s.Jobs {
			job.Status = "pending"
			job.Error = ""
			job.LogID = ""
		}
	}
	s.mu.Unlock() // Unlock before starting goroutine to avoid holding lock during runLoop init

	s.wg.Add(1)
	go s.runLoop()
	s.LogChannel <- fmt.Sprintf("Session %s started.", s.ID)
	return nil
}

// runLoop is the main goroutine for processing report jobs.
func (s *Session) runLoop() {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		if r := recover(); r != nil {
			s.LogChannel <- fmt.Sprintf("FATAL: Session runLoop panicked: %v", r)
			s.State = Failed // Mark session as failed due to panic
		}
		// If loop finishes naturally and not aborted, mark as completed or stopped.
		if s.State != Aborted && s.State != Failed {
			if s.CurrentJobIndex >= len(s.Jobs) {
				s.State = Completed
				s.LogChannel <- "Session completed: All jobs processed."
			} else {
				// This case implies it was paused and then an abort signal was sent,
				// or some other logic error. If paused, it should remain Paused/Stopped.
				// If it was Running and exited loop without finishing, that's unexpected.
				// For now, if not completed, and not aborted, assume it was externally stopped/paused.
				if s.State != Paused { // If it wasn't a clean pause, mark as stopped.
					s.State = Stopped
				}
			}
		}
		s.EndTime = time.Now()
		// Consider closing LogChannel here, but only if no other goroutine (like Abort) will use it.
		// For safety, let TUI handle channel closure detection or use a separate signal.
		// close(s.LogChannel) // Be careful with closing if TUI is still listening.
	}()

	for {
		s.mu.Lock()
		if s.CurrentJobIndex >= len(s.Jobs) {
			s.mu.Unlock()
			s.LogChannel <- "All jobs processed."
			break // All jobs processed
		}

		currentState := s.State
		currentJob := s.Jobs[s.CurrentJobIndex]
		s.mu.Unlock() // Unlock while processing a single job or checking control channel

		// Non-blocking check for control messages
		select {
		case cmd := <-s.controlChannel:
			s.mu.Lock()
			switch cmd {
			case "pause":
				s.State = Paused
				s.LogChannel <- "Session paused."
				s.mu.Unlock() // Unlock before blocking for resume/abort
				// Block until resume or abort
				for pausedCmd := range s.controlChannel {
					if pausedCmd == "resume" {
						s.mu.Lock()
						s.State = Running
						s.LogChannel <- "Session resumed."
						s.mu.Unlock()
						break // Exit pause loop, continue outer runLoop
					} else if pausedCmd == "abort" {
						s.State = Aborted
						s.LogChannel <- "Session aborting (from paused state)..."
						s.mu.Unlock()
						return // Exit runLoop
					}
					// else, invalid command while paused, ignore or log
				}
				// If controlChannel closed while paused, treat as abort.
				if s.State == Paused { // Check if still paused (channel closed)
					s.mu.Lock()
					s.State = Aborted
					s.LogChannel <- "Session aborting (control channel closed while paused)..."
					s.mu.Unlock()
					return
				}
				continue // Continue to next iteration of runLoop (re-check conditions)
			case "abort":
				s.State = Aborted
				s.LogChannel <- "Session aborting..."
				s.mu.Unlock()
				return // Exit runLoop
			default:
				// unknown command, log and ignore
				s.LogChannel <- fmt.Sprintf("Unknown control command: %s", cmd)
				s.mu.Unlock() // Must unlock if default case taken
			}
			// if a command was processed, re-evaluate loop conditions from start
			// This continue might not be strictly necessary if state changes are handled well inside cases.
			// continue
		default:
			// No control message, proceed with job if running
		}

		s.mu.Lock() // Re-lock for state check before processing job
		if s.State != Running {
			s.mu.Unlock() // Not running, re-iterate to check control messages or exit conditions
			// Add a small sleep to prevent busy-looping when paused and no control messages
			if currentState == Paused { // Use currentState captured at start of loop iteration
				time.Sleep(100 * time.Millisecond)
			}
			continue
		}
		s.mu.Unlock() // Unlock for the actual SendReport call

		currentJob.Status = "processing"
		currentJob.StartTime = time.Now()
		currentJob.Attempts++
		s.LogChannel <- fmt.Sprintf("Processing job %d/%d: %s", s.CurrentJobIndex+1, len(s.Jobs), currentJob.TargetURL)

		// Call Reporter.SendReport. This is a blocking call.
		// The Reporter's logger will handle detailed structured logging.
		// Session's LogChannel is for TUI status updates.
		reportErr := s.Reporter.SendReport(currentJob.TargetURL, currentJob.Reason, s.ID)

		currentJob.EndTime = time.Now()

		s.mu.Lock()
		if reportErr != nil {
			currentJob.Status = "failed"
			currentJob.Error = reportErr.Error()
			s.FailedReports++
			s.LogChannel <- fmt.Sprintf("Job %s failed: %s", currentJob.TargetURL, reportErr.Error())
		} else {
			currentJob.Status = "success"
			// currentJob.LogID = ... // SendReport needs to return this, or it's in the detailed log
			s.SuccessfulReports++
			s.LogChannel <- fmt.Sprintf("Job %s success.", currentJob.TargetURL)
		}
		s.ProcessedJobs++
		s.CurrentJobIndex++
		s.mu.Unlock()
	}
}

// Pause sends a command to pause the session.
func (s *Session) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != Running {
		return fmt.Errorf("session is not running, cannot pause (state: %s)", s.State)
	}
	s.controlChannel <- "pause"
	return nil
}

// Resume sends a command to resume a paused session.
func (s *Session) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != Paused {
		return fmt.Errorf("session is not paused, cannot resume (state: %s)", s.State)
	}
	s.controlChannel <- "resume"
	return nil
}

// Abort sends a command to stop the session permanently and waits for completion.
func (s *Session) Abort() error {
	s.mu.Lock()
	// Allow abort from Running or Paused states primarily
	if s.State != Running && s.State != Paused && s.State != Stopping {
		// If Idle, Stopped, Completed, Failed, Aborted, nothing to do or already done.
		if s.State == Idle || s.State == Stopped || s.State == Completed || s.State == Failed || s.State == Aborted {
			s.mu.Unlock()
			// Ensure LogChannel is closed if it wasn't by runLoop
			// This is tricky, as runLoop might not have started.
			// Consider a dedicated close mechanism or rely on GC for unstarted sessions.
			// if s.State == Idle { close(s.LogChannel) }
			return nil
		}
		s.mu.Unlock()
		return fmt.Errorf("session cannot be aborted from state: %s", s.State)
	}

	if s.State != Stopping { // Avoid sending multiple "abort" commands if already stopping
		s.State = Stopping // Tentatively set to Stopping, runLoop will confirm with Aborted
		s.LogChannel <- "Abort signal sent to session."
		s.controlChannel <- "abort"
	}
	s.mu.Unlock()

	// Wait for runLoop to finish.
	// Add a timeout to prevent indefinite blocking if runLoop is stuck.
	waitTimeout := time.NewTimer(10 * time.Second) // Adjust timeout as needed
	defer waitTimeout.Stop()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Successfully waited for runLoop to finish
		s.mu.Lock()
		s.State = Aborted // Confirm final state
		s.EndTime = time.Now()
		s.mu.Unlock()
		s.LogChannel <- "Session aborted successfully."
	case <-waitTimeout.C:
		// Timeout waiting for runLoop
		s.LogChannel <- "Timeout waiting for session to abort. Goroutine might be stuck."
		return fmt.Errorf("timeout waiting for session to abort")
	}

	// It's generally safer for the producer (runLoop) to close the LogChannel.
	// However, if Abort can happen before runLoop starts or after it finishes,
	// closing here needs care. For now, let runLoop handle its closure or TUI handle reading from closed chan.
	// close(s.LogChannel) // This could panic if runLoop tries to send after this.
	return nil
}

// GetSummary provides a string summary of the session.
func (s *Session) GetSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var duration time.Duration
	if !s.StartTime.IsZero() {
		if s.EndTime.IsZero() {
			duration = time.Since(s.StartTime)
		} else {
			duration = s.EndTime.Sub(s.StartTime)
		}
	}

	summary := fmt.Sprintf("Session ID: %s\nState: %s\n", s.ID, s.State.String())
	summary += fmt.Sprintf("Total Jobs: %d\n", len(s.Jobs))
	summary += fmt.Sprintf("Processed Jobs: %d\n", s.ProcessedJobs)
	summary += fmt.Sprintf("Successful Reports: %d\n", s.SuccessfulReports)
	summary += fmt.Sprintf("Failed Reports: %d\n", s.FailedReports)
	summary += fmt.Sprintf("Duration: %s\n", duration.Round(time.Second).String())
	// TODO: Add proxy usage summary if s.ProxiesUsed gets populated
	return summary
}

// GetState returns the current state of the session in a thread-safe manner.
func (s *Session) GetState() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State
}
