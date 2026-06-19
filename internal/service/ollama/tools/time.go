package tools

import (
	"time"
)

func GetCurrentTime() string {
	currentTime := time.Now()
	return currentTime.String()
}

func GetCurrentTimeSchema() Tools {
	parameters := []byte("{\"type\": \"object\", \"required\": []}")
	return Tools{
		Type: "function",
		Function: ToolFunction{
			Name:        "get_current_time",
			Parameters:  parameters,
			Description: "Gets the current time",
		},
	}
}
