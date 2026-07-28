package tools

import (
	"encoding/json"
	"time"
)

// GetCurrentTime returns the current local time as a human-readable string.
func GetCurrentTime() string {
	currentTime := time.Now()
	return currentTime.String()
}

// GetCurrentTimeSchema returns the Ollama function-calling schema for the get_current_time tool.
func GetCurrentTimeSchema() Tools {
	parameters := json.RawMessage(`{
    	"type": "object", 
    	"required": []
	}`)

	return Tools{
		Type: "function",
		Function: ToolFunction{
			Name:        "get_current_time",
			Parameters:  parameters,
			Description: "Gets the current time",
		},
	}
}
