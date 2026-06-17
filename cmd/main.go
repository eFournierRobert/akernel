package main

import (
	"akernel/internal/handlers/chat"
	"akernel/internal/handlers/conversation"
	"akernel/internal/handlers/model"
	"akernel/internal/repository"
	"akernel/internal/service/ollama"
	"flag"
	"log"
	"net/http"
)

func main() {
	isDev := flag.Bool("dev", false, "If the user is in dev mode")
	flag.Parse()

	log.Println("Starting akernel...")
	ollamaService, err := ollama.New()
	if err != nil {
		log.Fatalln(err.Error())
	}

	db, err := repository.ConnectDB(*isDev)
	if err != nil {
		log.Fatalln(err.Error())
	}

	chatHandler := chat.New(*ollamaService, db)
	modelHandler := model.New(*ollamaService)
	conversationHandler := conversation.New(db, *ollamaService)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat", chatHandler.Chat)
	mux.HandleFunc("GET /models", modelHandler.GetModels)
	mux.HandleFunc("GET /conversation", conversationHandler.GetConversations)
	mux.HandleFunc("GET /conversation/{id}", chatHandler.GetFullConversation)
	mux.HandleFunc("DELETE /conversation/{id}", conversationHandler.DeleteConversation)
	mux.HandleFunc("POST /conversation/{id}/summarize", conversationHandler.PostSummarize)

	log.Println("Listening on localhost:50500")
	err = http.ListenAndServe("localhost:50500", mux)
	if err != nil {
		log.Fatalln(err)
	}
}
