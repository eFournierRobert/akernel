package ollama

import "github.com/google/jsonschema-go/jsonschema"

type Model struct {
	Name  string `json:"name"`
	Model string `json:"model"`
	Size  int    `json:"size"`
}

type Models struct {
	ModelList []Model `json:"models"`
}

type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tools struct{}

type Message struct {
	Model     string             `json:"model"`
	Messages  []HistoryMessage   `json:"messages"`
	Format    *jsonschema.Schema `json:"format"`
	Think     bool               `json:"think"`
	KeepAlive string             `json:"keep_alive"`
	Stream    bool               `json:"stream"`
	Options   Options            `json:"options"`
}

type Options struct {
	Temperature float32 `json:"temperature"`
}

type MessageResponse struct {
	Model    string         `json:"model"`
	Done     bool           `json:"done"`
	Response HistoryMessage `json:"message"`
}

type MessageFormat struct {
	MessageBody []MessageParagraph `json:"messageBody" jsonschema:"body containing a group of paragraphs"`
}

type MessageParagraph struct {
	Heading string `json:"Heading" jsonschema:"Heading of this paragraph. This is not required if unnecessary."`
	Body    string `json:"body" jsonschema:"paragraph body in markdown"`
}
