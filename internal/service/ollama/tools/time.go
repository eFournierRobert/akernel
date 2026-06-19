package tools

import (
	"encoding/json"
	"time"
)

// Tools is used for defining tools available to the AI model.
type Tools struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Parameters  json.RawMessage `json:"parameters"`
	Description string          `json:"description"`
}

func GetAllToolSchemas() []Tools {
	return []Tools{GetCurrentTimeSchema()}
}

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
			Description: "Current time in America/Montreal timezone",
		},
	}
}
