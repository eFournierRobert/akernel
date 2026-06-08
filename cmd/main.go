package main

import (
	"akernel/internal/database"
	"akernel/internal/handlers/chat"
	"akernel/internal/ollama"
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

	db, err := database.ConnectDB(ollamaService, *isDev)
	if err != nil {
		log.Fatalln(err.Error())
	}

	chatHandler := chat.New(ollamaService, db)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat", chatHandler.Chat)
	mux.HandleFunc("GET /conversation", chatHandler.GetConversations)
	mux.HandleFunc("GET /models", chatHandler.GetModels)

	log.Println("Listening on localhost:8080")
	err = http.ListenAndServe("localhost:8080", mux)
	if err != nil {
		log.Fatalln(err)
	}
}
