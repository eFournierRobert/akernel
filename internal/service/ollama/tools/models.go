// Package tools define the tool schemas, types, and router that Ollama uses for function calling.
package tools

import "encoding/json"

// Tools is used for defining tools available to the AI model.
type Tools struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction specifies the name, description, and JSON-schema parameters of an Ollama function-calling tool.
type ToolFunction struct {
	Name        string          `json:"name"`
	Parameters  json.RawMessage `json:"parameters"`
	Description string          `json:"description"`
}

// ToolCall represents a single tool invocation request returned by the Ollama model.
type ToolCall struct {
	Function Function `json:"function"`
}

// Function holds the name and deserialized arguments of a tool call issued by the model.
type Function struct {
	Name        string          `json:"name"`
	Arguments   json.RawMessage `json:"arguments"`
	Description string          `json:"description"`
}
