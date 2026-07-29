package tools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllToolSchemas_Valid(t *testing.T) {
	testCases := []struct {
		name          string
		braveApiKey   string
		expectedCount int
	}{
		{
			name:          "without_brave_api_key",
			braveApiKey:   "",
			expectedCount: 2,
		},
		{
			name:          "with_brave_api_key",
			braveApiKey:   "test-api-key",
			expectedCount: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldConfig := Config
			defer func() { Config = oldConfig }()

			Config.BraveApiKey = tc.braveApiKey

			schemas := GetAllToolSchemas()
			require.Len(t, schemas, tc.expectedCount)

			for _, schema := range schemas {
				data, err := json.Marshal(schema)
				require.NoError(t, err)

				var decoded Tools
				require.NoError(t, json.Unmarshal(data, &decoded))

				assert.Equal(t, schema.Type, decoded.Type)
				assert.Equal(t, schema.Function.Name, decoded.Function.Name)
				assert.Equal(t, schema.Function.Description, decoded.Function.Description)
			}
		})
	}
}