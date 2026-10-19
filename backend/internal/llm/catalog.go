package llm

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/models/operations"
)

// catalogTTL is how long the list of usable models is reused. OpenRouter adds and
// retires models rarely, and the list is a large response.
const catalogTTL = 10 * time.Minute

// ModelInfo is an OpenRouter model a case may use, as the admin model picker shows it.
type ModelInfo struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	ContextLength int64   `json:"context_length"`
	PromptPrice   float64 `json:"prompt_price"`     // USD per million prompt tokens
	AnswerPrice   float64 `json:"completion_price"` // USD per million answer tokens
	Free          bool    `json:"free"`
}

type modelCatalog struct {
	mu      sync.Mutex
	models  []ModelInfo
	fetched time.Time
}

// ListTextModels returns the OpenRouter models a case may be assigned: text output
// only, a valid id (no routers, no :online), and within MaxPricePerMillion. It is
// what CheckTextModel accepts, so an admin can only pick models that will be saved.
// The list is cached for catalogTTL; a failed refresh keeps serving the old list.
// It returns ErrDisabled when no API key is configured.
func (c *Clients) ListTextModels(ctx context.Context) ([]ModelInfo, error) {
	if c.OpenRouter == nil {
		return nil, ErrDisabled
	}
	c.catalog.mu.Lock()
	defer c.catalog.mu.Unlock()
	if c.catalog.models != nil && time.Since(c.catalog.fetched) < catalogTTL {
		return c.catalog.models, nil
	}
	all := "all"
	res, err := c.OpenRouter.Models.List(ctx, &operations.GetModelsRequest{OutputModalities: &all})
	if err != nil {
		if c.catalog.models != nil {
			return c.catalog.models, nil // stale beats nothing; CheckTextModel still verifies on save
		}
		return nil, fmt.Errorf("llm: list models: %w", err)
	}
	c.catalog.models, c.catalog.fetched = textModels(res.Result.Data), time.Now()
	return c.catalog.models, nil
}

// maxDescription bounds each description sent to the admin panel.
const maxDescription = 300

// textModels keeps the models a case may use, cheapest first, then by name.
func textModels(all []components.Model) []ModelInfo {
	out := make([]ModelInfo, 0, len(all))
	for _, m := range all {
		o := m.Architecture.OutputModalities
		if len(o) != 1 || o[0] != components.OutputModalityText || !ValidModelID(m.ID) {
			continue
		}
		if tooExpensive(m.Pricing.Prompt) || tooExpensive(m.Pricing.Completion) {
			continue
		}
		info := ModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			PromptPrice: perMillion(m.Pricing.Prompt),
			AnswerPrice: perMillion(m.Pricing.Completion),
		}
		info.Free = strings.HasSuffix(m.ID, ":free") || (info.PromptPrice == 0 && info.AnswerPrice == 0)
		if m.ContextLength != nil {
			info.ContextLength = *m.ContextLength
		}
		if m.Description != nil {
			info.Description = truncate(*m.Description, maxDescription)
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		pi, pj := out[i].PromptPrice+out[i].AnswerPrice, out[j].PromptPrice+out[j].AnswerPrice
		if pi != pj {
			return pi < pj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func perMillion(perToken string) float64 {
	v, err := strconv.ParseFloat(perToken, 64)
	if err != nil {
		return 0
	}
	return v * 1_000_000
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}
