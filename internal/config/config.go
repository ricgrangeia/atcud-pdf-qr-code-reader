// Package config loads application settings from environment variables.
package config

import "os"

// AppVersion, LastUpdated e Author são definidos aqui e expostos
// pelo endpoint GET /api/v1/version.
const (
	AppVersion  = "1.1.0"
	Author      = "Ricardo Grangeia"
	AuthorURL   = "https://ricardo.grangeia.pt"
)

// Config holds every setting the application needs at startup.
// All values come from environment variables so the image is config-agnostic.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port string

	// URLHostDomain is the public domain name of the service (e.g. example.com).
	URLHostDomain string

	// VLLMBaseURL is the base URL of the vLLM-compatible OpenAI endpoint.
	// Available for AI-assisted processing if needed in the future.
	VLLMBaseURL string

	// VLLMAPIKey is the bearer token for authenticating with the vLLM service.
	VLLMAPIKey string

	// VLLMModel is the model identifier to use when calling vLLM.
	VLLMModel string

	// ToolServerURL is the base URL of the AI tool server that resolves NIFs to names.
	// Example: http://192.168.1.94:8000
	ToolServerURL string

	// ToolServerAPIKey is the x-api-key used to authenticate with the AI tool server.
	ToolServerAPIKey string
}

// Load reads the configuration from environment variables.
// PORT defaults to 8080 when not set.
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	domain := os.Getenv("URL_HOST_DOMAIN")
	if domain == "" {
		domain = "localhost"
	}

	return &Config{
		Port:          port,
		URLHostDomain: domain,
		VLLMBaseURL:   os.Getenv("VLLM_BASE_URL"),
		VLLMAPIKey:    os.Getenv("VLLM_API_KEY"),
		VLLMModel:     os.Getenv("VLLM_MODEL"),
		ToolServerURL:  os.Getenv("TOOL_SERVER_URL"),
		ToolServerAPIKey:     os.Getenv("TOOL_SERVER_API_KEY"),
	}
}
