package llm

// NewDeepSeekClient creates a new DeepSeek client.
func NewDeepSeekClient(apiKey string) *OpenAICompatibleClient {
	return NewOpenAICompatibleClient("https://api.deepseek.com/v1", apiKey)
}
