package llm

// NewKimiClient creates a new Kimi API client.
// Kimi uses an OpenAI-compatible API at https://api.kimi.com/coding/v1
func NewKimiClient(apiKey string) *OpenAICompatibleClient {
	return NewOpenAICompatibleClient("https://api.kimi.com/coding/v1", apiKey)
}
