package e2e

import (
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
)

const (
	clusterHealthTimeout = 30 * time.Second
)

type MCPHTTPStage struct {
	*helpers.BaseContext

	t *testing.T

	mcpClient *helpers.MCPClient

	initResult     map[string]interface{}
	tools          []helpers.ToolDefinition
	toolCallResult map[string]interface{}
	prompts        []helpers.PromptDefinition
	promptResult   map[string]interface{}

	// Helper managers
	ctxHelper *helpers.ContextHelper
}

func NewMCPHTTPStage(t *testing.T) (*MCPHTTPStage, *MCPHTTPStage, *MCPHTTPStage) {
	s := &MCPHTTPStage{
		t: t,
	}
	return s, s, s
}

func (s *MCPHTTPStage) and() *MCPHTTPStage {
	return s
}

func (s *MCPHTTPStage) a_test_environment() *MCPHTTPStage {
	// Use shared MCP-enabled deployment instead of creating a new one per test
	testCtx := helpers.SetupE2ETestSharedMCP(s.t)
	s.BaseContext = helpers.NewBaseContext(s.t, testCtx)

	// Initialize helper managers
	s.ctxHelper = helpers.NewContextHelper(s.t)

	return s
}

func (s *MCPHTTPStage) mcp_server_is_deployed() *MCPHTTPStage {
	// MCP server is already deployed and enabled on the shared deployment
	// No need to update Helm release or wait for deployment
	s.T.Logf("Using shared MCP deployment in namespace: %s", s.TestCtx.SharedDeployment.Namespace)
	return s
}

func (s *MCPHTTPStage) mcp_client_is_connected() *MCPHTTPStage {
	// Create port-forward to the shared MCP server
	serviceName := s.TestCtx.ReleaseName + "-spectre"
	// Important: Use SharedDeployment.Namespace, not TestCtx.Namespace
	mcpNamespace := s.TestCtx.SharedDeployment.Namespace
	mcpPortForward, err := helpers.NewPortForwarder(s.T, s.TestCtx.Cluster.GetContext(), mcpNamespace, serviceName, 8082)
	s.Require.NoError(err, "failed to create MCP port-forward")

	err = mcpPortForward.WaitForReady(30 * time.Second)
	s.Require.NoError(err, "MCP server not reachable via port-forward")

	s.T.Cleanup(func() {
		if err := mcpPortForward.Stop(); err != nil {
			s.T.Logf("Warning: failed to stop MCP port-forward: %v", err)
		}
	})

	s.mcpClient = helpers.NewMCPClient(s.T, mcpPortForward.GetURL())
	return s
}

func (s *MCPHTTPStage) mcp_server_is_healthy() *MCPHTTPStage {
	ctx, cancel := s.ctxHelper.WithTimeout(10 * time.Second)
	defer cancel()

	err := s.mcpClient.Health(ctx)
	s.Require.NoError(err, "MCP server health check failed")
	return s
}

func (s *MCPHTTPStage) ping_succeeds() *MCPHTTPStage {
	ctx, cancel := s.ctxHelper.WithTimeout(10 * time.Second)
	defer cancel()

	err := s.mcpClient.Ping(ctx)
	s.Require.NoError(err, "ping failed")
	return s
}

func (s *MCPHTTPStage) session_is_initialized() *MCPHTTPStage {
	ctx, cancel := s.ctxHelper.WithTimeout(10 * time.Second)
	defer cancel()

	result, err := s.mcpClient.Initialize(ctx)
	s.Require.NoError(err, "initialize failed")
	s.Require.NotNil(result, "initialize result should not be nil")

	s.initResult = result
	return s
}

func (s *MCPHTTPStage) server_info_is_correct() *MCPHTTPStage {
	s.Require.NotNil(s.initResult, "initialize must be called first")

	serverInfo, ok := s.initResult["serverInfo"].(map[string]interface{})
	s.Require.True(ok, "serverInfo should be present in initialize result")

	name, ok := serverInfo["name"].(string)
	s.Require.True(ok, "serverInfo.name should be a string")
	s.Assert.Equal("Spectre MCP Server", name)

	version, ok := serverInfo["version"].(string)
	s.Require.True(ok, "serverInfo.version should be a string")
	s.Assert.NotEmpty(version)

	return s
}

func (s *MCPHTTPStage) capabilities_include_tools_and_prompts() *MCPHTTPStage {
	s.Require.NotNil(s.initResult, "initialize must be called first")

	capabilities, ok := s.initResult["capabilities"].(map[string]interface{})
	s.Require.True(ok, "capabilities should be present in initialize result")

	_, hasTools := capabilities["tools"]
	s.Assert.True(hasTools, "capabilities should include tools")

	_, hasPrompts := capabilities["prompts"]
	s.Assert.True(hasPrompts, "capabilities should include prompts")

	return s
}

func (s *MCPHTTPStage) tools_are_listed() *MCPHTTPStage {
	ctx, cancel := s.ctxHelper.WithTimeout(10 * time.Second)
	defer cancel()

	tools, err := s.mcpClient.ListTools(ctx)
	s.Require.NoError(err, "list tools failed")
	s.Require.NotNil(tools, "tools should not be nil")

	s.tools = tools
	return s
}

func (s *MCPHTTPStage) four_tools_are_available() *MCPHTTPStage {
	s.Require.NotNil(s.tools, "tools must be listed first")
	// Should have 5 tools (base tools including causal_paths)
	toolCount := len(s.tools)
	s.Assert.Equal(5, toolCount, "should have 5 tools, got %d", toolCount)
	s.T.Logf("Available tools count: %d", toolCount)
	return s
}

func (s *MCPHTTPStage) expected_tools_are_present() *MCPHTTPStage {
	s.Require.NotNil(s.tools, "tools must be listed first")

	// Base tools that should always be present (including causal_paths which now uses HTTP API)
	baseTools := map[string]bool{
		"cluster_health":            false,
		"resource_timeline_changes": false,
		"resource_timeline":         false,
		"detect_anomalies":          false,
		"causal_paths":              false,
	}

	for _, tool := range s.tools {
		if _, expected := baseTools[tool.Name]; expected {
			baseTools[tool.Name] = true
		}
	}

	// Assert all base tools are present
	for toolName, found := range baseTools {
		s.Assert.True(found, "expected base tool %s to be present", toolName)
	}

	return s
}

func (s *MCPHTTPStage) each_tool_has_description_and_schema() *MCPHTTPStage {
	s.Require.NotNil(s.tools, "tools must be listed first")

	for _, tool := range s.tools {
		s.Assert.NotEmpty(tool.Name, "tool should have a name")
		s.Assert.NotEmpty(tool.Description, "tool %s should have a description", tool.Name)
		s.Assert.NotNil(tool.InputSchema, "tool %s should have an input schema", tool.Name)
	}

	return s
}

func (s *MCPHTTPStage) cluster_health_tool_is_called() *MCPHTTPStage {
	ctx, cancel := s.ctxHelper.WithTimeout(clusterHealthTimeout)
	defer cancel()

	args := map[string]interface{}{
		"start_time": time.Now().Add(-1 * time.Hour).Unix(),
		"end_time":   time.Now().Unix(),
	}

	result, err := s.mcpClient.CallTool(ctx, "cluster_health", args)
	s.Require.NoError(err, "cluster_health tool call failed")
	s.Require.NotNil(result, "tool result should not be nil")

	s.toolCallResult = result
	return s
}

func (s *MCPHTTPStage) tool_result_contains_content() *MCPHTTPStage {
	s.Require.NotNil(s.toolCallResult, "tool must be called first")

	content, ok := s.toolCallResult["content"]
	s.Require.True(ok, "result should contain 'content' field")
	s.Assert.NotNil(content, "content should not be nil")

	return s
}

func (s *MCPHTTPStage) tool_result_is_not_error() *MCPHTTPStage {
	s.Require.NotNil(s.toolCallResult, "tool must be called first")

	isError, ok := s.toolCallResult["isError"].(bool)
	if ok {
		s.Assert.False(isError, "tool result should not be an error")
	}

	return s
}

func (s *MCPHTTPStage) prompts_are_listed() *MCPHTTPStage {
	ctx, cancel := s.ctxHelper.WithTimeout(10 * time.Second)
	defer cancel()

	prompts, err := s.mcpClient.ListPrompts(ctx)
	s.Require.NoError(err, "list prompts failed")
	s.Require.NotNil(prompts, "prompts should not be nil")

	s.prompts = prompts
	return s
}

func (s *MCPHTTPStage) two_prompts_are_available() *MCPHTTPStage {
	s.Require.NotNil(s.prompts, "prompts must be listed first")
	s.Assert.Len(s.prompts, 2, "should have exactly 2 prompts")
	return s
}

func (s *MCPHTTPStage) expected_prompts_are_present() *MCPHTTPStage {
	s.Require.NotNil(s.prompts, "prompts must be listed first")

	expectedPrompts := map[string]bool{
		"post_mortem_incident_analysis": false,
		"live_incident_handling":        false,
	}

	for _, prompt := range s.prompts {
		if _, ok := expectedPrompts[prompt.Name]; ok {
			expectedPrompts[prompt.Name] = true
		}
	}

	for promptName, found := range expectedPrompts {
		s.Assert.True(found, "expected prompt %s to be present", promptName)
	}

	return s
}

func (s *MCPHTTPStage) prompt_has_required_arguments() *MCPHTTPStage {
	s.Require.NotNil(s.prompts, "prompts must be listed first")

	for _, prompt := range s.prompts {
		s.Assert.NotEmpty(prompt.Name, "prompt should have a name")
		s.Assert.NotEmpty(prompt.Description, "prompt %s should have a description", prompt.Name)

		// Check that each prompt has at least some arguments
		hasRequiredArgs := false
		for _, arg := range prompt.Arguments {
			if arg.Required {
				hasRequiredArgs = true
				break
			}
		}
		s.Assert.True(hasRequiredArgs, "prompt %s should have at least one required argument", prompt.Name)
	}

	return s
}

func (s *MCPHTTPStage) post_mortem_prompt_is_retrieved() *MCPHTTPStage {
	ctx, cancel := s.ctxHelper.WithTimeout(10 * time.Second)
	defer cancel()

	args := map[string]interface{}{
		"start_time": time.Now().Add(-2 * time.Hour).Unix(),
		"end_time":   time.Now().Add(-1 * time.Hour).Unix(),
	}

	result, err := s.mcpClient.GetPrompt(ctx, "post_mortem_incident_analysis", args)
	s.Require.NoError(err, "get prompt failed")
	s.Require.NotNil(result, "prompt result should not be nil")

	s.promptResult = result
	return s
}

func (s *MCPHTTPStage) prompt_result_contains_messages() *MCPHTTPStage {
	s.Require.NotNil(s.promptResult, "prompt must be retrieved first")

	messages, ok := s.promptResult["messages"]
	s.Require.True(ok, "result should contain 'messages' field")
	s.Assert.NotNil(messages, "messages should not be nil")

	return s
}

func (s *MCPHTTPStage) logging_level_can_be_set() *MCPHTTPStage {
	ctx, cancel := s.ctxHelper.WithTimeout(10 * time.Second)
	defer cancel()

	err := s.mcpClient.SetLoggingLevel(ctx, "debug")
	s.Assert.NoError(err, "set logging level failed")

	return s
}

func (s *MCPHTTPStage) http_transport_test_complete() *MCPHTTPStage {
	s.T.Log("✓ MCP HTTP transport test completed successfully!")
	return s
}
