package tools

import "encoding/json"

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

type ToolCall struct {
	Function Function `json:"function"`
}

type Function struct {
	Name        string          `json:"name"`
	Arguments   json.RawMessage `json:"arguments"`
	Description string          `json:"description"`
}
