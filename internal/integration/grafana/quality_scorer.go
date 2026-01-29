package grafana

import (
	"math"
	"strings"
	"time"
)

// DashboardQuality represents the five factors used to compute dashboard quality.
// Each factor is normalized to 0-1 range.
type DashboardQuality struct {
	// Freshness: 0-1, based on last modified time
	// 90 days or less = 1.0, linear decay to 0.0 at 365 days
	Freshness float64

	// RecentUsage: 0 or 1, binary check for views in last 30 days
	// Requires Grafana Stats API, gracefully handles absence
	RecentUsage float64

	// HasAlerts: 0 or 1, binary check for attached alert rules
	HasAlerts float64

	// Ownership: 1.0 for team folder, 0.5 for "General"
	// Team folders indicate ownership and maintenance
	Ownership float64

	// Completeness: 0-1, based on description and meaningful panel titles
	// 0.5 for description, 0.5 for >50% panels with meaningful titles
	Completeness float64
}

// ComputeDashboardQuality computes quality score from dashboard metadata.
//
// Formula: base = avg(Freshness, RecentUsage, Ownership, Completeness) / 4
//          alertBoost = HasAlerts * 0.2
//          quality = min(1.0, base + alertBoost)
//
// Quality tiers:
//   - high: >= 0.7
//   - medium: >= 0.4
//   - low: < 0.4
//
// Parameters:
//   - dashboard: Dashboard metadata with Updated timestamp, FolderTitle, and Panels
//   - alertRuleCount: Number of alert rules attached to dashboard (0 if none)
//   - viewsLast30Days: View count from Grafana Stats API (0 if unavailable)
//
// Returns quality score (0.0-1.0)
func ComputeDashboardQuality(dashboard *GrafanaDashboard, alertRuleCount int, viewsLast30Days int, updated time.Time, folderTitle string, description string) float64 {
	q := DashboardQuality{}

	// Freshness: linear decay from 90 to 365 days
	daysSinceModified := time.Since(updated).Hours() / 24
	if daysSinceModified <= 90 {
		q.Freshness = 1.0
	} else if daysSinceModified >= 365 {
		q.Freshness = 0.0
	} else {
		// Linear interpolation: 1.0 at 90 days, 0.0 at 365 days
		q.Freshness = 1.0 - (daysSinceModified-90)/(365-90)
	}

	// RecentUsage: binary check (gracefully handle missing Stats API)
	if viewsLast30Days > 0 {
		q.RecentUsage = 1.0
	}

	// HasAlerts: binary check
	if alertRuleCount > 0 {
		q.HasAlerts = 1.0
	}

	// Ownership: team folder vs General
	if folderTitle != "" && folderTitle != "General" {
		q.Ownership = 1.0
	} else {
		q.Ownership = 0.5
	}

	// Completeness: description + meaningful panel titles
	completeness := 0.0
	if description != "" {
		completeness += 0.5
	}
	if dashboard != nil {
		meaningfulTitles := countMeaningfulPanelTitles(dashboard.Panels)
		if len(dashboard.Panels) > 0 && float64(meaningfulTitles)/float64(len(dashboard.Panels)) > 0.5 {
			completeness += 0.5
		}
	}
	q.Completeness = completeness

	// Formula: base = avg(4 factors), alertBoost = 0.2 if alerts exist
	base := (q.Freshness + q.RecentUsage + q.Ownership + q.Completeness) / 4.0
	alertBoost := q.HasAlerts * 0.2
	quality := math.Min(1.0, base+alertBoost)

	return quality
}

// countMeaningfulPanelTitles counts panels with non-default, non-empty titles.
// Meaningful = not empty, not "Panel Title", not generic placeholders.
func countMeaningfulPanelTitles(panels []GrafanaPanel) int {
	count := 0
	for _, panel := range panels {
		if isMeaningfulTitle(panel.Title) {
			count++
		}
	}
	return count
}

// isMeaningfulTitle checks if a panel title is meaningful (not default/empty).
func isMeaningfulTitle(title string) bool {
	if title == "" {
		return false
	}
	lowerTitle := strings.ToLower(strings.TrimSpace(title))
	// Default Grafana panel title
	if lowerTitle == "panel title" {
		return false
	}
	// Common placeholders
	placeholders := []string{"untitled", "new panel", "panel", "graph"}
	for _, placeholder := range placeholders {
		if lowerTitle == placeholder {
			return false
		}
	}
	return true
}

// QualityTier returns the quality tier (high/medium/low) based on score.
func QualityTier(score float64) string {
	if score >= 0.7 {
		return "high"
	} else if score >= 0.4 {
		return "medium"
	}
	return "low"
}
