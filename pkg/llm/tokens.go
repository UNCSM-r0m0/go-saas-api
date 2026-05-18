package llm

// EstimateTokens approximates token count using the standard ~4 characters per token
// rule of thumb. This is accurate enough for business metrics and cost tracking.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

// EstimateRequestTokens sums estimated tokens for all messages in a request,
// including content and tool call names.
func EstimateRequestTokens(msgs []Message) int {
	var total int
	for _, m := range msgs {
		total += EstimateTokens(m.Content)
		for _, tc := range m.ToolCalls {
			total += EstimateTokens(tc.Name)
		}
	}
	return total
}
