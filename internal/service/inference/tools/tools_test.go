package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllToolSchemas_Valid(t *testing.T) {
	schemas := GetAllToolSchemas()
	require.NotEmpty(t, schemas)

	for _, schema := range schemas {
		t.Run(schema.Function.Name, func(t *testing.T) {
			data, err := json.Marshal(schema)
			require.NoError(t, err)

			var decoded Tools
			require.NoError(t, json.Unmarshal(data, &decoded))

			assert.Equal(t, schema.Type, decoded.Type)
			assert.Equal(t, schema.Function.Name, decoded.Function.Name)
			assert.Equal(t, schema.Function.Description, decoded.Function.Description)
		})
	}
}

func TestToolCallRouter_GetCurrentTime(t *testing.T) {
	before := time.Now()

	result := ToolCallRouter(ToolCall{
		Function: Function{Name: "get_current_time"},
	})

	require.NotEmpty(t, result)
	assert.Contains(t, result, before.Format("2006-01-02"))
}

func TestToolCallRouter_UnknownTool(t *testing.T) {
	result := ToolCallRouter(ToolCall{
		Function: Function{Name: "does_not_exist"},
	})

	assert.Empty(t, result)
}

func TestToolCallRouter_WriteToFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldDataHome := xdg.DataHome
	xdg.DataHome = tmpDir
	t.Cleanup(func() { xdg.DataHome = oldDataHome })

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "akernel"), 0755))

	args, err := json.Marshal(WriteToFileArgs{
		FileName: "test.txt",
		Content:  "hello from test",
	})
	require.NoError(t, err)

	result := ToolCallRouter(ToolCall{
		Function: Function{Name: "write_to_file", Arguments: args},
	})

	assert.Equal(t, "wrote to file successfully", result)

	content, err := os.ReadFile(filepath.Join(tmpDir, "akernel", "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello from test", string(content))
}

func TestToolCallRouter_WriteToFile_InvalidArgs(t *testing.T) {
	result := ToolCallRouter(ToolCall{
		Function: Function{Name: "write_to_file", Arguments: json.RawMessage(`{invalid`)},
	})

	assert.Contains(t, result, "error while writing to file")
}
