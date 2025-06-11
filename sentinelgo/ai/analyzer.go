package ai

import (
	"fmt"
	"math/rand" // For dummy analyzer randomness
	"strings"

	"sentinelgo/sentinelgo/utils"
)

// AnalysisResult holds the outcome of content analysis.
type AnalysisResult struct {
	ThreatScore float64                `json:"threat_score"`
	Category    string                 `json:"category"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Error       error                  `json:"error,omitempty"`
}

// ContentAnalyzer defines the interface for AI content analysis.
type ContentAnalyzer interface {
	Analyze(sessionID string, postID string, contentText string) (*AnalysisResult, error)
}

// DummyAnalyzer is a placeholder implementation of ContentAnalyzer.
type DummyAnalyzer struct {
	Logger *utils.Logger
}

// NewDummyAnalyzer creates a new DummyAnalyzer.
func NewDummyAnalyzer(logger *utils.Logger) *DummyAnalyzer {
	return &DummyAnalyzer{Logger: logger}
}

// Analyze performs a dummy analysis of the content.
func (da *DummyAnalyzer) Analyze(sessionID string, postID string, contentText string) (*AnalysisResult, error) {
	if da.Logger != nil {
		da.Logger.Info(utils.LogEntry{
			SessionID:      sessionID,
			Message:        fmt.Sprintf("Performing dummy AI analysis for post ID: %s", postID),
			AdditionalData: map[string]interface{}{"content_snippet": firstNChars(contentText, 50)},
		})
	}

	result := &AnalysisResult{
		Details: make(map[string]interface{}),
	}

	// Simple keyword spotting for dummy logic
	lowerContent := strings.ToLower(contentText)
	if strings.Contains(lowerContent, "urgent") || strings.Contains(lowerContent, "alert") || strings.Contains(lowerContent, "danger") || strings.Contains(lowerContent, "warning") {
		result.ThreatScore = 70.0 + rand.Float64()*20.0 // 70-90
		result.Category = "Potential Incitement/Scaremongering"
		result.Details["matched_keywords"] = []string{"urgent/alert/danger/warning"}
	} else if strings.Contains(lowerContent, "fake news") || strings.Contains(lowerContent, "conspiracy") || strings.Contains(lowerContent, "hoax") {
		result.ThreatScore = 60.0 + rand.Float64()*25.0 // 60-85
		result.Category = "Misinformation/Propaganda"
		result.Details["matched_keywords"] = []string{"fake news/conspiracy/hoax"}
	} else if len(contentText) > 100 { // Longer content slightly higher base
		result.ThreatScore = 20.0 + rand.Float64()*30.0 // 20-50
		result.Category = "General Content"
	} else if contentText == "" {
		result.ThreatScore = 0.0
		result.Category = "No Content"
		result.Details["info"] = "Content was empty."
	} else {
		result.ThreatScore = 5.0 + rand.Float64()*25.0 // 5-30
		result.Category = "Low Impact / Benign"
	}

	if result.ThreatScore > 100.0 {
		result.ThreatScore = 100.0
	}

	if da.Logger != nil {
		da.Logger.Info(utils.LogEntry{
			SessionID: sessionID,
			Message:   fmt.Sprintf("Dummy AI Analysis complete for post ID: %s. Score: %.2f, Category: %s", postID, result.ThreatScore, result.Category),
		})
	}

	return result, nil
}

func firstNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
