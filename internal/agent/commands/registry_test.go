package commands

import "testing"

func TestParseCommand_ValidCommand(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantArgs []string
	}{
		{"/help", "help", nil},
		{"/stats", "stats", nil},
		{"/pin 1", "pin", []string{"1"}},
		{"/export myfile.md", "export", []string{"myfile.md"}},
		{"/compact some prompt text", "compact", []string{"some", "prompt", "text"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd := ParseCommand(tt.input)
			if cmd == nil {
				t.Fatal("expected command, got nil")
			}
			if cmd.Name != tt.wantName {
				t.Errorf("name = %q, want %q", cmd.Name, tt.wantName)
			}
			if len(cmd.Args) != len(tt.wantArgs) {
				t.Errorf("args len = %d, want %d", len(cmd.Args), len(tt.wantArgs))
			}
			for i := range cmd.Args {
				if cmd.Args[i] != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, cmd.Args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestParseCommand_NotACommand(t *testing.T) {
	tests := []string{
		"hello",
		"not a command",
		"",
		"/",       // empty command
		"  /help", // whitespace before slash
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			cmd := ParseCommand(input)
			if cmd != nil {
				t.Errorf("expected nil for %q, got %+v", input, cmd)
			}
		})
	}
}

func TestRegistry_AllEntries(t *testing.T) {
	entries := DefaultRegistry.AllEntries()
	if len(entries) == 0 {
		t.Error("expected entries, got none")
	}

	// Verify help command is registered
	found := false
	for _, e := range entries {
		if e.Name == "help" {
			found = true
			break
		}
	}
	if !found {
		t.Error("help command not found in registry")
	}
}

func TestRegistry_FuzzyMatch_ExactPrefix(t *testing.T) {
	matches := DefaultRegistry.FuzzyMatch("he")
	if len(matches) == 0 {
		t.Fatal("expected matches for 'he'")
	}
	if matches[0].Name != "help" {
		t.Errorf("first match = %q, want 'help'", matches[0].Name)
	}
}

func TestRegistry_FuzzyMatch_Empty(t *testing.T) {
	matches := DefaultRegistry.FuzzyMatch("")
	entries := DefaultRegistry.AllEntries()
	if len(matches) != len(entries) {
		t.Errorf("empty query should return all entries, got %d want %d", len(matches), len(entries))
	}
}

func TestRegistry_Execute_UnknownCommand(t *testing.T) {
	ctx := &Context{}
	cmd := &Command{Name: "nonexistent", Args: nil}
	result := DefaultRegistry.Execute(ctx, cmd)
	if result.Success {
		t.Error("expected failure for unknown command")
	}
	if result.Message == "" {
		t.Error("expected error message")
	}
}

func TestRegistry_Execute_Help(t *testing.T) {
	ctx := &Context{}
	cmd := &Command{Name: "help", Args: nil}
	result := DefaultRegistry.Execute(ctx, cmd)
	if !result.Success {
		t.Errorf("help command failed: %s", result.Message)
	}
	if !result.IsInfo {
		t.Error("help should be an info message")
	}
}

func TestRegistry_Execute_Stats(t *testing.T) {
	ctx := &Context{
		SessionID:         "test-session",
		TotalLLMRequests:  5,
		TotalInputTokens:  1000,
		TotalOutputTokens: 500,
	}
	cmd := &Command{Name: "stats", Args: nil}
	result := DefaultRegistry.Execute(ctx, cmd)
	if !result.Success {
		t.Errorf("stats command failed: %s", result.Message)
	}
}

func TestRegistry_Execute_PinInvalidArgs(t *testing.T) {
	ctx := &Context{}
	cmd := &Command{Name: "pin", Args: []string{"not-a-number"}}
	result := DefaultRegistry.Execute(ctx, cmd)
	if result.Success {
		t.Error("expected failure for invalid pin argument")
	}
}
