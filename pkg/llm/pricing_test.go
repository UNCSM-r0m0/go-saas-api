package llm

import "testing"

func TestResolvePricing(t *testing.T) {
	tests := []struct {
		model    string
		expected ModelPricing
	}{
		{"gpt-4o", ModelPricing{InputPricePer1K: 0.00250, OutputPricePer1K: 0.01000}},
		{"gpt-4o-mini", ModelPricing{InputPricePer1K: 0.00015, OutputPricePer1K: 0.00060}},
		{"qwen2.5-coder:3b", ModelPricing{InputPricePer1K: 0, OutputPricePer1K: 0}},
		{"unknown-model", ModelPricing{InputPricePer1K: 0.002, OutputPricePer1K: 0.008}}, // fallback
	}

	for _, tt := range tests {
		got := ResolvePricing(tt.model)
		if got != tt.expected {
			t.Errorf("ResolvePricing(%q) = %+v, want %+v", tt.model, got, tt.expected)
		}
	}
}

func TestCalculateCost(t *testing.T) {
	p := ModelPricing{InputPricePer1K: 0.00250, OutputPricePer1K: 0.01000}
	cost := p.CalculateCost(1000, 500)
	// (1000 * 0.00250 / 1000) + (500 * 0.01000 / 1000) = 0.0025 + 0.005 = 0.0075
	want := 0.0075
	if cost != want {
		t.Errorf("CalculateCost(1000, 500) = %f, want %f", cost, want)
	}
}
