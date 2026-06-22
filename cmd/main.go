package main

import (
	"akernel/internal/handlers/chat"
	"akernel/internal/handlers/conversation"
	"akernel/internal/handlers/model"
	"akernel/internal/repository"
	"akernel/internal/service/ollama"
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/adrg/xdg"
)

func main() {
	isDev := flag.Bool("dev", false, "If the user is in dev mode")
	flag.Parse()
	log.Println("Reading system prompt...")
	systemPrompt, err := readSystemPrompt()
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Starting akernel...")
	ollamaService := ollama.New(systemPrompt)

	db, err := repository.ConnectDB(*isDev)
	if err != nil {
		log.Fatalln(err)
	}

	chatHandler := chat.New(ollamaService, db)
	modelHandler := model.New(ollamaService)
	conversationHandler := conversation.New(db, ollamaService)

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

func readSystemPrompt() (string, error) {
	systemPromptPath, err := xdg.ConfigFile("akernel/system.md")
	if err != nil {
		return "", err
	}

	f, err := os.Open(systemPromptPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	systemPrompt, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return string(systemPrompt), nil
}
