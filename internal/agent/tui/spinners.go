package tui

import (
	"math/rand"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// SpinnerAnimation defines a spinner animation with its frames.
type SpinnerAnimation struct {
	Frames   []string
	Interval time.Duration
}

// Available spinner animations
var spinnerAnimations = []SpinnerAnimation{
	// Braille dots (classic)
	{
		Frames:   []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		Interval: 80 * time.Millisecond,
	},
	// Bouncing ball
	{
		Frames:   []string{"⠁", "⠂", "⠄", "⡀", "⢀", "⠠", "⠐", "⠈"},
		Interval: 100 * time.Millisecond,
	},
	// Growing dots
	{
		Frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		Interval: 80 * time.Millisecond,
	},
	// Arc
	{
		Frames:   []string{"◜", "◠", "◝", "◞", "◡", "◟"},
		Interval: 100 * time.Millisecond,
	},
	// Circle quarters
	{
		Frames:   []string{"◴", "◷", "◶", "◵"},
		Interval: 120 * time.Millisecond,
	},
	// Box bounce
	{
		Frames:   []string{"▖", "▘", "▝", "▗"},
		Interval: 120 * time.Millisecond,
	},
	// Moon phases
	{
		Frames:   []string{"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"},
		Interval: 100 * time.Millisecond,
	},
	// Arrows
	{
		Frames:   []string{"←", "↖", "↑", "↗", "→", "↘", "↓", "↙"},
		Interval: 100 * time.Millisecond,
	},
	// Pulse
	{
		Frames:   []string{"█", "▓", "▒", "░", "▒", "▓"},
		Interval: 120 * time.Millisecond,
	},
}

// Spinner colors for variety
var spinnerColors = []lipgloss.Color{
	lipgloss.Color("#FF79C6"), // Pink
	lipgloss.Color("#8BE9FD"), // Cyan
	lipgloss.Color("#50FA7B"), // Green
	lipgloss.Color("#FFB86C"), // Orange
	lipgloss.Color("#BD93F9"), // Purple
	lipgloss.Color("#F1FA8C"), // Yellow
}

// AnimatedSpinner manages a spinner with random animation and starting frame.
type AnimatedSpinner struct {
	animation  SpinnerAnimation
	frameIndex int
	style      lipgloss.Style
	lastUpdate time.Time
}

// NewAnimatedSpinner creates a new spinner with a random animation and starting frame.
func NewAnimatedSpinner() *AnimatedSpinner {
	// Pick random animation
	// #nosec G404 -- Using math/rand for UI animation variety, not cryptography
	animIdx := rand.Intn(len(spinnerAnimations))
	anim := spinnerAnimations[animIdx]

	// Pick random starting frame
	// #nosec G404 -- Using math/rand for UI animation variety, not cryptography
	startFrame := rand.Intn(len(anim.Frames))

	// Pick random color
	// #nosec G404 -- Using math/rand for UI animation variety, not cryptography
	colorIdx := rand.Intn(len(spinnerColors))
	style := lipgloss.NewStyle().
		Foreground(spinnerColors[colorIdx]).
		Bold(true)

	return &AnimatedSpinner{
		animation:  anim,
		frameIndex: startFrame,
		style:      style,
		lastUpdate: time.Now(),
	}
}

// View returns the current spinner frame with styling.
func (s *AnimatedSpinner) View() string {
	return s.style.Render(s.animation.Frames[s.frameIndex])
}

// Tick advances the spinner to the next frame if enough time has passed.
// Returns true if the frame changed.
func (s *AnimatedSpinner) Tick() bool {
	now := time.Now()
	if now.Sub(s.lastUpdate) >= s.animation.Interval {
		s.frameIndex = (s.frameIndex + 1) % len(s.animation.Frames)
		s.lastUpdate = now
		return true
	}
	return false
}

// SpinnerManager manages multiple spinners for different contexts.
type SpinnerManager struct {
	spinners map[string]*AnimatedSpinner
}

// NewSpinnerManager creates a new spinner manager.
func NewSpinnerManager() *SpinnerManager {
	return &SpinnerManager{
		spinners: make(map[string]*AnimatedSpinner),
	}
}

// Get returns a spinner for the given key, creating one if it doesn't exist.
func (m *SpinnerManager) Get(key string) *AnimatedSpinner {
	if s, ok := m.spinners[key]; ok {
		return s
	}
	s := NewAnimatedSpinner()
	m.spinners[key] = s
	return s
}

// Remove removes a spinner for the given key.
func (m *SpinnerManager) Remove(key string) {
	delete(m.spinners, key)
}

// Clear removes all spinners.
func (m *SpinnerManager) Clear() {
	m.spinners = make(map[string]*AnimatedSpinner)
}

// TickAll advances all spinners. Returns true if any frame changed.
func (m *SpinnerManager) TickAll() bool {
	changed := false
	for _, s := range m.spinners {
		if s.Tick() {
			changed = true
		}
	}
	return changed
}
