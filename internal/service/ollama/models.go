package ollama

// Model represents metadata for an AI model available in Ollama.
type Model struct {
	Name  string `json:"name"`
	Model string `json:"model"`
	Size  int    `json:"size"`
}

// Models contain a list of AI models retrieved from the Ollama API.
type Models struct {
	ModelList []Model `json:"models"`
}

// HistoryMessage represents a single message within a conversation history, including its role and content.
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Tools is used for defining tools available to the AI model.
type Tools struct{}

// Message defines the request payload for sending a chat prompt to an Ollama model.
type Message struct {
	Model     string           `json:"model"`
	Messages  []HistoryMessage `json:"messages"`
	Think     bool             `json:"think"`
	KeepAlive string           `json:"keep_alive"`
	Stream    bool             `json:"stream"`
	Options   Options          `json:"options"`
}

// Options provides configuration parameters for the AI model's generation process.
type Options struct {
	Temperature float32 `json:"temperature"`
}

// MessageResponse represents a single chunk of a streaming response from the Ollama API.
type MessageResponse struct {
	Model    string         `json:"model"`
	Done     bool           `json:"done"`
	Response HistoryMessage `json:"message"`
}
