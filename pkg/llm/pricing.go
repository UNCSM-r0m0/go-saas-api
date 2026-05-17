package llm

import "strings"

// ModelPricing holds per-model pricing in USD per 1K tokens.
type ModelPricing struct {
	InputPricePer1K  float64
	OutputPricePer1K float64
}

// DefaultPricing maps known model names to their pricing.
// Local/self-hosted models have zero cost (infrastructure is owned).
var DefaultPricing = map[string]ModelPricing{
	"gpt-4o":              {InputPricePer1K: 0.00250, OutputPricePer1K: 0.01000},
	"gpt-4o-mini":         {InputPricePer1K: 0.00015, OutputPricePer1K: 0.00060},
	"gpt-4-turbo":         {InputPricePer1K: 0.01000, OutputPricePer1K: 0.03000},
	"claude-3-5-sonnet":   {InputPricePer1K: 0.00300, OutputPricePer1K: 0.01500},
	"claude-3-opus":       {InputPricePer1K: 0.01500, OutputPricePer1K: 0.07500},
	"kimi-for-coding":     {InputPricePer1K: 0.00200, OutputPricePer1K: 0.00800},
	"deepseek-chat":       {InputPricePer1K: 0.00014, OutputPricePer1K: 0.00028},
	"deepseek-reasoner":   {InputPricePer1K: 0.00055, OutputPricePer1K: 0.00219},
	"gemini-1.5-pro":      {InputPricePer1K: 0.00125, OutputPricePer1K: 0.00500},
	"gemini-1.5-flash":    {InputPricePer1K: 0.000075, OutputPricePer1K: 0.00030},
	// Local models: cost 0 (own infrastructure)
	"qwen2.5-coder:3b":    {InputPricePer1K: 0, OutputPricePer1K: 0},
	"qwen2.5-coder:14b":   {InputPricePer1K: 0, OutputPricePer1K: 0},
	"llama3.1":            {InputPricePer1K: 0, OutputPricePer1K: 0},
	"llama3.2":            {InputPricePer1K: 0, OutputPricePer1K: 0},
	"mixtral":             {InputPricePer1K: 0, OutputPricePer1K: 0},
}

// ResolvePricing returns pricing for a model name. Falls back to a conservative
// default if the model is not in the map.
func ResolvePricing(modelName string) ModelPricing {
	if p, ok := DefaultPricing[modelName]; ok {
		return p
	}
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	if p, ok := DefaultPricing[normalized]; ok {
		return p
	}
	// Conservative fallback for unknown API models
	return ModelPricing{InputPricePer1K: 0.002, OutputPricePer1K: 0.008}
}

// CalculateCost computes the total cost in USD for the given token counts.
func (p ModelPricing) CalculateCost(inputTokens, outputTokens int) float64 {
	return float64(inputTokens)*p.InputPricePer1K/1000.0 +
		float64(outputTokens)*p.OutputPricePer1K/1000.0
}
