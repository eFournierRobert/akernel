package main

import (
	"akernel/example/server"
	"flag"
	"fmt"
)

var conversationId *int
var modelName *string

var listModels *bool
var listConversations *bool
var chat *string

func setUpFlags() {
	conversationId = flag.Int("convo", 0, "ID of the conversation")
	modelName = flag.String("model", "", "Name of the AI model to use")

	listModels = flag.Bool("list-models", false, "List AI models available locally")
	listConversations = flag.Bool("list-convo", false, "List all conversations")
	chat = flag.String("chat", "", "Chat with an AI model")
}

func main() {
	setUpFlags()
	flag.Parse()

	if *listModels {
		listAIModels()
		return
	}
	if *listConversations {
		listConvo()
		return
	}
	if *chat != "" {
		if *modelName == "" {
			fmt.Println("provide an AI model name with -model")
			return
		}

		sendChat()
	}
}

func sendChat() {
	fmt.Println("Waiting for AI answer...")
	responses, err := server.PostChat(*conversationId, *modelName, *chat)
	if err != nil {
		fmt.Println("Error while trying to send chat: ", err.Error())
		return
	}

	fmt.Printf("\n--Conversation: %d--\n", responses[0].ConversationId)
	for _, r := range responses {
		fmt.Printf("Role: %s\n\t%s\n", r.Role, r.Message)
	}
}

func listConvo() {
	convos, err := server.ListConversations()
	if err != nil {
		fmt.Println("Error while trying to list conversations: ", err.Error())
		return
	}

	for _, c := range convos {
		fmt.Printf("%d:\t%s\n", c.Id, c.Title)
	}
}

func listAIModels() {
	models, err := server.ListModels()
	if err != nil {
		fmt.Println("Error while trying to list models: ", err.Error())
		return
	}

	for _, m := range models {
		fmt.Printf("Name: %s\n\tModel: %s\n\tSize: %.2f GB\n", m.Name, m.Model, m.Size)
	}
}
