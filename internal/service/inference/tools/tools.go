package tools

import "akernel/internal/config"

var Config config.Config

// GetAllToolSchemas returns a slice of all registered tool schemas for Ollama's function calling.
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
