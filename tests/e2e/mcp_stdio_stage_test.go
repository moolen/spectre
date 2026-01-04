package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MCPStdioStage struct {
	t              *testing.T
	require        *require.Assertions
	assert         *assert.Assertions
	testCtx        *helpers.TestContext
	spectreBinary  string
	subprocess     *helpers.MCPSubprocess
	initResult     map[string]interface{}
	tools          []interface{}
	toolCallResult map[string]interface{}
	prompts        []interface{}
	promptResult   map[string]interface{}
}

func NewMCPStdioStage(t *testing.T) (*MCPStdioStage, *MCPStdioStage, *MCPStdioStage) {
	s := &MCPStdioStage{
		t:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
	return s, s, s
}

func (s *MCPStdioStage) and() *MCPStdioStage {
	return s
}

func (s *MCPStdioStage) a_test_environment() *MCPStdioStage {
	s.testCtx = helpers.SetupE2ETestShared(s.t)
	return s
}

func (s *MCPStdioStage) spectre_binary_is_built() *MCPStdioStage {
	binary, err := helpers.BuildSpectreBinary(s.t)
	s.require.NoError(err, "failed to build spectre binary")
	s.spectreBinary = binary

	s.t.Cleanup(func() {
		// Binary cleanup is handled by the test framework
	})

	return s
}

func (s *MCPStdioStage) mcp_subprocess_is_started() *MCPStdioStage {
	spectreURL := s.testCtx.APIClient.BaseURL

	subprocess, err := helpers.StartMCPSubprocess(s.t, s.spectreBinary, spectreURL)
	s.require.NoError(err, "failed to start MCP subprocess")

	s.subprocess = subprocess

	s.t.Cleanup(func() {
		if s.subprocess != nil {
			if err := s.subprocess.Close(); err != nil {
				s.t.Logf("Warning: failed to close subprocess: %v", err)
			}
		}
	})

	return s
}

func (s *MCPStdioStage) subprocess_is_ready() *MCPStdioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Second)
	defer cancel()

	err := helpers.WaitForMCPReady(ctx, s.subprocess, 10*time.Second)
	s.require.NoError(err, "subprocess not ready")

	return s
}

func (s *MCPStdioStage) session_is_initialized() *MCPStdioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Second)
	defer cancel()

	result, err := s.subprocess.Initialize(ctx)
	s.require.NoError(err, "initialize failed")
	s.require.NotNil(result, "initialize result should not be nil")

	s.initResult = result
	return s
}

func (s *MCPStdioStage) server_info_is_correct() *MCPStdioStage {
	s.require.NotNil(s.initResult, "initialize must be called first")

	serverInfo, ok := s.initResult["serverInfo"].(map[string]interface{})
	s.require.True(ok, "serverInfo should be present in initialize result")

	name, ok := serverInfo["name"].(string)
	s.require.True(ok, "serverInfo.name should be a string")
	s.assert.Equal("Spectre MCP Server", name)

	version, ok := serverInfo["version"].(string)
	s.require.True(ok, "serverInfo.version should be a string")
	s.assert.NotEmpty(version)

	return s
}

func (s *MCPStdioStage) capabilities_include_tools_and_prompts() *MCPStdioStage {
	s.require.NotNil(s.initResult, "initialize must be called first")

	capabilities, ok := s.initResult["capabilities"].(map[string]interface{})
	s.require.True(ok, "capabilities should be present in initialize result")

	_, hasTools := capabilities["tools"]
	s.assert.True(hasTools, "capabilities should include tools")

	_, hasPrompts := capabilities["prompts"]
	s.assert.True(hasPrompts, "capabilities should include prompts")

	return s
}

func (s *MCPStdioStage) tools_are_listed() *MCPStdioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Second)
	defer cancel()

	tools, err := s.subprocess.ListTools(ctx)
	s.require.NoError(err, "list tools failed")
	s.require.NotNil(tools, "tools should not be nil")

	s.tools = tools
	return s
}

func (s *MCPStdioStage) four_tools_are_available() *MCPStdioStage {
	s.require.NotNil(s.tools, "tools must be listed first")
	// Tools can be 4 (base tools only) or 6 (with graph tools)
	// Graph tools are conditional on --graph-enabled flag
	toolCount := len(s.tools)
	s.assert.True(toolCount == 4 || toolCount == 6, 
		"should have 4 tools (base) or 6 tools (with graph), got %d", toolCount)
	return s
}

func (s *MCPStdioStage) expected_tools_are_present() *MCPStdioStage {
	s.require.NotNil(s.tools, "tools must be listed first")

	// Base tools that should always be present
	baseTools := map[string]bool{
		"cluster_health":    false,
		"resource_changes":  false,
		"investigate":       false,
		"resource_explorer": false,
	}

	// Graph tools are conditional (only present if --graph-enabled)
	graphTools := map[string]bool{
		"find_root_cause":       false,
		"calculate_blast_radius": false,
	}

	// Convert tools to JSON and back to get proper types
	toolsJSON, err := json.Marshal(s.tools)
	s.require.NoError(err, "failed to marshal tools")

	var toolsList []map[string]interface{}
	err = json.Unmarshal(toolsJSON, &toolsList)
	s.require.NoError(err, "failed to unmarshal tools")

	for _, tool := range toolsList {
		name, ok := tool["name"].(string)
		if ok {
			if _, expected := baseTools[name]; expected {
				baseTools[name] = true
			}
			if _, expected := graphTools[name]; expected {
				graphTools[name] = true
			}
		}
	}

	// Assert all base tools are present
	for toolName, found := range baseTools {
		s.assert.True(found, "expected base tool %s to be present", toolName)
	}

	// Graph tools are optional - just log if present
	hasGraphTools := false
	for toolName, found := range graphTools {
		if found {
			hasGraphTools = true
			s.t.Logf("✓ Graph tool %s is available", toolName)
		}
	}
	
	if !hasGraphTools {
		s.t.Log("ℹ Graph tools not available (--graph-enabled not set)")
	}

	return s
}

func (s *MCPStdioStage) each_tool_has_description_and_schema() *MCPStdioStage {
	s.require.NotNil(s.tools, "tools must be listed first")

	// Convert tools to JSON and back to get proper types
	toolsJSON, err := json.Marshal(s.tools)
	s.require.NoError(err, "failed to marshal tools")

	var toolsList []map[string]interface{}
	err = json.Unmarshal(toolsJSON, &toolsList)
	s.require.NoError(err, "failed to unmarshal tools")

	for _, tool := range toolsList {
		name, ok := tool["name"].(string)
		s.assert.True(ok, "tool should have a name")
		s.assert.NotEmpty(name, "tool name should not be empty")

		description, ok := tool["description"].(string)
		s.assert.True(ok, "tool %s should have a description", name)
		s.assert.NotEmpty(description, "tool %s description should not be empty", name)

		inputSchema, ok := tool["inputSchema"].(map[string]interface{})
		s.assert.True(ok, "tool %s should have an input schema", name)
		s.assert.NotNil(inputSchema, "tool %s input schema should not be nil", name)
	}

	return s
}

func (s *MCPStdioStage) cluster_health_tool_is_called() *MCPStdioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"start_time": time.Now().Add(-1 * time.Hour).Unix(),
		"end_time":   time.Now().Unix(),
	}

	result, err := s.subprocess.CallTool(ctx, "cluster_health", args)
	s.require.NoError(err, "cluster_health tool call failed")
	s.require.NotNil(result, "tool result should not be nil")

	s.toolCallResult = result
	return s
}

func (s *MCPStdioStage) tool_result_contains_content() *MCPStdioStage {
	s.require.NotNil(s.toolCallResult, "tool must be called first")

	content, ok := s.toolCallResult["content"]
	s.require.True(ok, "result should contain 'content' field")
	s.assert.NotNil(content, "content should not be nil")

	return s
}

func (s *MCPStdioStage) tool_result_is_not_error() *MCPStdioStage {
	s.require.NotNil(s.toolCallResult, "tool must be called first")

	isError, ok := s.toolCallResult["isError"].(bool)
	if ok {
		s.assert.False(isError, "tool result should not be an error")
	}

	return s
}

func (s *MCPStdioStage) prompts_are_listed() *MCPStdioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Second)
	defer cancel()

	prompts, err := s.subprocess.ListPrompts(ctx)
	s.require.NoError(err, "list prompts failed")
	s.require.NotNil(prompts, "prompts should not be nil")

	s.prompts = prompts
	return s
}

func (s *MCPStdioStage) two_prompts_are_available() *MCPStdioStage {
	s.require.NotNil(s.prompts, "prompts must be listed first")
	s.assert.Len(s.prompts, 2, "should have exactly 2 prompts")
	return s
}

func (s *MCPStdioStage) expected_prompts_are_present() *MCPStdioStage {
	s.require.NotNil(s.prompts, "prompts must be listed first")

	expectedPrompts := map[string]bool{
		"post_mortem_incident_analysis": false,
		"live_incident_handling":        false,
	}

	// Convert prompts to JSON and back to get proper types
	promptsJSON, err := json.Marshal(s.prompts)
	s.require.NoError(err, "failed to marshal prompts")

	var promptsList []map[string]interface{}
	err = json.Unmarshal(promptsJSON, &promptsList)
	s.require.NoError(err, "failed to unmarshal prompts")

	for _, prompt := range promptsList {
		name, ok := prompt["name"].(string)
		if ok {
			if _, expected := expectedPrompts[name]; expected {
				expectedPrompts[name] = true
			}
		}
	}

	for promptName, found := range expectedPrompts {
		s.assert.True(found, "expected prompt %s to be present", promptName)
	}

	return s
}

func (s *MCPStdioStage) post_mortem_prompt_is_retrieved() *MCPStdioStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 10*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"start_time": time.Now().Add(-2 * time.Hour).Unix(),
		"end_time":   time.Now().Add(-1 * time.Hour).Unix(),
	}

	result, err := s.subprocess.GetPrompt(ctx, "post_mortem_incident_analysis", args)
	s.require.NoError(err, "get prompt failed")
	s.require.NotNil(result, "prompt result should not be nil")

	s.promptResult = result
	return s
}

func (s *MCPStdioStage) prompt_result_contains_messages() *MCPStdioStage {
	s.require.NotNil(s.promptResult, "prompt must be retrieved first")

	messages, ok := s.promptResult["messages"]
	s.require.True(ok, "result should contain 'messages' field")
	s.assert.NotNil(messages, "messages should not be nil")

	return s
}

func (s *MCPStdioStage) stdio_transport_test_complete() *MCPStdioStage {
	s.t.Log("✓ MCP stdio transport test completed successfully!")
	return s
}
