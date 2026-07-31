package tools

import "encoding/json"

// Tools is used for defining tools available to the AI model.
type Tools struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction specifies the name, description, and JSON-schema parameters of a function-calling tool.
type ToolFunction struct {
	Name        string          `json:"name"`
	Parameters  json.RawMessage `json:"parameters"`
	Description string          `json:"description"`
}

// ToolCall represents a single tool invocation request returned by an LLM provider.
// The Type field indicates the kind of call (e.g., "function"). The Id field is
// a unique identifier for the invocation, used to correlate results back to callers.
type ToolCall struct {
	Function Function `json:"function"`
	Type     string   `json:"type"`
	Id       string   `json:"id"`
}

// Function holds the name and deserialized arguments of a tool call issued by the model.
type Function struct {
	Name        string          `json:"name"`
	Arguments   json.RawMessage `json:"arguments"`
	Description string          `json:"description"`
}
