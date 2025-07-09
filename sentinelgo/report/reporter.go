package report

import (
	// "bytes" // No longer needed if not constructing specific body like form data
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url" // Required for url.Error
	"strings"
	"time"

	"sentinelgo/sentinelgo/ai"
	"sentinelgo/sentinelgo/config"
	"sentinelgo/sentinelgo/proxy"
	"sentinelgo/sentinelgo/utils"
)

// defaultUserAgents provides a fallback list of User-Agent strings if not specified in AppConfig.
// This helps in mimicking various legitimate browsers or devices.
var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/99.0.4844.51 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/99.0.4844.51 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:98.0) Gecko/20100101 Firefox/98.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
}

// Reporter encapsulates the logic for sending a single report, including handling proxies,
// retries, configuration, logging, and optional AI analysis.
type Reporter struct {
	Config     *config.AppConfig   // Application configuration, passed as a pointer.
	ProxyMgr   *proxy.ProxyManager // Manages proxy selection and status.
	Logger     *utils.Logger       // Structured logger for recording events.
	AIAnalyzer ai.ContentAnalyzer  // Optional content analyzer.
	HTTPClient *http.Client        // HTTP client used for sending requests.
}

// NewReporter creates and returns a new Reporter instance.
//
// Parameters:
//   - cfg: A pointer to the application's AppConfig.
//   - pm: A pointer to the ProxyManager for proxy handling.
//   - logger: A pointer to the Logger for structured logging.
//   - analyzer: An implementation of the ai.ContentAnalyzer interface for content analysis (can be nil).
//
// The HTTPClient is initialized here but its transport (including proxy) is configured per request attempt.
func NewReporter(cfg *config.AppConfig, pm *proxy.ProxyManager, logger *utils.Logger, analyzer ai.ContentAnalyzer) *Reporter {
	return &Reporter{
		Config:     cfg,
		ProxyMgr:   pm,
		Logger:     logger,
		AIAnalyzer: analyzer,
		HTTPClient: &http.Client{
			// Global timeout for the client can be set here, but more granular timeouts
			// are often applied per request using context or transport settings.
		},
	}
}

// SendReport attempts to send a single "report" to the specified targetURL.
// This method manages the entire lifecycle of a single report transmission, including:
//   - Selecting a proxy via the ProxyManager.
//   - Constructing and sending an HTTP POST request (currently with a nil body).
//   - Applying headers and cookies from AppConfig.
//   - Retrying the request up to Config.MaxRetries times on failure.
//   - Performing AI content analysis on the response if an AIAnalyzer is configured and the request is successful.
//   - Logging all significant events (attempts, successes, failures, AI results) using the structured logger.
//
// Parameters:
//   - targetURL: The URL to which the report request will be sent.
//   - sessionID: A unique identifier for the current reporting session, used for logging context.
//
// Returns:
//   - `nil` if the report is considered successfully sent (e.g., HTTP 2xx response) after any retries.
//   - An error if the report fails after all retry attempts, or if a non-retryable error occurs
//     (e.g., failure to get a proxy, request creation failure).
//
// Note: The "reportReason" parameter was removed as the request body is currently nil.
// The actual nature of the "report" is implicit in the targetURL and the POST request method.
func (r *Reporter) SendReport(targetURL string, sessionID string) error {
	var lastErr error // Stores the error from the last attempt.

	// Retry loop based on MaxRetries from configuration.
	for attempt := 0; attempt < r.Config.MaxRetries; attempt++ {
		// Context for per-attempt timeout and potential cancellation.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30) // Overall timeout for one attempt.
		defer cancel()                                                           // Ensure cancel is called to free resources.

		// Select a proxy for this attempt.
		selectedProxy, err := r.ProxyMgr.GetProxy() // TODO: Future: pass targetRegion if strategy needs it.
		if err != nil {
			// Log and return if no proxy is available, as this is a prerequisite.
			r.Logger.Error(utils.LogEntry{
				SessionID: sessionID, Message: "Failed to get proxy for report attempt", ReportURL: targetURL,
				Error: err.Error(), Outcome: "failed_prereq",
			})
			return fmt.Errorf("failed to get proxy: %w", err)
		}

		// Configure HTTP client transport for this attempt with the selected proxy.
		transport := &http.Transport{
			Proxy:                 http.ProxyURL(selectedProxy.URL),
			ResponseHeaderTimeout: 20 * time.Second, // Specific timeout for receiving headers.
			ExpectContinueTimeout: 5 * time.Second,  // Timeout for 100-continue responses.
		}
		r.HTTPClient.Transport = transport
		// r.HTTPClient.Timeout can be set here too for the entire Do call, if preferred over context.

		// Create the HTTP request. Currently, it's a POST with a nil body.
		// If a request body is needed in the future, it would be constructed here.
		var reqBody io.Reader = nil // Explicitly nil for POST with no body.
		reqBodyStr := ""            // For logging; empty as there's no body.

		req, err := http.NewRequestWithContext(ctx, "POST", targetURL, reqBody)
		if err != nil {
			// Log and return if request creation fails (should not be retried).
			r.Logger.Error(utils.LogEntry{SessionID: sessionID, Message: "Failed to create request", ReportURL: targetURL, Error: err.Error()})
			return fmt.Errorf("failed to create request: %w", err) // Critical failure for this attempt.
		}

		// Set headers from AppConfig and a fallback default user agent list.
		userAgent := r.Config.DefaultHeaders["User-Agent"]
		if userAgent == "" && len(defaultUserAgents) > 0 {
			userAgent = defaultUserAgents[rand.Intn(len(defaultUserAgents))]
		}
		req.Header.Set("User-Agent", userAgent)
		for key, value := range r.Config.DefaultHeaders {
			if key != "User-Agent" { // Avoid setting User-Agent twice.
				req.Header.Set(key, value)
			}
		}

		// Add custom cookies from AppConfig.
		for _, cookie := range r.Config.CustomCookies {
			req.AddCookie(&cookie)
		}

		// Log before sending the request.
		preReqLogEntry := utils.LogEntry{
			SessionID:      sessionID,
			Message:        fmt.Sprintf("Attempting report (attempt %d/%d)", attempt+1, r.Config.MaxRetries),
			ReportURL:      targetURL,
			Proxy:          selectedProxy.URL.String(),
			UserAgent:      req.Header.Get("User-Agent"),
			RequestMethod:  req.Method,
			RequestHeaders: req.Header.Clone(), // Clone to log headers as prepared.
			RequestBody:    reqBodyStr,         // Empty as reqBody is nil.
		}
		r.Logger.Info(preReqLogEntry)

		// Execute the request.
		startTime := time.Now()
		resp, err := r.HTTPClient.Do(req)
		latency := time.Since(startTime)

		// Prepare a log entry for the outcome, to be filled as details emerge.
		logEntry := utils.LogEntry{
			SessionID: sessionID, Message: "Report attempt completed", ReportURL: targetURL,
			Proxy: selectedProxy.URL.String(), UserAgent: req.Header.Get("User-Agent"),
			RequestMethod: req.Method, RequestHeaders: preReqLogEntry.RequestHeaders, RequestBody: reqBodyStr,
		}

		if err != nil { // Network error or client-side error (e.g., timeout).
			lastErr = fmt.Errorf("attempt %d/%d to %s via %s failed: %w", attempt+1, r.Config.MaxRetries, targetURL, selectedProxy.URL.String(), err)
			logEntry.Error = err.Error()
			logEntry.Outcome = "failed_request_error"
			r.Logger.Error(logEntry)

			// Heuristically update proxy status if the error seems proxy-related.
			if urlErr, ok := err.(*url.Error); ok && (urlErr.Timeout() || urlErr.Temporary()) {
				r.ProxyMgr.UpdateProxyStatus(selectedProxy.URL.String(), "unhealthy", latency)
			} else if strings.Contains(err.Error(), "connect: connection refused") || strings.Contains(err.Error(), "proxyconnect") {
				r.ProxyMgr.UpdateProxyStatus(selectedProxy.URL.String(), "unhealthy", latency)
			}

			// If not the last attempt, sleep and continue to the next retry.
			if attempt < r.Config.MaxRetries-1 {
				time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second) // Simple random backoff.
				continue
			}
			return lastErr // All retries exhausted for this specific error type.
		}
		defer resp.Body.Close() // Ensure response body is closed for this successful attempt.

		// Read response body.
		bodyBytes, readErr := io.ReadAll(resp.Body)
		responseBodyStr := string(bodyBytes)

		if readErr != nil { // Error reading response body.
			lastErr = fmt.Errorf("attempt %d/%d to %s: failed to read response body: %w", attempt+1, r.Config.MaxRetries, targetURL, readErr)
			logEntry.Error = readErr.Error()
			logEntry.Outcome = "failed_read_body"
			logEntry.ResponseStatus = resp.StatusCode // Log status code even if body read fails.
			logEntry.ResponseHeaders = resp.Header.Clone()
			r.Logger.Error(logEntry)

			if attempt < r.Config.MaxRetries-1 {
				continue
			}
			return lastErr
		}

		// Populate remaining fields in the log entry.
		logEntry.ResponseStatus = resp.StatusCode
		logEntry.ResponseHeaders = resp.Header.Clone()
		logEntry.ResponseBody = responseBodyStr        // Caution: can be large.
		logEntry.LogID = resp.Header.Get("X-Tt-Logid") // Example TikTok log ID header.

		// AI Analysis Hook (if analyzer is configured and request was successful so far).
		if r.AIAnalyzer != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			simulatedPostID := "post123_" + targetURL // Simplified post ID.
			analysisText := responseBodyStr
			if len(analysisText) > 500 {
				analysisText = analysisText[:500]
			} // Truncate for analysis.
			if analysisText == "" {
				analysisText = "No textual content found in response to analyze."
			}

			aiResult, aiErr := r.AIAnalyzer.Analyze(sessionID, simulatedPostID, analysisText)
			if aiErr != nil {
				r.Logger.Error(utils.LogEntry{SessionID: sessionID, Message: "AI analysis failed", ReportURL: targetURL, Error: aiErr.Error(), AdditionalData: map[string]interface{}{"post_id": simulatedPostID}})
			} else if aiResult != nil {
				if logEntry.AdditionalData == nil {
					logEntry.AdditionalData = make(map[string]interface{})
				}
				logEntry.AdditionalData["AIThreatScore"] = aiResult.ThreatScore
				logEntry.AdditionalData["AICategory"] = aiResult.Category
				if len(aiResult.Details) > 0 {
					logEntry.AdditionalData["AIDetails"] = aiResult.Details
				}

				r.Logger.Info(utils.LogEntry{SessionID: sessionID, Message: "AI Analysis Result", ReportURL: targetURL, AdditionalData: map[string]interface{}{"post_id": simulatedPostID, "threat_score": aiResult.ThreatScore, "category": aiResult.Category}})
				if aiResult.ThreatScore > r.Config.RiskThreshold {
					r.Logger.Warn(utils.LogEntry{SessionID: sessionID, Message: "AI detected high risk content!", ReportURL: targetURL, AdditionalData: map[string]interface{}{"post_id": simulatedPostID, "threat_score": aiResult.ThreatScore, "category": aiResult.Category, "threshold": r.Config.RiskThreshold}, Outcome: "high_risk_detected"})
				}
			}
		}

		// Final outcome based on status code.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 { // Successful response.
			logEntry.Outcome = "accepted"
			r.Logger.Info(logEntry)
			return nil // Report successful, exit retry loop.
		}

		// Non-2xx status code is considered a failure for this attempt.
		lastErr = fmt.Errorf("attempt %d/%d to %s: report failed with status %d", attempt+1, r.Config.MaxRetries, targetURL, resp.StatusCode)
		if logEntry.Error == "" {
			logEntry.Error = fmt.Sprintf("status code %d", resp.StatusCode)
		}
		logEntry.Outcome = "failed_status_code"
		r.Logger.Error(logEntry)

		if resp.StatusCode == http.StatusProxyAuthRequired || resp.StatusCode == http.StatusForbidden {
			r.ProxyMgr.UpdateProxyStatus(selectedProxy.URL.String(), "unhealthy", latency)
		}

		if attempt < r.Config.MaxRetries-1 {
			continue
		} // Go to next retry if not last attempt.
		return lastErr // All retries failed for non-2xx status.
	}
	return lastErr // Should only be reached if MaxRetries is 0 or less (loop doesn't run).
}
