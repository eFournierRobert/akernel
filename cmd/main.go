package main

import (
	"akernel/internal/config"
	"akernel/internal/handlers/chat"
	"akernel/internal/handlers/conversation"
	"akernel/internal/handlers/model"
	"akernel/internal/middleware"
	"akernel/internal/repository"
	"akernel/internal/service/inference"
	"flag"
	"log"
	"net/http"
)

func main() {
	isDev := flag.Bool("dev", false, "If the user is in dev mode")
	flag.Parse()
	log.Println("Reading configurations...")
	config, err := config.ReadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Starting akernel...")

	db, err := repository.ConnectDB(*isDev)
	if err != nil {
		log.Fatalln(err)
	}

	inference := inference.New(config)

	chatHandler := chat.New(db, inference)
	modelHandler := model.New(inference)
	conversationHandler := conversation.New(db, inference)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat", chatHandler.Chat)
	mux.HandleFunc("GET /models", modelHandler.GetModels)
	mux.HandleFunc("GET /conversation", conversationHandler.GetConversations)
	mux.HandleFunc("GET /conversation/{id}", chatHandler.GetFullConversation)
	mux.HandleFunc("DELETE /conversation/{id}", conversationHandler.DeleteConversation)
	mux.HandleFunc("POST /conversation/{id}/summarize", conversationHandler.PostSummarize)

	handler := middleware.CorsMiddleware(mux)

	log.Println("Listening on 0.0.0.0:50500")
	err = http.ListenAndServe("0.0.0.0:50500", handler)
	if err != nil {
		log.Fatalln(err)
	}
}
