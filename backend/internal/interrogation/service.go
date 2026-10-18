package interrogation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"backend/internal/chat"
	"backend/internal/llm"
	"backend/internal/wallet"
	"backend/pkg/utils"
	"github.com/google/uuid"
)

// historyTurns is how many earlier exchanges with the suspect are sent back to
// the model so it remembers the conversation.
const historyTurns = 6

var (
	// ErrBlocked means moderation turned the question away. No token was spent.
	ErrBlocked = errors.New("interrogation: question blocked")
	// ErrNotInCase means the player is not locked into this case.
	ErrNotInCase = errors.New("interrogation: case not started")
	// ErrSuspectNotFound means the suspect does not belong to the case.
	ErrSuspectNotFound = errors.New("interrogation: suspect not found")
	// ErrNoTokens means the player has no question tokens left.
	ErrNoTokens = errors.New("interrogation: no tokens left")
)

// Wallet is the token balance; *wallet.Service implements it.
type Wallet interface {
	// SpendQuestion takes one token (Redis DECR, mirrored to Postgres) or
	// returns wallet.ErrInsufficient.
	SpendQuestion(ctx context.Context, userID string) (int64, error)
	// RefundQuestion gives one token back (Postgres, then Redis INCR).
	RefundQuestion(ctx context.Context, userID string) (int64, error)
}

// History reads earlier exchanges; *chat.MongoStore implements it.
type History interface {
	History(ctx context.Context, userID, caseID, suspectID string, limit int) ([]chat.Message, error)
}

// Publisher puts a message on a queue; *queue.RedisListBroker implements it.
type Publisher interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

// Input is one question from a player.
type Input struct {
	UserID    string
	StoryID   string
	SuspectID string
	Question  string // cleaned with llm.CleanQuestion, 1..llm.MaxQuestionLength runes
}

// Result is the suspect's answer.
type Result struct {
	SuspectID       string `json:"suspect_id"`
	Reply           string `json:"reply"`
	TokensRemaining int64  `json:"tokens_remaining"`
}

// RefundedError wraps a model failure after which the spent token was given back.
type RefundedError struct{ Err error }

func (e *RefundedError) Error() string { return "token refunded: " + e.Err.Error() }
func (e *RefundedError) Unwrap() error { return e.Err }

type Service struct {
	store   Store
	gen     llm.Generator
	wallet  Wallet
	events  Publisher
	history History        // nil: every question is answered without memory
	pending sync.WaitGroup // in-flight event publishes, drained by Wait
}

func NewService(store Store, gen llm.Generator, wallet Wallet, events Publisher, history History) *Service {
	return &Service{store: store, gen: gen, wallet: wallet, events: events, history: history}
}

// GenerateSuspectResponse answers in as the suspect. In order: moderation (before
// any token is spent, so a blocked question costs nothing and needs no refund),
// the active case lock, the case's model and suspects, the recent conversation,
// spending one token (Redis DECR), the model call, and then the exchange is
// published to the chat queue in the background. If the model times out or
// fails, the token is refunded (INCR) and a *RefundedError is returned.
func (s *Service) GenerateSuspectResponse(ctx context.Context, in Input) (*Result, error) {
	if llm.Moderate(in.Question) != llm.Allowed {
		return nil, ErrBlocked
	}

	lock, err := s.store.Lock(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("load lock: %w", err)
	}
	if lock == nil || lock.StoryID != in.StoryID {
		return nil, ErrNotInCase
	}
	c, err := s.store.Case(ctx, in.StoryID)
	if err != nil {
		return nil, err
	}
	suspects, err := s.store.Suspects(ctx, in.StoryID)
	if err != nil {
		return nil, fmt.Errorf("load suspects: %w", err)
	}
	var target *Suspect
	others := make([]llm.Acquaintance, 0, len(suspects))
	for i := range suspects {
		if suspects[i].ID == in.SuspectID {
			target = &suspects[i]
			continue
		}
		others = append(others, llm.AcquaintanceOf(suspects[i].Persona))
	}
	if target == nil {
		return nil, ErrSuspectNotFound
	}

	if !s.gen.Enabled() { // checked before spending, so there is nothing to refund
		return nil, llm.ErrDisabled
	}
	history := s.recentTurns(ctx, in.UserID, in.StoryID, in.SuspectID)
	remaining, err := s.wallet.SpendQuestion(ctx, in.UserID)
	if errors.Is(err, wallet.ErrInsufficient) {
		return nil, ErrNoTokens
	}
	if err != nil {
		return nil, fmt.Errorf("spend token: %w", err)
	}

	model := llm.ModelOrDefault(c.Model)
	started := time.Now()
	reply, err := s.gen.Generate(ctx, llm.Request{
		Model:    model,
		System:   llm.BuildSystemPrompt(llm.CaseInfo{Title: c.Title, Plot: c.Plot}, target.Persona, others),
		History:  history,
		Question: in.Question,
		User:     in.UserID,
	})
	if err != nil {
		s.refund(ctx, in.UserID, err)
		return nil, &RefundedError{Err: err}
	}
	utils.LogInfoCtx(ctx, "suspect answered", "user_id", in.UserID, "story_id", in.StoryID,
		"suspect_id", in.SuspectID, "model", reply.Model, "latency_ms", time.Since(started).Milliseconds())

	s.publish(ctx, chat.Event{
		EventID: uuid.NewString(), UserID: in.UserID, CaseID: in.StoryID, PlayID: lock.CaseID,
		SuspectID: in.SuspectID, SuspectName: target.Name, UserQuery: in.Question, AIResponse: reply.Text,
		Model: model, Timestamp: time.Now().UTC(),
	})
	return &Result{SuspectID: in.SuspectID, Reply: reply.Text, TokensRemaining: remaining}, nil
}

// refund gives the spent token back. It runs even if the client has gone away,
// since the player paid for an answer they did not get.
func (s *Service) refund(ctx context.Context, userID string, cause error) {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := s.wallet.RefundQuestion(rctx, userID); err != nil {
		utils.LogErrorCtx(ctx, "token refund failed", "user_id", userID, "cause", cause, "err", err)
		return
	}
	utils.LogWarnCtx(ctx, "model call failed; token refunded", "user_id", userID, "err", cause)
}

// publish queues the exchange for MongoDB without making the player wait for it.
func (s *Service) publish(ctx context.Context, ev chat.Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		utils.LogErrorCtx(ctx, "encode chat event failed", "err", err)
		return
	}
	pctx := context.WithoutCancel(ctx)
	s.pending.Add(1)
	go func() {
		defer s.pending.Done()
		c, cancel := context.WithTimeout(pctx, 5*time.Second)
		defer cancel()
		if err := s.events.Publish(c, chat.Topic, payload); err != nil {
			utils.LogErrorCtx(c, "publish chat event failed", "event_id", ev.EventID, "user_id", ev.UserID, "err", err)
		}
	}()
}

// recentTurns returns the last few exchanges with this suspect, oldest first, so
// the model remembers what was already said. It is best effort: if MongoDB is
// slow or down the question is answered without memory rather than failing.
func (s *Service) recentTurns(ctx context.Context, userID, caseID, suspectID string) []llm.Turn {
	if s.history == nil {
		return nil
	}
	hctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	msgs, err := s.history.History(hctx, userID, caseID, suspectID, historyTurns)
	if err != nil {
		utils.LogWarnCtx(ctx, "load recent chat failed; answering without memory", "user_id", userID, "err", err)
		return nil
	}
	turns := make([]llm.Turn, len(msgs))
	for i, m := range msgs {
		turns[i] = llm.Turn{Question: m.UserQuery, Answer: m.AIResponse}
	}
	return turns
}

// Wait blocks until background publishes have finished; call it on shutdown.
func (s *Service) Wait() { s.pending.Wait() }
