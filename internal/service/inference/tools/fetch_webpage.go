package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

type fetchWebPageArg struct {
	Url string `json:"url"`
}

func fetchWebPage(args []byte) string {
	var arguments fetchWebPageArg
	err := json.Unmarshal(args, &arguments)
	if err != nil {
		return err.Error()
	}

	log.Println("fetching web page: " + arguments.Url)

	resp, err := http.Get(arguments.Url)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("error. got: %d", resp.StatusCode)
	}

	content, err := htmltomarkdown.ConvertReader(resp.Body)
	if err != nil {
		return err.Error()
	}

	return string(content)
}

func fetchWebPageSchema() Tools {
	parameters := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "URL of the web page to fetch."
			}
		},
		"required": [
			"url"
		]
	}`)

	return Tools{
		Type: "function",
		Function: ToolFunction{
			Name:        "fetch_webpage",
			Parameters:  parameters,
			Description: "Fetches a web page",
		},
	}
}
