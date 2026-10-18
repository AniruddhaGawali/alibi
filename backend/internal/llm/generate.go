package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/models/operations"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"
	"github.com/OpenRouterTeam/go-sdk/retry"
)

var (
	// ErrDisabled means OPENROUTER_API_KEY is not set.
	ErrDisabled = errors.New("llm: openrouter is not configured")
	// ErrTimeout means the model did not answer within the request timeout.
	ErrTimeout = errors.New("llm: model timed out")
	// ErrUpstream means OpenRouter or the provider failed.
	ErrUpstream = errors.New("llm: model request failed")
	// ErrBadOutput means the answer was empty or was not plain text (a tool call,
	// an image, audio). It is thrown away rather than shown.
	ErrBadOutput = errors.New("llm: model returned no usable text")
)

// DenyDataCollection asks OpenRouter to route only to providers that do not store or
// train on prompts (data_collection: "deny"). If no provider of a case's model meets
// that, the call fails with an upstream error instead of falling back.
const DenyDataCollection = true

// maxReplyTokens hard-caps the answer; the prompt asks for MaxReplyWords.
const maxReplyTokens = 160

// Turn is one earlier question and answer of the same interrogation.
type Turn struct {
	Question string
	Answer   string
}

// Request is one interrogation call.
type Request struct {
	Model    string // OpenRouter model id; DefaultModel when empty or invalid
	System   string // from BuildSystemPrompt
	History  []Turn // oldest first
	Question string // already moderated
	User     string // stable per-player id for OpenRouter's abuse isolation (never the email)
}

// Reply is the model's answer.
type Reply struct {
	Text  string
	Model string // the model that actually answered
	// TotalTokens is what OpenRouter says the call used (prompt and answer); 0 when it did not say.
	TotalTokens int64
}

// Generator produces a suspect's answer. *Clients implements it; tests use fakes.
type Generator interface {
	// Enabled reports whether a model provider is configured.
	Enabled() bool
	Generate(ctx context.Context, req Request) (*Reply, error)
}

// Enabled reports whether OpenRouter is configured (OPENROUTER_API_KEY is set).
func (c *Clients) Enabled() bool { return c.OpenRouter != nil }

// Generate sends one text-to-text chat completion. The request is locked down so
// the model can only write text: no tools or tool choice, no plugins (web search
// and file parsing are switched off explicitly in case the account enables them by
// default), output modalities limited to text, and no SDK retries (a retry would
// blow the latency budget; the caller refunds and the player can ask again).
func (c *Clients) Generate(ctx context.Context, req Request) (*Reply, error) {
	if c.OpenRouter == nil {
		return nil, ErrDisabled
	}
	off := false
	chat := components.ChatRequest{
		Model:               openrouter.String(ModelOrDefault(req.Model)),
		Messages:            buildMessages(req),
		Modalities:          []components.Modality{components.ModalityText},
		MaxCompletionTokens: optionalnullable.From(openrouter.Pointer(int64(maxReplyTokens))),
		Temperature:         optionalnullable.From(openrouter.Pointer(0.8)),
		Stream:              openrouter.Pointer(false),
		Plugins: []components.ChatRequestPlugin{
			components.CreateChatRequestPluginWeb(components.WebSearchPlugin{ID: components.WebSearchPluginIDWeb, Enabled: &off}),
			components.CreateChatRequestPluginFileParser(components.FileParserPlugin{ID: components.FileParserPluginIDFileParser, Enabled: &off}),
		},
	}
	if DenyDataCollection {
		// Providers that log or train on prompts are skipped: the prompt holds the case's secrets.
		chat.Provider = optionalnullable.From(&components.ProviderPreferences{
			DataCollection: optionalnullable.From(openrouter.Pointer(components.DataCollectionDeny)),
		})
	}
	if req.User != "" {
		chat.User = openrouter.String(req.User)
	}

	res, err := c.OpenRouter.Chat.Send(ctx, chat, nil,
		operations.WithRetries(retry.Config{Strategy: "none"}),
		operations.WithOperationTimeout(c.timeout),
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %w", ErrTimeout, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrUpstream, err)
	}
	if res == nil || res.ChatResult == nil {
		return nil, fmt.Errorf("%w: no result", ErrBadOutput)
	}
	return replyFrom(res.ChatResult)
}

func buildMessages(req Request) []components.ChatMessages {
	msgs := make([]components.ChatMessages, 0, 2+2*len(req.History))
	msgs = append(msgs, components.CreateChatMessagesSystem(components.ChatSystemMessage{
		Role:    components.ChatSystemMessageRoleSystem,
		Content: components.CreateChatSystemMessageContentStr(req.System),
	}))
	for _, t := range req.History {
		msgs = append(msgs, userMessage(t.Question))
		answer := components.CreateChatAssistantMessageContentStr(t.Answer)
		msgs = append(msgs, components.CreateChatMessagesAssistant(components.ChatAssistantMessage{
			Role:    components.ChatAssistantMessageRoleAssistant,
			Content: optionalnullable.From(&answer),
		}))
	}
	return append(msgs, userMessage(req.Question))
}

func userMessage(text string) components.ChatMessages {
	return components.CreateChatMessagesUser(components.ChatUserMessage{
		Role:    components.ChatUserMessageRoleUser,
		Content: components.CreateChatUserMessageContentStr(text),
	})
}

// replyFrom accepts only a plain text answer. Anything else (tool calls, images,
// audio, a content-filter stop) is rejected so it never reaches the player.
func replyFrom(r *components.ChatResult) (*Reply, error) {
	if len(r.Choices) == 0 {
		return nil, fmt.Errorf("%w: no choices", ErrBadOutput)
	}
	ch := r.Choices[0]
	m := ch.Message
	if len(m.ToolCalls) > 0 || len(m.Images) > 0 || m.Audio != nil {
		return nil, fmt.Errorf("%w: non-text output", ErrBadOutput)
	}
	if ch.FinishReason != nil {
		switch *ch.FinishReason {
		case components.ChatFinishReasonEnumToolCalls, components.ChatFinishReasonEnumContentFilter, components.ChatFinishReasonEnumError:
			return nil, fmt.Errorf("%w: finish reason %s", ErrBadOutput, *ch.FinishReason)
		}
	}
	content, ok := m.Content.Get()
	if !ok || content == nil || content.Str == nil {
		return nil, fmt.Errorf("%w: no text content", ErrBadOutput)
	}
	text := strings.TrimSpace(*content.Str)
	if text == "" {
		return nil, fmt.Errorf("%w: empty text", ErrBadOutput)
	}
	reply := &Reply{Text: text, Model: r.Model}
	if r.Usage != nil {
		reply.TotalTokens = r.Usage.TotalTokens
	}
	return reply, nil
}

// defaultTimeout bounds one interrogation call when NewClients gets none.
const defaultTimeout = 20 * time.Second
