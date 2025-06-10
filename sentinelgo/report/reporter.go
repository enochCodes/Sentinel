package report

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"strings"
	"time"

	"sentinelgo/ai" // Import AI package
	"sentinelgo/config"
	"sentinelgo/proxy"
	"sentinelgo/utils"
)

// defaultUserAgents is a fallback list if not provided in config.
var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/99.0.4844.51 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/99.0.4844.51 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:98.0) Gecko/20100101 Firefox/98.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
}

// Reporter handles the process of sending a report.
type Reporter struct {
	Config     *config.AppConfig
	ProxyMgr   *proxy.ProxyManager
	Logger     *utils.Logger
	AIAnalyzer ai.ContentAnalyzer // Added AI Analyzer
	HTTPClient *http.Client
}

// NewReporter creates a new Reporter instance.
func NewReporter(cfg *config.AppConfig, pm *proxy.ProxyManager, logger *utils.Logger, analyzer ai.ContentAnalyzer) *Reporter {
	return &Reporter{
		Config:     cfg,
		ProxyMgr:   pm,
		Logger:     logger,
		AIAnalyzer: analyzer, // Store the analyzer
		HTTPClient: &http.Client{
			// Timeout will be set per request using context or transport for more fine-grained control
		},
	}
}

// SendReport attempts to send a single report to the targetURL.
// It handles proxy selection, request configuration, retries, and logging.
func (r *Reporter) SendReport(targetURL string, reportReason string, sessionID string) error {
	var lastErr error

	for attempt := 0; attempt < r.Config.MaxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30) // Overall timeout for one attempt including connection
		defer cancel()

		selectedProxy, err := r.ProxyMgr.GetProxy() // TODO: Add targetRegion if strategy is region-prioritized
		if err != nil {
			r.Logger.Error(utils.LogEntry{
				SessionID: sessionID,
				Message:   "Failed to get proxy for report attempt",
				ReportURL: targetURL,
				Error:     err.Error(),
				Outcome:   "failed_prereq",
			})
			return fmt.Errorf("failed to get proxy: %w", err)
		}

		// Configure HTTP client with the selected proxy for this attempt
		transport := &http.Transport{
			Proxy: http.ProxyURL(selectedProxy.URL),
			// Add other transport settings like TLSHandshakeTimeout, IdleConnTimeout etc. if needed
			ResponseHeaderTimeout: 20 * time.Second, // Timeout for receiving headers
			ExpectContinueTimeout: 5 * time.Second,
		}
		r.HTTPClient.Transport = transport
        // The client's Timeout field can also be set here if preferred over context for the entire Do call
        // r.HTTPClient.Timeout = 25 * time.Second


		// Request Body (Placeholder - actual form data would be constructed here)
		// For now, using a simple string body. In a real scenario, this would be form data.
		// e.g. form := url.Values{}; form.Add("reason", reportReason); reqBody := strings.NewReader(form.Encode())
		var reqBody io.Reader
		var reqBodyStr string // For logging
		if reportReason != "" { // Assuming reportReason is part of the body
			// This is a placeholder. Actual body structure depends on TikTok API.
			// It could be JSON: `{"report_reason": reportReason, "object_id": ...}`
			// Or form-urlencoded.
			formData := url.Values{}
			formData.Set("reason", reportReason) // Example field
			formData.Set("session_id", sessionID) // Example field
			reqBodyStr = formData.Encode()
			reqBody = strings.NewReader(reqBodyStr)
		}


		req, err := http.NewRequestWithContext(ctx, "POST", targetURL, reqBody)
		if err != nil {
			r.Logger.Error(utils.LogEntry{SessionID: sessionID, Message: "Failed to create request", ReportURL: targetURL, Error: err.Error()})
			// Non-retryable if request creation fails
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set Headers
		userAgent := r.Config.DefaultHeaders["User-Agent"]
		if userAgent == "" && len(defaultUserAgents) > 0 {
			userAgent = defaultUserAgents[rand.Intn(len(defaultUserAgents))]
		}
		req.Header.Set("User-Agent", userAgent)

		for key, value := range r.Config.DefaultHeaders {
			if key != "User-Agent" { // Already set
				req.Header.Set(key, value)
			}
		}
        if reqBody != nil && req.Header.Get("Content-Type") == "" {
             req.Header.Set("Content-Type", "application/x-www-form-urlencoded") // Common for form data
        }


		// Add Custom Cookies
		for _, cookie := range r.Config.CustomCookies {
			req.AddCookie(&cookie)
		}

		// Log pre-request (headers logged here are as prepared, before client.Do might modify them e.g. for Host)
		preReqLogEntry := utils.LogEntry{
			SessionID:     sessionID,
			Message:       fmt.Sprintf("Attempting report (attempt %d/%d)", attempt+1, r.Config.MaxRetries),
			ReportURL:     targetURL,
			Proxy:         selectedProxy.URL.String(),
			UserAgent:     req.Header.Get("User-Agent"),
			RequestMethod: req.Method,
			RequestHeaders: req.Header.Clone(), // Clone to log the state before Do
			RequestBody:   reqBodyStr,       // Log the string version of the body
		}
		r.Logger.Info(preReqLogEntry)

		// Execute Request
		startTime := time.Now()
		resp, err := r.HTTPClient.Do(req)
		latency := time.Since(startTime)

		logEntry := utils.LogEntry{ // Prepare log entry, fill details as they become available
			SessionID:       sessionID,
			Message:         "Report attempt completed",
			ReportURL:       targetURL,
			Proxy:           selectedProxy.URL.String(),
			UserAgent:       req.Header.Get("User-Agent"),
			RequestMethod:   req.Method,
			RequestHeaders:  preReqLogEntry.RequestHeaders, // Log headers as sent
			RequestBody:     reqBodyStr,
		}

		if err != nil {
			lastErr = fmt.Errorf("attempt %d: request failed: %w", attempt+1, err)
			logEntry.Error = err.Error()
			logEntry.Outcome = "failed_request_error"
			r.Logger.Error(logEntry)

			// Update proxy status if error seems proxy-related (e.g., connection refused, timeout)
            // This is a heuristic. More specific error checking is better.
			if urlErr, ok := err.(*url.Error); ok && (urlErr.Timeout() || urlErr.Temporary()){
                 r.ProxyMgr.UpdateProxyStatus(selectedProxy.URL.String(), "unhealthy", latency)
            } else if strings.Contains(err.Error(), "connect: connection refused") || strings.Contains(err.Error(), "proxyconnect") {
				r.ProxyMgr.UpdateProxyStatus(selectedProxy.URL.String(), "unhealthy", latency)
			}


			if attempt < r.Config.MaxRetries-1 {
				time.Sleep(time.Duration(rand.Intn(2)+1) * time.Second) // Simple backoff
				continue
			}
			return lastErr // All retries failed
		}
		defer resp.Body.Close()

		bodyBytes, readErr := io.ReadAll(resp.Body)
		responseBodyStr := string(bodyBytes) // Store for AI analysis and logging

		if readErr != nil {
			lastErr = fmt.Errorf("attempt %d: failed to read response body: %w", attempt+1, readErr)
			logEntry.Error = readErr.Error()
			logEntry.Outcome = "failed_read_body"
			logEntry.ResponseStatus = resp.StatusCode
			logEntry.ResponseHeaders = resp.Header.Clone()
			// logEntry.ResponseBody will be set below if readErr is nil
			r.Logger.Error(logEntry)
			// Don't necessarily mark proxy as bad for this, could be server issue
			if attempt < r.Config.MaxRetries-1 {
				continue
			}
			return lastErr
		}

		logEntry.ResponseStatus = resp.StatusCode
		logEntry.ResponseHeaders = resp.Header.Clone()
		logEntry.ResponseBody = responseBodyStr // Be cautious with very large bodies in real scenarios
		logEntry.LogID = resp.Header.Get("X-Tt-Logid") // Common TikTok log ID header, adjust if different

		// AI Analysis Hook
		if r.AIAnalyzer != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 { // Only analyze successful fetches for now
			// Simulate extracting relevant text. For now, using a part of the response body.
			// In a real scenario, this would parse HTML/JSON to find user-generated content.
			// Using a placeholder postID as well.
			simulatedPostID := "post123_" + targetURL[len(targetURL)-10:]
			// Use a portion of the body or a placeholder. For dummy, even a fixed string works.
			// Let's assume the first 500 chars of response might contain something interesting.
			analysisText := responseBodyStr
			if len(analysisText) > 500 {
				analysisText = analysisText[:500]
			}
			if analysisText == "" {
				analysisText = "No textual content found in response to analyze."
			}


			aiResult, aiErr := r.AIAnalyzer.Analyze(sessionID, simulatedPostID, analysisText)
			if aiErr != nil {
				r.Logger.Error(utils.LogEntry{
					SessionID: sessionID, Message: "AI analysis failed", ReportURL: targetURL, Error: aiErr.Error(),
					AdditionalData: map[string]interface{}{"post_id": simulatedPostID},
				})
			} else if aiResult != nil {
				logEntry.AdditionalData = make(map[string]interface{}) // Ensure map is initialized
				logEntry.AdditionalData["AIThreatScore"] = aiResult.ThreatScore
				logEntry.AdditionalData["AICategory"] = aiResult.Category
				if len(aiResult.Details) > 0 {
					logEntry.AdditionalData["AIDetails"] = aiResult.Details
				}
				r.Logger.Info(utils.LogEntry{ // Log AI results separately or append to main entry
					SessionID: sessionID, Message: "AI Analysis Result", ReportURL: targetURL,
					AdditionalData: map[string]interface{}{
						"post_id": simulatedPostID, "threat_score": aiResult.ThreatScore, "category": aiResult.Category,
					},
				})

				if aiResult.ThreatScore > r.Config.RiskThreshold {
					r.Logger.Warn(utils.LogEntry{
						SessionID: sessionID, Message: "AI detected high risk content!", ReportURL: targetURL,
						AdditionalData: map[string]interface{}{
							"post_id": simulatedPostID, "threat_score": aiResult.ThreatScore, "category": aiResult.Category, "threshold": r.Config.RiskThreshold,
						},
						Outcome: "high_risk_detected",
					})
					// Future: Trigger conditional action, e.g., prioritize report, notify admin
				}
			}
		}


		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logEntry.Outcome = "accepted"
			r.Logger.Info(logEntry) // Main log entry for the report outcome
			// Optionally update proxy status to healthy if it was previously marked otherwise
			// r.ProxyMgr.UpdateProxyStatus(selectedProxy.URL.String(), "healthy", latency)
			return nil // Report successful
		}

		// Non-2xx status code
		lastErr = fmt.Errorf("attempt %d: report failed with status %d", attempt+1, resp.StatusCode)
		if logEntry.Error == "" { // Only set if not already set by readErr or aiErr
			logEntry.Error = fmt.Sprintf("status code %d", resp.StatusCode)
		}
		logEntry.Outcome = "failed_status_code"
		r.Logger.Error(logEntry) // Main log entry for the report outcome

        // Consider specific status codes for proxy health
        // e.g. 403, 407, 50x from proxy might indicate proxy issue
        if resp.StatusCode == http.StatusProxyAuthRequired || resp.StatusCode == http.StatusForbidden {
            r.ProxyMgr.UpdateProxyStatus(selectedProxy.URL.String(), "unhealthy", latency)
        }


		if attempt < r.Config.MaxRetries-1 {
			// Optional: check for specific status codes that shouldn't be retried
			// e.g. if status 400 (Bad Request), retrying with same data might be pointless
			continue
		}
		return lastErr // All retries failed or final attempt failed
	}
	return lastErr // Should be unreachable if MaxRetries > 0, but good for safety
}
