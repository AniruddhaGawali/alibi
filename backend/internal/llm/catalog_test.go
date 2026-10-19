package llm

import (
	"testing"

	"github.com/OpenRouterTeam/go-sdk/models/components"
)

func model(id string, out []components.OutputModality, prompt, completion string) components.Model {
	return components.Model{
		ID: id, Name: id,
		Architecture: components.ModelArchitecture{OutputModalities: out},
		Pricing:      components.PublicPricing{Prompt: prompt, Completion: completion},
	}
}

// The picker offers exactly what CheckTextModel would accept.
func TestTextModelsFilters(t *testing.T) {
	text := []components.OutputModality{components.OutputModalityText}
	got := textModels([]components.Model{
		model("google/gemma-4-31b-it", text, "0.0000001", "0.0000002"),
		model("google/gemma-3-27b-it:free", text, "0", "0"),
		model("big/pricey", text, "0.00002", "0.00008"),
		model("google/gemini-2.5-flash-image", []components.OutputModality{components.OutputModalityText, components.OutputModalityImage}, "0", "0"),
		model("openrouter/auto", text, "0", "0"),
		model("someone/web:online", text, "0", "0"),
		model("broken/price", text, "n/a", "0"),
	})
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	// Cheapest first: the free model leads.
	if got[0].ID != "google/gemma-3-27b-it:free" || !got[0].Free || got[1].ID != "google/gemma-4-31b-it" || got[1].Free {
		t.Fatalf("order/free: %+v", got)
	}
	if p := got[1].PromptPrice; p < 0.0999 || p > 0.1001 {
		t.Fatalf("prompt price per million %v", p)
	}
}
