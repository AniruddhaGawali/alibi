// Package llm initializes the LLM provider clients: Ollama for local model
// testing and OpenRouter for cloud model access.
package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/ollama/ollama/api"

	"backend/pkg/utils"
)

type Clients struct {
	Ollama     *api.Client
	OpenRouter *openrouter.OpenRouter // nil when no API key is configured
}

// NewClients builds both clients. ollamaHost is e.g. http://localhost:11434.
// An empty openRouterKey leaves OpenRouter disabled rather than failing, so
// local development works with Ollama alone.
func NewClients(ollamaHost, openRouterKey string) (*Clients, error) {
	u, err := url.Parse(ollamaHost)
	if err != nil {
		return nil, fmt.Errorf("invalid OLLAMA_HOST %q: %w", ollamaHost, err)
	}
	c := &Clients{Ollama: api.NewClient(u, http.DefaultClient)}

	if openRouterKey != "" {
		c.OpenRouter = openrouter.New(
			openrouter.WithSecurity(openRouterKey),
			openrouter.WithTimeout(30*time.Second),
		)
		utils.LogInfo("[LLM] OpenRouter client initialized")
	} else {
		utils.LogInfo("[LLM] OPENROUTER_API_KEY not set; OpenRouter disabled")
	}
	return c, nil
}

// PingOllama checks whether the local Ollama server is reachable. Failure is
// non-fatal: Ollama is optional outside local testing.
func (c *Clients) PingOllama(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.Ollama.Heartbeat(ctx); err != nil {
		utils.LogInfo("[LLM] Ollama not reachable (optional)", "err", err)
		return
	}
	utils.LogInfo("[LLM] Ollama connected successfully")
}
