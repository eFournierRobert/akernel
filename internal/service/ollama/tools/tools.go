package tools

func GetAllToolSchemas() []Tools {
	return []Tools{GetCurrentTimeSchema()}
}

func ToolCallRouter(toolCall ToolCall) string {
	switch toolCall.Function.Name {
	case "get_current_time":
		return GetCurrentTime()
	default:
		return ""
	}
}
