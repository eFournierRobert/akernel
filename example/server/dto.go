package server

type Model struct {
	Name  string  `json:"name"`
	Model string  `json:"model"`
	Size  float32 `json:"size"`
}

type Conversation struct {
	Id    int    `json:"id"`
	Title string `json:"title"`
}

type ChatBody struct {
	Model          string `json:"model"`
	Message        string `json:"message"`
	ConversationId int    `json:"conversationId"`
}

type ChatResponse struct {
	Role           string `json:"role"`
	ConversationId int    `json:"conversationId"`
	Message        string `json:"message"`
}
