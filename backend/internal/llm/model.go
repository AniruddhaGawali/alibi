package llm

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/models/operations"
)

// DefaultModel is the OpenRouter model a case uses unless an admin picks another.
// It must match the column default in sql/migrations/00011_llm_interrogation.sql.
const DefaultModel = "google/gemma-4-31b-it"

// modelIDPattern is "author/slug" with an optional ":free", ":nitro" or ":floor"
// variant. Every other suffix is refused, notably ":online", which makes OpenRouter
// run a web search for the model. The database enforces the same pattern
// (stories_llm_model_format).
var modelIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*(:(free|nitro|floor))?$`)

// ValidModelID reports whether id is an OpenRouter model id a case may use.
// Router models ("openrouter/auto" and friends) are refused because they can
// pick any model, including ones that are not text-only.
func ValidModelID(id string) bool {
	return len(id) <= 100 && modelIDPattern.MatchString(id) && !strings.HasPrefix(id, "openrouter/")
}

// ModelOrDefault returns id, or DefaultModel when id is empty or not valid.
func ModelOrDefault(id string) string {
	if ValidModelID(id) {
		return id
	}
	return DefaultModel
}

var (
	// ErrUnknownModel means OpenRouter does not list the model.
	ErrUnknownModel = errors.New("llm: model not found on OpenRouter")
	// ErrNotTextOnly means the model can produce something other than text (images, audio).
	ErrNotTextOnly = errors.New("llm: model must output text only")
)

// CheckTextModel asks OpenRouter whether model exists and outputs text only, so an
// admin cannot assign an image or audio generator to a case. It returns
// ErrDisabled when no API key is configured.
func (c *Clients) CheckTextModel(ctx context.Context, model string) error {
	if c.OpenRouter == nil {
		return ErrDisabled
	}
	all := "all"
	res, err := c.OpenRouter.Models.List(ctx, &operations.GetModelsRequest{OutputModalities: &all})
	if err != nil {
		return fmt.Errorf("llm: list models: %w", err)
	}
	base := model
	if i := strings.LastIndexByte(model, ':'); i >= 0 {
		base = model[:i]
	}
	for _, m := range res.Result.Data {
		if m.ID != model && m.ID != base {
			continue
		}
		out := m.Architecture.OutputModalities
		if len(out) == 1 && out[0] == components.OutputModalityText {
			return nil
		}
		return ErrNotTextOnly
	}
	return ErrUnknownModel
}
