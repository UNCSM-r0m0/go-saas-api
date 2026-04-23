package llm

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(apiKey string) *OpenAICompatibleClient {
	return NewOpenAICompatibleClient("https://api.openai.com/v1", apiKey)
}
