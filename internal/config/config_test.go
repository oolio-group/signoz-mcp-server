package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_CustomHeaders(t *testing.T) {
	tests := []struct {
		name            string
		envValue        string
		expectedHeaders map[string]string
	}{
		{
			name:            "empty env var produces empty map",
			envValue:        "",
			expectedHeaders: map[string]string{},
		},
		{
			name:     "single header pair",
			envValue: "X-Custom-Auth:my-token",
			expectedHeaders: map[string]string{
				"X-Custom-Auth": "my-token",
			},
		},
		{
			name:     "multiple header pairs",
			envValue: "CF-Access-Client-Id:abc123.access,CF-Access-Client-Secret:secret456",
			expectedHeaders: map[string]string{
				"CF-Access-Client-Id":     "abc123.access",
				"CF-Access-Client-Secret": "secret456",
			},
		},
		{
			name:     "whitespace is trimmed",
			envValue: " Key1 : Value1 , Key2 : Value2 ",
			expectedHeaders: map[string]string{
				"Key1": "Value1",
				"Key2": "Value2",
			},
		},
		{
			name:     "value containing colon is preserved",
			envValue: "Authorization:Bearer my-jwt-token:with:colons",
			expectedHeaders: map[string]string{
				"Authorization": "Bearer my-jwt-token:with:colons",
			},
		},
		{
			name:     "malformed entry without colon is skipped",
			envValue: "ValidKey:ValidValue,MalformedEntry",
			expectedHeaders: map[string]string{
				"ValidKey": "ValidValue",
			},
		},
		{
			name:     "empty header name is skipped",
			envValue: ":some-value,ValidKey:ValidValue",
			expectedHeaders: map[string]string{
				"ValidKey": "ValidValue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SIGNOZ_URL", "http://localhost:8080")
			t.Setenv("SIGNOZ_API_KEY", "test-key")

			if tt.envValue != "" {
				t.Setenv("SIGNOZ_CUSTOM_HEADERS", tt.envValue)
			}

			cfg, err := LoadConfig()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedHeaders, cfg.CustomHeaders)
		})
	}
}

func TestValidateConfig_HTTPAllowsCredentialsFromHeaders(t *testing.T) {
	cfg := &Config{
		TransportMode: "http",
		Port:          "8000",
	}

	require.NoError(t, cfg.ValidateConfig())
}

func TestValidateConfig_StdioRequiresConfiguredCredentials(t *testing.T) {
	cfg := &Config{
		TransportMode: "stdio",
	}

	require.ErrorContains(t, cfg.ValidateConfig(), "SIGNOZ_API_KEY is required")
}

// TestLoadConfig_BasicAuth verifies that the BasicAuthHeader is precomputed
// correctly and that the raw credentials are never exposed on Config.
func TestLoadConfig_BasicAuth(t *testing.T) {
	t.Run("both vars set produces correct Basic header", func(t *testing.T) {
		t.Setenv("SIGNOZ_BASIC_AUTH_USERNAME", "alice")
		t.Setenv("SIGNOZ_BASIC_AUTH_PASSWORD", "s3cret")
		t.Setenv("SIGNOZ_URL", "http://localhost:8080")
		t.Setenv("SIGNOZ_API_KEY", "k")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		// base64("alice:s3cret") == "YWxpY2U6czNjcmV0"
		assert.Equal(t, "Basic YWxpY2U6czNjcmV0", cfg.BasicAuthHeader)
	})

	t.Run("neither var set leaves BasicAuthHeader empty", func(t *testing.T) {
		t.Setenv("SIGNOZ_URL", "http://localhost:8080")
		t.Setenv("SIGNOZ_API_KEY", "k")

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Empty(t, cfg.BasicAuthHeader)
	})
}

// TestValidateConfig_BasicAuth exercises the both-or-neither and colon rules.
func TestValidateConfig_BasicAuth(t *testing.T) {
	base := Config{TransportMode: "http", Port: "8000"}

	t.Run("both set is valid", func(t *testing.T) {
		cfg := base
		cfg.basicAuthUsername = "user"
		cfg.basicAuthPasswordSet = true
		require.NoError(t, cfg.ValidateConfig())
	})

	t.Run("username only errors", func(t *testing.T) {
		cfg := base
		cfg.basicAuthUsername = "user"
		cfg.basicAuthPasswordSet = false
		require.ErrorContains(t, cfg.ValidateConfig(), SignozBasicAuthPassword)
	})

	t.Run("password only errors", func(t *testing.T) {
		cfg := base
		cfg.basicAuthUsername = ""
		cfg.basicAuthPasswordSet = true
		require.ErrorContains(t, cfg.ValidateConfig(), SignozBasicAuthUsername)
	})

	t.Run("username with colon errors", func(t *testing.T) {
		cfg := base
		cfg.basicAuthUsername = "user:name"
		cfg.basicAuthPasswordSet = true
		require.ErrorContains(t, cfg.ValidateConfig(), "colon")
	})

	t.Run("neither set is valid", func(t *testing.T) {
		cfg := base
		require.NoError(t, cfg.ValidateConfig())
	})
}
