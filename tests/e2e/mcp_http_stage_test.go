package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MCPHTTPStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	mcpClient *helpers.MCPClient

	initResult     map[string]interface{}
	tools          []helpers.ToolDefinition
	toolCallResult map[string]interface{}
	prompts        []helpers.PromptDefinition
	promptResult   map[string]interface{}
}

func NewMCPHTTPStage(t *testing.T) (*MCPHTTPStage, *MCPHTTPStage, *MCPHTTPStage) {
	s := &MCPHTTPStage{
		t:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
	return s, s, s
}

func (s *MCPHTTPStage) and() *MCPHTTPStage {
	return s
}

func (s *MCPHTTPStage) a_test_environment() *MCPHTTPStage {
	s.testCtx = helpers.SetupE2ETest(s.t)
	return s
}

func (s *MCPHTTPStage) mcp_server_is_deployed() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Update Helm release to enable MCP server
	err := helpers.UpdateHelmRelease(s.testCtx, map[string]interface{}{
		"mcp": map[string]interface{}{
			"enabled": true,
			"httpAddr": ":8081",
		},
	})
	s.require.NoError(err, "failed to update Helm release with MCP enabled")

	// Wait for the deployment to be ready
	err = helpers.WaitForAppReady(ctx, s.testCtx.K8sClient, s.testCtx.Namespace, s.testCtx.ReleaseName)
	s.require.NoError(err, "failed to wait for app to be ready after MCP enable")

	return s
}

func (s *MCPHTTPStage) mcp_client_is_connected() *MCPHTTPStage {
	// Create port-forward for MCP server
	serviceName := s.testCtx.ReleaseName + "-spectre"
	mcpPortForward, err := helpers.NewPortForwarder(s.t, s.testCtx.Cluster.GetKubeConfig(), s.testCtx.Namespace, serviceName, 8081)
	s.require.NoError(err, "failed to create MCP port-forward")

	err = mcpPortForward.WaitForReady(30 * time.Second)
	s.require.NoError(err, "MCP server not reachable via port-forward")

	s.t.Cleanup(func() {
		if err := mcpPortForward.Stop(); err != nil {
			s.t.Logf("Warning: failed to stop MCP port-forward: %v", err)
		}
	})

	s.mcpClient = helpers.NewMCPClient(s.t, mcpPortForward.GetURL())
	return s
}

func (s *MCPHTTPStage) mcp_server_is_healthy() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.mcpClient.Health(ctx)
	s.require.NoError(err, "MCP server health check failed")
	return s
}

func (s *MCPHTTPStage) ping_succeeds() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.mcpClient.Ping(ctx)
	s.require.NoError(err, "ping failed")
	return s
}

func (s *MCPHTTPStage) session_is_initialized() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := s.mcpClient.Initialize(ctx)
	s.require.NoError(err, "initialize failed")
	s.require.NotNil(result, "initialize result should not be nil")

	s.initResult = result
	return s
}

func (s *MCPHTTPStage) server_info_is_correct() *MCPHTTPStage {
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

func (s *MCPHTTPStage) capabilities_include_tools_and_prompts() *MCPHTTPStage {
	s.require.NotNil(s.initResult, "initialize must be called first")

	capabilities, ok := s.initResult["capabilities"].(map[string]interface{})
	s.require.True(ok, "capabilities should be present in initialize result")

	_, hasTools := capabilities["tools"]
	s.assert.True(hasTools, "capabilities should include tools")

	_, hasPrompts := capabilities["prompts"]
	s.assert.True(hasPrompts, "capabilities should include prompts")

	return s
}

func (s *MCPHTTPStage) tools_are_listed() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tools, err := s.mcpClient.ListTools(ctx)
	s.require.NoError(err, "list tools failed")
	s.require.NotNil(tools, "tools should not be nil")

	s.tools = tools
	return s
}

func (s *MCPHTTPStage) four_tools_are_available() *MCPHTTPStage {
	s.require.NotNil(s.tools, "tools must be listed first")
	s.assert.Len(s.tools, 4, "should have exactly 4 tools")
	return s
}

func (s *MCPHTTPStage) expected_tools_are_present() *MCPHTTPStage {
	s.require.NotNil(s.tools, "tools must be listed first")

	expectedTools := map[string]bool{
		"cluster_health":    false,
		"resource_changes":  false,
		"investigate":       false,
		"resource_explorer": false,
	}

	for _, tool := range s.tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for toolName, found := range expectedTools {
		s.assert.True(found, "expected tool %s to be present", toolName)
	}

	return s
}

func (s *MCPHTTPStage) each_tool_has_description_and_schema() *MCPHTTPStage {
	s.require.NotNil(s.tools, "tools must be listed first")

	for _, tool := range s.tools {
		s.assert.NotEmpty(tool.Name, "tool should have a name")
		s.assert.NotEmpty(tool.Description, "tool %s should have a description", tool.Name)
		s.assert.NotNil(tool.InputSchema, "tool %s should have an input schema", tool.Name)
	}

	return s
}

func (s *MCPHTTPStage) cluster_health_tool_is_called() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"start_time": time.Now().Add(-1 * time.Hour).Unix(),
		"end_time":   time.Now().Unix(),
	}

	result, err := s.mcpClient.CallTool(ctx, "cluster_health", args)
	s.require.NoError(err, "cluster_health tool call failed")
	s.require.NotNil(result, "tool result should not be nil")

	s.toolCallResult = result
	return s
}

func (s *MCPHTTPStage) tool_result_contains_content() *MCPHTTPStage {
	s.require.NotNil(s.toolCallResult, "tool must be called first")

	content, ok := s.toolCallResult["content"]
	s.require.True(ok, "result should contain 'content' field")
	s.assert.NotNil(content, "content should not be nil")

	return s
}

func (s *MCPHTTPStage) tool_result_is_not_error() *MCPHTTPStage {
	s.require.NotNil(s.toolCallResult, "tool must be called first")

	isError, ok := s.toolCallResult["isError"].(bool)
	if ok {
		s.assert.False(isError, "tool result should not be an error")
	}

	return s
}

func (s *MCPHTTPStage) prompts_are_listed() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompts, err := s.mcpClient.ListPrompts(ctx)
	s.require.NoError(err, "list prompts failed")
	s.require.NotNil(prompts, "prompts should not be nil")

	s.prompts = prompts
	return s
}

func (s *MCPHTTPStage) two_prompts_are_available() *MCPHTTPStage {
	s.require.NotNil(s.prompts, "prompts must be listed first")
	s.assert.Len(s.prompts, 2, "should have exactly 2 prompts")
	return s
}

func (s *MCPHTTPStage) expected_prompts_are_present() *MCPHTTPStage {
	s.require.NotNil(s.prompts, "prompts must be listed first")

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
		s.assert.True(found, "expected prompt %s to be present", promptName)
	}

	return s
}

func (s *MCPHTTPStage) prompt_has_required_arguments() *MCPHTTPStage {
	s.require.NotNil(s.prompts, "prompts must be listed first")

	for _, prompt := range s.prompts {
		s.assert.NotEmpty(prompt.Name, "prompt should have a name")
		s.assert.NotEmpty(prompt.Description, "prompt %s should have a description", prompt.Name)

		// Check that each prompt has at least some arguments
		hasRequiredArgs := false
		for _, arg := range prompt.Arguments {
			if arg.Required {
				hasRequiredArgs = true
				break
			}
		}
		s.assert.True(hasRequiredArgs, "prompt %s should have at least one required argument", prompt.Name)
	}

	return s
}

func (s *MCPHTTPStage) post_mortem_prompt_is_retrieved() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := map[string]interface{}{
		"start_time": time.Now().Add(-2 * time.Hour).Unix(),
		"end_time":   time.Now().Add(-1 * time.Hour).Unix(),
	}

	result, err := s.mcpClient.GetPrompt(ctx, "post_mortem_incident_analysis", args)
	s.require.NoError(err, "get prompt failed")
	s.require.NotNil(result, "prompt result should not be nil")

	s.promptResult = result
	return s
}

func (s *MCPHTTPStage) prompt_result_contains_messages() *MCPHTTPStage {
	s.require.NotNil(s.promptResult, "prompt must be retrieved first")

	messages, ok := s.promptResult["messages"]
	s.require.True(ok, "result should contain 'messages' field")
	s.assert.NotNil(messages, "messages should not be nil")

	return s
}

func (s *MCPHTTPStage) logging_level_can_be_set() *MCPHTTPStage {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.mcpClient.SetLoggingLevel(ctx, "debug")
	s.assert.NoError(err, "set logging level failed")

	return s
}

func (s *MCPHTTPStage) http_transport_test_complete() *MCPHTTPStage {
	s.t.Log("✓ MCP HTTP transport test completed successfully!")
	return s
}
