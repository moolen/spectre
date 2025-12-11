package e2e

import (
	"testing"
)

func TestMCPStdioTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewMCPStdioStage(t)

	given.a_test_environment().and().
		spectre_binary_is_built().and().
		mcp_subprocess_is_started()

	when.subprocess_is_ready().and().
		session_is_initialized()

	then.server_info_is_correct().and().
		capabilities_include_tools_and_prompts()

	when.tools_are_listed()

	then.four_tools_are_available().and().
		expected_tools_are_present().and().
		each_tool_has_description_and_schema()

	when.cluster_health_tool_is_called()

	then.tool_result_contains_content().and().
		tool_result_is_not_error()

	when.prompts_are_listed()

	then.two_prompts_are_available().and().
		expected_prompts_are_present()

	when.post_mortem_prompt_is_retrieved()

	then.prompt_result_contains_messages().and().
		stdio_transport_test_complete()
}
