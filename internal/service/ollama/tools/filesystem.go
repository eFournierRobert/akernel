package tools

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/adrg/xdg"
)

type WriteToFileArgs struct {
	Content  string `json:"content"`
	FileName string `json:"filename"`
}

func WriteToFile(args []byte) string {
	var arguments WriteToFileArgs
	err := json.Unmarshal(args, &arguments)
	if err != nil {
		return fmt.Errorf("error while writing to file: %w", err).Error()
	}

	filePath := fmt.Sprintf("%s/akernel/%s", xdg.DataHome, arguments.FileName)

	if err := os.WriteFile(filePath, []byte(arguments.Content), 0644); err != nil {
		return fmt.Errorf("error while writing to file: %w", err).Error()
	}
	return "wrote to file successfully"
}

func GetWriteToFileSchema() Tools {
	properties := []byte("\"properties\": {\"filename\": {\"type\": \"string\", \"description\": \"The name of the file to write to\"}, \"content\": {\"type\": \"string\", \"description\": \"The content to write\"}}")
	parameters := []byte("{\"type\": \"object\", ")
	parameters = append(parameters, properties...)
	parameters = append(parameters, []byte(",\"required\": [\"filename\", \"content\"]}")...)
	return Tools{
		Type: "function",
		Function: ToolFunction{
			Name:        "write_to_file",
			Parameters:  parameters,
			Description: "Write something to a file with the given name",
		},
	}
}
