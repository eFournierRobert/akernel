package tools

func GetAllToolSchemas() []Tools {
	return []Tools{GetCurrentTimeSchema(), GetWriteToFileSchema()}
}

func ToolCallRouter(toolCall ToolCall) string {
	switch toolCall.Function.Name {
	case "get_current_time":
		return GetCurrentTime()
	case "write_to_file":
		return WriteToFile(toolCall.Function.Arguments)
	default:
		return ""
	}
}
