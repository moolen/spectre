//go:build disabled

package commands

import (
	"sort"
	"strings"
	"sync"
)

// DefaultRegistry is the global registry for auto-registration via init().
var DefaultRegistry = NewRegistry()

// Registry manages command handlers and provides lookup functionality.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	entries  []Entry // cached for dropdown
}

// NewRegistry creates a new empty command registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
		entries:  nil,
	}
}

// Register adds a handler to the registry.
// The handler's Entry().Name is used as the command name.
func (r *Registry) Register(h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := h.Entry()
	r.handlers[entry.Name] = h
	r.entries = nil // invalidate cache
}

// Execute runs the command with the given context.
// Returns an error result if the command is not found.
func (r *Registry) Execute(ctx *Context, cmd *Command) Result {
	r.mu.RLock()
	handler, ok := r.handlers[cmd.Name]
	r.mu.RUnlock()

	if !ok {
		return Result{
			Success: false,
			Message: "Unknown command: /" + cmd.Name + " (type /help for available commands)",
			IsInfo:  true,
		}
	}

	return handler.Execute(ctx, cmd.Args)
}

// AllEntries returns all registered command entries, sorted by name.
func (r *Registry) AllEntries() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.entries != nil {
		return r.entries
	}

	// Build and cache entries
	entries := make([]Entry, 0, len(r.handlers))
	for _, h := range r.handlers {
		entries = append(entries, h.Entry())
	}

	// Sort by name for consistent ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	r.entries = entries
	return r.entries
}

// FuzzyMatch returns entries that match the query, scored and sorted by relevance.
func (r *Registry) FuzzyMatch(query string) []Entry {
	entries := r.AllEntries()

	if query == "" {
		return entries
	}

	query = strings.ToLower(query)

	type scored struct {
		entry Entry
		score int
	}
	matches := make([]scored, 0, len(entries))

	for _, entry := range entries {
		name := strings.ToLower(entry.Name)
		desc := strings.ToLower(entry.Description)

		score := 0

		// Exact prefix match on name (highest priority)
		if strings.HasPrefix(name, query) {
			// Shorter matches rank higher (exact match = 100, longer = less)
			score = 100 - (len(name) - len(query))
		} else if strings.Contains(name, query) {
			// Substring match on name
			score = 50
		} else if fuzzyContains(name, query) {
			// Fuzzy match on name (characters in order)
			score = 25
		} else if strings.Contains(desc, query) {
			// Match in description
			score = 10
		} else {
			continue // No match
		}

		matches = append(matches, scored{entry, score})
	}

	// Sort by score descending, then alphabetically
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].entry.Name < matches[j].entry.Name
	})

	result := make([]Entry, len(matches))
	for i, m := range matches {
		result[i] = m.entry
	}
	return result
}

// fuzzyContains checks if all characters of query appear in str in order.
func fuzzyContains(str, query string) bool {
	qi := 0
	for _, c := range str {
		if qi < len(query) && c == rune(query[qi]) {
			qi++
		}
	}
	return qi == len(query)
}

// ParseCommand parses a slash command string into a Command.
// Returns nil if the input is not a command (doesn't start with /).
func ParseCommand(input string) *Command {
	if !strings.HasPrefix(input, "/") {
		return nil
	}

	input = strings.TrimPrefix(input, "/")
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	return &Command{
		Name: strings.ToLower(parts[0]),
		Args: parts[1:],
	}
}
