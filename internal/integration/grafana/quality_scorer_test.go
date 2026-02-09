package grafana

import (
	"math"
	"testing"
	"time"
)

func TestComputeDashboardQuality_Freshness(t *testing.T) {
	tests := []struct {
		name         string
		daysAgo      float64
		expectedFreshness float64
	}{
		{
			name:         "0 days old → 1.0",
			daysAgo:      0,
			expectedFreshness: 1.0,
		},
		{
			name:         "45 days old → 1.0",
			daysAgo:      45,
			expectedFreshness: 1.0,
		},
		{
			name:         "90 days old → 1.0",
			daysAgo:      90,
			expectedFreshness: 1.0,
		},
		{
			name:         "180 days old → ~0.67",
			daysAgo:      180,
			expectedFreshness: 1.0 - (180.0-90.0)/(365.0-90.0), // ~0.6727
		},
		{
			name:         "270 days old → ~0.35",
			daysAgo:      270,
			expectedFreshness: 1.0 - (270.0-90.0)/(365.0-90.0), // ~0.3455
		},
		{
			name:         "365 days old → 0.0",
			daysAgo:      365,
			expectedFreshness: 0.0,
		},
		{
			name:         "500 days old → 0.0",
			daysAgo:      500,
			expectedFreshness: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := time.Now().Add(-time.Duration(tt.daysAgo*24) * time.Hour)
			dashboard := &GrafanaDashboard{Panels: []GrafanaPanel{}}

			quality := ComputeDashboardQuality(dashboard, 0, 0, updated, "General", "")

			// Freshness is 1/4 of base score when other factors are 0
			// base = (Freshness + 0 + 0.5 + 0) / 4 = (Freshness + 0.5) / 4
			// quality = base (no alert boost)
			expectedQuality := (tt.expectedFreshness + 0.5) / 4.0

			if math.Abs(quality-expectedQuality) > 0.01 {
				t.Errorf("expected quality %.4f, got %.4f (freshness should be %.4f)", expectedQuality, quality, tt.expectedFreshness)
			}
		})
	}
}

func TestComputeDashboardQuality_RecentUsage(t *testing.T) {
	tests := []struct {
		name            string
		viewsLast30Days int
		expectedUsage   float64
	}{
		{
			name:            "no views → 0.0",
			viewsLast30Days: 0,
			expectedUsage:   0.0,
		},
		{
			name:            "1 view → 1.0",
			viewsLast30Days: 1,
			expectedUsage:   1.0,
		},
		{
			name:            "100 views → 1.0",
			viewsLast30Days: 100,
			expectedUsage:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := time.Now().Add(-30 * 24 * time.Hour) // 30 days old
			dashboard := &GrafanaDashboard{Panels: []GrafanaPanel{}}

			quality := ComputeDashboardQuality(dashboard, 0, tt.viewsLast30Days, updated, "General", "")

			// base = (Freshness + RecentUsage + Ownership + Completeness) / 4
			// Freshness at 30 days = 1.0, Ownership for "General" = 0.5, Completeness = 0
			expectedBase := (1.0 + tt.expectedUsage + 0.5 + 0.0) / 4.0
			expectedQuality := expectedBase

			if math.Abs(quality-expectedQuality) > 0.01 {
				t.Errorf("expected quality %.4f, got %.4f", expectedQuality, quality)
			}
		})
	}
}

func TestComputeDashboardQuality_HasAlerts(t *testing.T) {
	tests := []struct {
		name           string
		alertRuleCount int
		expectedBoost  float64
	}{
		{
			name:           "no alerts → 0.0 boost",
			alertRuleCount: 0,
			expectedBoost:  0.0,
		},
		{
			name:           "1 alert → 0.2 boost",
			alertRuleCount: 1,
			expectedBoost:  0.2,
		},
		{
			name:           "5 alerts → 0.2 boost",
			alertRuleCount: 5,
			expectedBoost:  0.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := time.Now().Add(-30 * 24 * time.Hour) // 30 days old
			dashboard := &GrafanaDashboard{Panels: []GrafanaPanel{}}

			quality := ComputeDashboardQuality(dashboard, tt.alertRuleCount, 0, updated, "General", "")

			// base = (1.0 + 0.0 + 0.5 + 0.0) / 4 = 0.375
			// quality = min(1.0, base + boost)
			expectedBase := 0.375
			expectedQuality := math.Min(1.0, expectedBase+tt.expectedBoost)

			if math.Abs(quality-expectedQuality) > 0.01 {
				t.Errorf("expected quality %.4f, got %.4f", expectedQuality, quality)
			}
		})
	}
}

func TestComputeDashboardQuality_Ownership(t *testing.T) {
	tests := []struct {
		name              string
		folderTitle       string
		expectedOwnership float64
	}{
		{
			name:              "General folder → 0.5",
			folderTitle:       "General",
			expectedOwnership: 0.5,
		},
		{
			name:              "empty folder → 0.5",
			folderTitle:       "",
			expectedOwnership: 0.5,
		},
		{
			name:              "team folder → 1.0",
			folderTitle:       "Platform Team",
			expectedOwnership: 1.0,
		},
		{
			name:              "another team folder → 1.0",
			folderTitle:       "SRE",
			expectedOwnership: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := time.Now().Add(-30 * 24 * time.Hour) // 30 days old
			dashboard := &GrafanaDashboard{Panels: []GrafanaPanel{}}

			quality := ComputeDashboardQuality(dashboard, 0, 0, updated, tt.folderTitle, "")

			// base = (1.0 + 0.0 + Ownership + 0.0) / 4
			expectedBase := (1.0 + 0.0 + tt.expectedOwnership + 0.0) / 4.0
			expectedQuality := expectedBase

			if math.Abs(quality-expectedQuality) > 0.01 {
				t.Errorf("expected quality %.4f, got %.4f", expectedQuality, quality)
			}
		})
	}
}

func TestComputeDashboardQuality_Completeness(t *testing.T) {
	tests := []struct {
		name                 string
		description          string
		panels               []GrafanaPanel
		expectedCompleteness float64
	}{
		{
			name:                 "no description, no panels → 0.0",
			description:          "",
			panels:               []GrafanaPanel{},
			expectedCompleteness: 0.0,
		},
		{
			name:        "description only → 0.5",
			description: "This is a dashboard",
			panels:      []GrafanaPanel{},
			expectedCompleteness: 0.5,
		},
		{
			name:        "description + default titles → 0.5",
			description: "This is a dashboard",
			panels: []GrafanaPanel{
				{Title: "Panel Title"},
				{Title: "Panel Title"},
			},
			expectedCompleteness: 0.5,
		},
		{
			name:        "description + meaningful titles → 1.0",
			description: "This is a dashboard",
			panels: []GrafanaPanel{
				{Title: "CPU Usage"},
				{Title: "Memory Usage"},
			},
			expectedCompleteness: 1.0,
		},
		{
			name:        "no description + meaningful titles → 0.5",
			description: "",
			panels: []GrafanaPanel{
				{Title: "CPU Usage"},
				{Title: "Memory Usage"},
			},
			expectedCompleteness: 0.5,
		},
		{
			name:        "description + 50% meaningful → 0.5 (threshold not met)",
			description: "This is a dashboard",
			panels: []GrafanaPanel{
				{Title: "CPU Usage"},
				{Title: "Panel Title"},
			},
			expectedCompleteness: 0.5,
		},
		{
			name:        "description + >50% meaningful → 1.0",
			description: "This is a dashboard",
			panels: []GrafanaPanel{
				{Title: "CPU Usage"},
				{Title: "Memory Usage"},
				{Title: "Panel Title"},
			},
			expectedCompleteness: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := time.Now().Add(-30 * 24 * time.Hour) // 30 days old
			dashboard := &GrafanaDashboard{Panels: tt.panels}

			quality := ComputeDashboardQuality(dashboard, 0, 0, updated, "General", tt.description)

			// base = (1.0 + 0.0 + 0.5 + Completeness) / 4
			expectedBase := (1.0 + 0.0 + 0.5 + tt.expectedCompleteness) / 4.0
			expectedQuality := expectedBase

			if math.Abs(quality-expectedQuality) > 0.01 {
				t.Errorf("expected quality %.4f, got %.4f (completeness should be %.2f)", expectedQuality, quality, tt.expectedCompleteness)
			}
		})
	}
}

func TestComputeDashboardQuality_AlertBoostCapped(t *testing.T) {
	// Test that alert boost caps at 1.0
	t.Run("alert boost caps at 1.0", func(t *testing.T) {
		updated := time.Now().Add(-30 * 24 * time.Hour) // 30 days old
		dashboard := &GrafanaDashboard{
			Panels: []GrafanaPanel{
				{Title: "CPU Usage"},
				{Title: "Memory Usage"},
			},
		}

		// High base score: Freshness=1.0, RecentUsage=1.0, Ownership=1.0, Completeness=1.0
		// base = 4.0 / 4 = 1.0
		// alertBoost = 0.2
		// quality = min(1.0, 1.0 + 0.2) = 1.0
		quality := ComputeDashboardQuality(dashboard, 1, 100, updated, "Team", "Description")

		if quality != 1.0 {
			t.Errorf("expected quality capped at 1.0, got %.4f", quality)
		}
	})
}

func TestQualityTier(t *testing.T) {
	tests := []struct {
		name         string
		score        float64
		expectedTier string
	}{
		{
			name:         "0.0 → low",
			score:        0.0,
			expectedTier: "low",
		},
		{
			name:         "0.3 → low",
			score:        0.3,
			expectedTier: "low",
		},
		{
			name:         "0.4 → medium",
			score:        0.4,
			expectedTier: "medium",
		},
		{
			name:         "0.6 → medium",
			score:        0.6,
			expectedTier: "medium",
		},
		{
			name:         "0.69 → medium",
			score:        0.69,
			expectedTier: "medium",
		},
		{
			name:         "0.7 → high",
			score:        0.7,
			expectedTier: "high",
		},
		{
			name:         "0.9 → high",
			score:        0.9,
			expectedTier: "high",
		},
		{
			name:         "1.0 → high",
			score:        1.0,
			expectedTier: "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := QualityTier(tt.score)
			if tier != tt.expectedTier {
				t.Errorf("expected tier %s, got %s", tt.expectedTier, tier)
			}
		})
	}
}

func TestIsMeaningfulTitle(t *testing.T) {
	tests := []struct {
		name       string
		title      string
		meaningful bool
	}{
		{
			name:       "empty title → not meaningful",
			title:      "",
			meaningful: false,
		},
		{
			name:       "Panel Title → not meaningful",
			title:      "Panel Title",
			meaningful: false,
		},
		{
			name:       "panel title (lowercase) → not meaningful",
			title:      "panel title",
			meaningful: false,
		},
		{
			name:       "Untitled → not meaningful",
			title:      "Untitled",
			meaningful: false,
		},
		{
			name:       "New Panel → not meaningful",
			title:      "New Panel",
			meaningful: false,
		},
		{
			name:       "Panel → not meaningful",
			title:      "Panel",
			meaningful: false,
		},
		{
			name:       "Graph → not meaningful",
			title:      "Graph",
			meaningful: false,
		},
		{
			name:       "CPU Usage → meaningful",
			title:      "CPU Usage",
			meaningful: true,
		},
		{
			name:       "Error Rate → meaningful",
			title:      "Error Rate",
			meaningful: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMeaningfulTitle(tt.title)
			if result != tt.meaningful {
				t.Errorf("expected %v, got %v", tt.meaningful, result)
			}
		})
	}
}

func TestComputeDashboardQuality_FullFormula(t *testing.T) {
	// Test the full formula with all factors
	t.Run("full quality computation", func(t *testing.T) {
		updated := time.Now().Add(-45 * 24 * time.Hour) // 45 days old
		dashboard := &GrafanaDashboard{
			Panels: []GrafanaPanel{
				{Title: "CPU Usage"},
				{Title: "Memory Usage"},
				{Title: "Error Rate"},
			},
		}

		quality := ComputeDashboardQuality(dashboard, 2, 50, updated, "Platform Team", "Production metrics")

		// Expected:
		// Freshness: 45 days = 1.0
		// RecentUsage: 50 views = 1.0
		// HasAlerts: 2 alerts = 1.0
		// Ownership: Team folder = 1.0
		// Completeness: description + 3/3 meaningful = 1.0
		// base = (1.0 + 1.0 + 1.0 + 1.0) / 4 = 1.0
		// alertBoost = 1.0 * 0.2 = 0.2
		// quality = min(1.0, 1.0 + 0.2) = 1.0

		if quality != 1.0 {
			t.Errorf("expected quality 1.0, got %.4f", quality)
		}

		tier := QualityTier(quality)
		if tier != "high" {
			t.Errorf("expected tier high, got %s", tier)
		}
	})
}
