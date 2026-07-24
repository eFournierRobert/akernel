package tools

import (
	"encoding/json"
	"testing"

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
