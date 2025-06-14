package ai

import (
	"fmt"
	"math/rand" // For dummy analyzer randomness
	"strings"
	"time" // For seeding random

	"sentinelgo/utils"
)

// AnalysisResult holds the outcome of an AI content analysis.
// It includes a threat score, a category, detailed findings, and any error encountered.
type AnalysisResult struct {
	// ThreatScore is a numerical value (typically 0-100) representing the assessed risk or threat level.
	ThreatScore float64 `json:"threat_score"`
	// Category is a string label for the type of content identified (e.g., "Propaganda", "Incitement", "None").
	Category string `json:"category"`
	// Details provides a map for any additional structured information returned by the analyzer
	// (e.g., matched keywords, specific rule violations).
	Details map[string]interface{} `json:"details,omitempty"`
	// Error holds any error that occurred during the analysis process.
	Error error `json:"error,omitempty"`
}

// ContentAnalyzer defines the interface for services that perform content analysis.
// This allows for different AI backends or analysis techniques to be plugged into the system.
type ContentAnalyzer interface {
	// Analyze processes the given content text and returns an AnalysisResult.
	//
	// Parameters:
	//   - sessionID: The ID of the current reporting session, for logging and context.
	//   - postID: A unique identifier for the content being analyzed (e.g., a TikTok post ID).
	//   - contentText: The actual text content to be analyzed.
	//
	// Returns:
	//   - A pointer to an `AnalysisResult` struct containing the analysis outcome.
	//   - An error if the analysis process itself fails.
	Analyze(sessionID string, postID string, contentText string) (*AnalysisResult, error)
}

// DummyAnalyzer is a placeholder implementation of the ContentAnalyzer interface.
// It simulates content analysis, useful for development and testing without a live AI backend.
// It uses simple keyword spotting and random scoring.
type DummyAnalyzer struct {
	Logger *utils.Logger // Optional logger for DummyAnalyzer's own operations.
	rng    *rand.Rand    // Local random number generator.
}

// NewDummyAnalyzer creates and returns a new DummyAnalyzer.
// If a logger is provided, it will be used for logging the analyzer's actions.
// It initializes its own random number generator.
func NewDummyAnalyzer(logger *utils.Logger) *DummyAnalyzer {
	source := rand.NewSource(time.Now().UnixNano())
	localRng := rand.New(source)
	return &DummyAnalyzer{Logger: logger, rng: localRng}
}

// Analyze performs a simulated analysis of the provided `contentText`.
// It logs the analysis attempt and outcome if a logger is configured.
// The analysis logic is a basic placeholder:
//   - It checks for specific keywords to assign categories like "Potential Incitement/Scaremongering"
//     or "Misinformation/Propaganda" and assigns a semi-random threat score.
//   - If no keywords are matched, it assigns a lower score based on content length or a default benign category.
//   - Empty content gets a score of 0 and "No Content" category.
//
// This method is intended for demonstration and testing; it does not perform real AI analysis.
func (da *DummyAnalyzer) Analyze(sessionID string, postID string, contentText string) (*AnalysisResult, error) {
	if da.Logger != nil {
		da.Logger.Info(utils.LogEntry{
			SessionID: sessionID,
			Message:   fmt.Sprintf("Performing dummy AI analysis for post ID: %s", postID),
			AdditionalData: map[string]interface{}{"content_snippet": firstNChars(contentText, 70)}, // Log a slightly longer snippet
		})
	}

	result := &AnalysisResult{
		Details: make(map[string]interface{}),
	}

	lowerContent := strings.ToLower(contentText)
	// More distinct keyword sets
	if strings.Contains(lowerContent, "attack now") || strings.Contains(lowerContent, "must fight") || strings.Contains(lowerContent, "eliminate them") {
		result.ThreatScore = 80.0 + da.rng.Float64()*20.0 // 80-100
		result.Category = "High-Risk: Incitement"
		result.Details["matched_keywords"] = []string{"attack now/must fight/eliminate them"}
	} else if strings.Contains(lowerContent, "urgent warning") || strings.Contains(lowerContent, "danger ahead") || strings.Contains(lowerContent, "total collapse") {
		result.ThreatScore = 70.0 + da.rng.Float64()*15.0 // 70-85
		result.Category = "Potential Scaremongering"
		result.Details["matched_keywords"] = []string{"urgent warning/danger ahead/total collapse"}
	} else if strings.Contains(lowerContent, "secret government plan") || strings.Contains(lowerContent, "this is a hoax") || strings.Contains(lowerContent, "they are lying") {
		result.ThreatScore = 60.0 + da.rng.Float64()*20.0 // 60-80
		result.Category = "Misinformation/Conspiracy"
		result.Details["matched_keywords"] = []string{"secret government plan/this is a hoax/they are lying"}
	} else if len(contentText) > 200 { // Slightly longer content considered more for "General"
		result.ThreatScore = 20.0 + da.rng.Float64()*30.0 // 20-50
		result.Category = "General Content (Long)"
	} else if contentText == "" {
		result.ThreatScore = 0.0
		result.Category = "No Content"
		result.Details["info"] = "Content was empty and not analyzed."
	} else if len(contentText) > 50 { // Moderate length content
		result.ThreatScore = 10.0 + da.rng.Float64()*20.0 // 10-30
		result.Category = "General Content (Short)"
	} else { // Very short or benign
		result.ThreatScore = 0.0 + da.rng.Float64()*10.0 // 0-10
		result.Category = "Low Impact / Benign"
	}

	// Ensure score is capped at 100
	if result.ThreatScore > 100.0 { result.ThreatScore = 100.0 }
	if result.ThreatScore < 0.0 { result.ThreatScore = 0.0 }


	if da.Logger != nil {
		logData := map[string]interface{}{
			"post_id": postID,
			"threat_score": fmt.Sprintf("%.2f", result.ThreatScore), // Format float for logging
			"category": result.Category,
		}
		if len(result.Details) > 0 {
			logData["ai_details"] = result.Details
		}
		da.Logger.Info(utils.LogEntry{
			SessionID: sessionID,
			Message:   "Dummy AI Analysis complete.",
			AdditionalData: logData,
		})
	}

	return result, nil
}

// firstNChars returns the first N characters of a string, adding "..." if truncated.
// Useful for logging snippets of potentially long content.
func firstNChars(s string, n int) string {
	if n < 0 { n = 0 } // Ensure n is not negative
	runes := []rune(s) // Use runes to handle multi-byte characters correctly
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
