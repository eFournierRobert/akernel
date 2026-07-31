package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// braveBaseUrl is the endpoint for the Brave Search web search API.
const braveBaseUrl string = "https://api.search.brave.com/res/v1/web/search"

// braveResponse represents the JSON structure returned by the Brave Search API.
type braveResponse struct {
	Discussions struct {
		Type    string `json:"type"`
		Results []struct {
			Title       string `json:"title"`
			Url         string `json:"url"`
			Description string `json:"description"`
			PageAge     string `json:"page_age"`
		} `json:"results"`
	} `json:"discussions"`
	Faq struct {
		Type    string `json:"type"`
		Results []struct {
			Question string `json:"question"`
			Answer   string `json:"answer"`
			Url      string `json:"url"`
		} `json:"results"`
	} `json:"faq"`
	News struct {
		Type    string `json:"type"`
		Results []struct {
			Title       string `json:"title"`
			Url         string `json:"url"`
			Description string `json:"description"`
			PageAge     string `json:"page_age"`
		} `json:"results"`
	} `json:"news"`
	Web struct {
		Type    string `json:"type"`
		Results []struct {
			Title       string `json:"title"`
			Url         string `json:"url"`
			Description string `json:"description"`
			PageAge     string `json:"page_age"`
		} `json:"results"`
	} `json:"web"`
}

// webSearchArg holds the decoded arguments for a web_search tool call with a query string.
type webSearchArg struct {
	Query string `json:"query"`
}

// webSearch performs a search using the Brave Search API and returns formatted markdown results.
func webSearch(args []byte) string {
	var arguments webSearchArg
	err := json.Unmarshal(args, &arguments)
	if err != nil {
		log.Println("ERROR web_search: " + err.Error())
		return err.Error()
	}

	log.Println("Searching the web: " + arguments.Query)

	formattedQuery := url.QueryEscape(arguments.Query)

	fullQuery := braveBaseUrl + "?q=" + formattedQuery + "&result_filter=discussions,faq,news,web&count=5"
	req, err := http.NewRequest(http.MethodGet, fullQuery, nil)
	if err != nil {
		log.Println("ERROR web_search: " + err.Error())
		return fmt.Sprintf("could not create new requests: %s", err.Error())
	}

	req.Header.Set("X-Subscription-Token", Config.BraveApiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("ERROR web_search: " + err.Error())
		return fmt.Sprintf("error while searching: %s", err.Error())
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("ERROR web_search: " + err.Error())
		return fmt.Sprintf("error while reading body: %s", err.Error())
	}

	var braveResponse braveResponse
	if err = json.Unmarshal(content, &braveResponse); err != nil {
		log.Println("ERROR web_search: " + err.Error())
		return fmt.Sprintf("error while json unmarshal: %s", err.Error())
	}

	return formatResponseToMarkdown(braveResponse)
}

// webSearchSchema returns the function-calling schema for the web_search tool.
func webSearchSchema() Tools {
	parameters := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Search query for a search engine."
			}
		},
		"required": [
			"query"
		]
	}`)

	return Tools{
		Type: "function",
		Function: ToolFunction{
			Name:        "web_search",
			Parameters:  parameters,
			Description: "Search something on the web.",
		},
	}
}

// formatResponseToMarkdown converts a braveResponse into markdown organized by result type.
func formatResponseToMarkdown(braveResponse braveResponse) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Web search results\n"))
	sb.WriteString("## Discussions\n")
	for _, r := range braveResponse.Discussions.Results {
		sb.WriteString(fmt.Sprintf("### %s\n", r.Title))
		sb.WriteString(fmt.Sprintf("*Page age: %s*\n", r.PageAge))
		sb.WriteString(fmt.Sprintf("%s\n", r.Description))
		sb.WriteString(fmt.Sprintf("url: %s\n", r.Url))
	}

	sb.WriteString("## News\n")
	for _, r := range braveResponse.News.Results {
		sb.WriteString(fmt.Sprintf("### %s\n", r.Title))
		sb.WriteString(fmt.Sprintf("*Page age: %s*\n", r.PageAge))
		sb.WriteString(fmt.Sprintf("%s\n", r.Description))
		sb.WriteString(fmt.Sprintf("url: %s\n", r.Url))
	}

	sb.WriteString("## Web\n")
	for _, r := range braveResponse.Web.Results {
		sb.WriteString(fmt.Sprintf("### %s\n", r.Title))
		sb.WriteString(fmt.Sprintf("*Page age: %s*\n", r.PageAge))
		sb.WriteString(fmt.Sprintf("%s\n", r.Description))
		sb.WriteString(fmt.Sprintf("url: %s\n", r.Url))
	}

	sb.WriteString("## FAQ\n")
	for _, r := range braveResponse.Faq.Results {
		sb.WriteString(fmt.Sprintf("### %s\n", r.Question))
		sb.WriteString(fmt.Sprintf("%s\n", r.Answer))
		sb.WriteString(fmt.Sprintf("url: %s\n", r.Url))
	}

	return sb.String()
}
