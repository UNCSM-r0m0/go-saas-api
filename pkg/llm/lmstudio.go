package llm

// NewLMStudioClient creates a new LM Studio client.
// LM Studio exposes an OpenAI-compatible API.
func NewLMStudioClient(baseURL, apiKey string) *OpenAICompatibleClient {
	return NewOpenAICompatibleClient(baseURL+"/v1", apiKey)
}
