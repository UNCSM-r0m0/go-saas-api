package llm

// NewKimiClient creates a new Kimi API client.
// Kimi uses an Anthropic-compatible API at https://api.kimi.com/coding/v1/messages
func NewKimiClient(apiKey string) *KimiAnthropicClient {
	return NewKimiAnthropicClient(apiKey)
}
