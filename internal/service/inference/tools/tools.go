package tools

// GetAllToolSchemas returns a slice of all registered tool schemas for Ollama's function calling.
func GetAllToolSchemas() []Tools {
	return []Tools{GetCurrentTimeSchema()}
}

// ToolCallRouter dispatches an incoming tool call to the corresponding handler function by name.
func ToolCallRouter(toolCall ToolCall) string {
	switch toolCall.Function.Name {
	case "get_current_time":
		return GetCurrentTime()
	default:
		return ""
	}
}
