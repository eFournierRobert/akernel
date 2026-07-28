package tools

// GetAllToolSchemas returns a slice of all registered tool schemas for Ollama's function calling.
func GetAllToolSchemas() []Tools {
	return []Tools{getCurrentTimeSchema(), fetchWebPageSchema()}
}

// ToolCallRouter dispatches an incoming tool call to the corresponding handler function by name.
func ToolCallRouter(toolCall ToolCall) string {
	switch toolCall.Function.Name {
	case "get_current_time":
		return getCurrentTime()
	case "fetch_webpage":
		return fetchWebPage(toolCall.Function.Arguments)
	default:
		return ""
	}
}
