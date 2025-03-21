package utils

import (
	"tracepr/config"
	"fmt"
	"regexp"
	"strings"
)

// ParseLLMSuggestions extracts file-based suggestions from Claude's response
func ParseLLMSuggestionsForObservability(llmResponse string) ([]config.FileSuggestion, error) {
	suggestions := []config.FileSuggestion{}

	// Check if response is LGTM
	if strings.Contains(llmResponse, "LGTM") {
		return suggestions, nil // Return empty suggestions for LGTM case
	}

	// Find all suggestion blocks
	suggestionPattern := regexp.MustCompile(`FILE: (.+?)\nLINE: (\d+)\nSUGGESTION:\n` +
		"```diff\n" + `((?s:.+?))` + "```")
	matches := suggestionPattern.FindAllStringSubmatch(llmResponse, -1)

	for _, match := range matches {
		if len(match) != 4 {
			continue
		}

		fileName := match[1]
		lineNum := match[2]
		diffContent := match[3]

		// Parse the diff content to get actual change
		suggestion := config.FileSuggestion{
			FileName: fileName,
			LineNum:  lineNum,
			Content:  ExtractActualContent(diffContent),
		}

		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

// ParseLLMSuggestionsForDashboards extracts dashboard suggestions from Claude's response
func ParseLLMSuggestionsForDashboards(llmResponse string) ([]config.DashboardSuggestion, error) {
	suggestions := []config.DashboardSuggestion{}

	// Check if response is LGTM
	if strings.Contains(llmResponse, "LGTM") {
		return suggestions, nil
	}

	// Find all dashboard suggestion blocks
	dashboardPattern := regexp.MustCompile(`DASHBOARD: (.+?)\nTYPE: (.+?)\nPRIORITY: (.+?)\nQUERIES:\n` +
		"```json\n" + `((?s:.+?))` + "```\n" +
		`PANELS:\n` + "```json\n" + `((?s:.+?))` + "```\n" +
		`ALERTS:\n` + "```json\n" + `((?s:.+?))` + "```")

	matches := dashboardPattern.FindAllStringSubmatch(llmResponse, -1)

	for _, match := range matches {
		if len(match) != 7 {
			continue
		}

		name := match[1]
		dashboardType := match[2] // "grafana" or "amplitude"
		priority := match[3]
		queries := match[4]
		panels := match[5]
		alerts := match[6]

		suggestion := config.DashboardSuggestion{
			Name:     name,
			Type:     dashboardType,
			Priority: priority,
			Queries:  queries,
			Panels:   panels,
			Alerts:   alerts,
		}

		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

func ParseLLMSuggestionsForAlerts(llmResponse string) ([]config.AlertSuggestion, error) {
	suggestions := []config.AlertSuggestion{}

	// Check if response is LGTM
	if strings.Contains(llmResponse, "LGTM") {
		return suggestions, nil
	}

	// Find all alert suggestion blocks based on the format in BuildAlertsPrompt
	alertPattern := regexp.MustCompile(`ALERT: (.+?)\nTYPE: (.+?)\nPRIORITY: (.+?)\nQUERY:\n` +
		"```\n" + `((?s:.+?))` + "```\n" +
		`DESCRIPTION: (.+?)\n` +
		`THRESHOLD: (.+?)\n` +
		`DURATION: (.+?)\n` +
		`NOTIFICATION: (.+?)\n` +
		`RUNBOOK_LINK: (.+?)(?:\n\n|\n?$)`)

	matches := alertPattern.FindAllStringSubmatch(llmResponse, -1)

	for _, match := range matches {
		if len(match) != 10 {
			continue
		}

		suggestion := config.AlertSuggestion{
			Name:         match[1],
			Type:         match[2],
			Priority:     match[3],
			Query:        strings.TrimSpace(match[4]),
			Description:  match[5],
			Threshold:    match[6],
			Duration:     match[7],
			Notification: match[8],
			RunbookLink:  match[9],
		}

		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

func ParseLLMSummary(llmResponse string) (string, error) {
	// Match everything from "SUMMARY:" to either the next section marker or end of text
	summaryPattern := regexp.MustCompile(`(?s)SUMMARY:\s*(.*?)(?:\n\n##|\n\nFILE:|$)`)

	matches := summaryPattern.FindStringSubmatch(llmResponse)
	if len(matches) < 2 {
		// Try a more permissive pattern as fallback
		summaryPattern = regexp.MustCompile(`(?s)SUMMARY:\s*(.+)$`)
		matches = summaryPattern.FindStringSubmatch(llmResponse)

		if len(matches) < 2 {
			return "", fmt.Errorf("could not extract summary from response")
		}
	}

	// Return the captured content and trim any trailing whitespace
	return strings.TrimSpace(matches[1]), nil
}
