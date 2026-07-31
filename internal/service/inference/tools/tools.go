// Package tools defines the tool schemas, types, and router used for LLM function calling.
package tools

import "akernel/internal/config"

// Config holds runtime configuration consumed by the tools package, such as API keys
// (e.g. BraveApiKey) that gate which tools are advertised to the model.
var Config config.Config

// GetAllToolSchemas returns a slice of all registered tool schemas for function calling.
// The available set is gated by Config: web_search is only advertised when a BraveApiKey is set.
func GetAllToolSchemas() []Tools {
	if len(Config.BraveApiKey) > 0 {
		return []Tools{getCurrentTimeSchema(), fetchWebPageSchema(), webSearchSchema()}
	}

	return []Tools{getCurrentTimeSchema(), fetchWebPageSchema()}
}

// ToolCallRouter dispatches an incoming tool call to the corresponding handler function by name.
func ToolCallRouter(toolCall ToolCall) string {
	switch toolCall.Function.Name {
	case "get_current_time":
		return getCurrentTime()
	case "fetch_webpage":
		return fetchWebPage(toolCall.Function.Arguments)
	case "web_search":
		return webSearch(toolCall.Function.Arguments)
	default:
		return ""
	}
}
