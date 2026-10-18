package interrogation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"backend/internal/chat"
	"backend/internal/llm"
	"backend/internal/wallet"
	"backend/pkg/utils"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// historyTurns is how many earlier exchanges with the suspect are sent back to
// the model so it remembers the conversation.
const historyTurns = 6

// historyChars caps the characters of earlier exchanges sent to the model, which
// bounds the input size of a call.
const historyChars = 3000

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
	// Balance reads the token balance (Redis, falling back to Postgres).
	Balance(ctx context.Context, userID string) (int64, error)
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
	// MessageID is the id the exchange gets in the chat history, so the client can
	// show it under its final id from the start.
	MessageID string `json:"message_id"`
}

// BlockedError is ErrBlocked with the reason moderation gave.
type BlockedError struct{ Verdict llm.Verdict }

func (e *BlockedError) Error() string {
	return "interrogation: question blocked (" + string(e.Verdict) + ")"
}
func (e *BlockedError) Is(target error) bool { return target == ErrBlocked }

// RefundedError wraps a model failure after which the spent token was given back.
type RefundedError struct{ Err error }

func (e *RefundedError) Error() string { return "token refunded: " + e.Err.Error() }
func (e *RefundedError) Unwrap() error { return e.Err }

// FailedError wraps a model failure after which the token was NOT given back: the
// reply may already have been billed (an unusable answer), the player used up
// today's refunds, or the refund itself failed. Callers must not promise a refund.
type FailedError struct{ Err error }

func (e *FailedError) Error() string { return "token kept: " + e.Err.Error() }
func (e *FailedError) Unwrap() error { return e.Err }

// generateTimeout is the hard stop for one model call, whatever the client does.
const generateTimeout = 2 * time.Minute

// maxFailedEvents bounds the in-memory list of chat events that could not be queued.
const maxFailedEvents = 200

type Service struct {
	store   Store
	gen     llm.Generator
	wallet  Wallet
	events  Publisher
	history History // nil: every question is answered without memory
	guard   Guard   // nil: no rate limits
	cache   *prepCache
	pending sync.WaitGroup // in-flight event publishes, drained by Wait

	failedMu sync.Mutex
	failed   []chat.Event // exchanges the queue refused; retried on the next publish and on Wait
}

// Option customises a Service.
type Option func(*Service)

// WithGuard installs the rate limits, daily budgets and circuit breaker.
func WithGuard(g Guard) Option { return func(s *Service) { s.guard = g } }

func NewService(store Store, gen llm.Generator, wallet Wallet, events Publisher, history History, opts ...Option) *Service {
	s := &Service{store: store, gen: gen, wallet: wallet, events: events, history: history, cache: newPrepCache(prepTTL)}
	for _, o := range opts {
		o(s)
	}
	return s
}

// AdmitIP charges the caller's address against the per-minute budget; the handler
// calls it before reading the body.
func (s *Service) AdmitIP(ctx context.Context, ip string) error {
	if s.guard == nil {
		return nil
	}
	return s.guard.AdmitIP(ctx, ip)
}

// GenerateSuspectResponse answers in as the suspect. In order: the player's
// request budget, moderation (before any token is spent, so a blocked question
// costs nothing and needs no refund), a cheap balance check, the active case
// lock, the case, the suspect and the recent conversation (read concurrently; the
// case and suspect from a short-lived cache), the player's model slot and daily
// allowance, spending one token (Redis DECR), the model call, and then the
// exchange is published to the chat queue in the background.
//
// The model call does not depend on the client's connection: it runs to the end
// even if the player closes the request, so aborting a request cannot be used to
// get billed calls for free. If the model fails before answering (a timeout or an
// upstream error) the token is refunded (INCR), at most a few times a day; an
// unusable answer keeps the token. The failure is returned as a *RefundedError or
// a *FailedError.
func (s *Service) GenerateSuspectResponse(ctx context.Context, in Input) (*Result, error) {
	if s.guard != nil {
		if err := s.guard.Admit(ctx, in.UserID); err != nil {
			return nil, err
		}
	}
	if v := llm.Moderate(in.Question); v != llm.Allowed {
		if v == llm.Injection && s.guard != nil {
			s.guard.Strike(ctx, in.UserID)
		}
		return nil, &BlockedError{Verdict: v}
	}
	if !s.gen.Enabled() { // checked before spending, so there is nothing to refund
		return nil, llm.ErrDisabled
	}
	// Out-of-tokens players are turned away by one Redis read, before any Postgres or Mongo work.
	if bal, err := s.wallet.Balance(ctx, in.UserID); err != nil {
		return nil, fmt.Errorf("read balance: %w", err)
	} else if bal < 1 {
		return nil, ErrNoTokens
	}

	var (
		lock    *Lock
		prep    *prepared
		history []llm.Turn
		lockErr error
		prepErr error
	)
	var g errgroup.Group
	g.Go(func() error { lock, lockErr = s.store.Lock(ctx, in.UserID); return nil })
	g.Go(func() error { prep, prepErr = s.prepare(ctx, in.StoryID, in.SuspectID); return nil })
	g.Go(func() error { history = s.recentTurns(ctx, in.UserID, in.StoryID, in.SuspectID); return nil })
	_ = g.Wait()
	if lockErr != nil {
		return nil, fmt.Errorf("load lock: %w", lockErr)
	}
	if lock == nil || lock.StoryID != in.StoryID {
		return nil, ErrNotInCase
	}
	if prepErr != nil {
		return nil, prepErr
	}

	release := func() {}
	if s.guard != nil {
		var err error
		if release, err = s.guard.Begin(ctx, in.UserID); err != nil {
			return nil, err
		}
	}
	defer release()

	remaining, err := s.wallet.SpendQuestion(ctx, in.UserID)
	if errors.Is(err, wallet.ErrInsufficient) {
		return nil, ErrNoTokens
	}
	if err != nil {
		return nil, fmt.Errorf("spend token: %w", err)
	}

	model := llm.ModelOrDefault(prep.cfg.Model)
	started := time.Now()
	gctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), generateTimeout)
	defer cancel()
	reply, err := s.gen.Generate(gctx, llm.Request{
		Model:    model,
		System:   prep.system,
		History:  history,
		Question: in.Question,
		User:     in.UserID,
	})
	if s.guard != nil {
		var tokens int64
		if reply != nil {
			tokens = reply.TotalTokens
		}
		s.guard.Finish(ctx, tokens, err)
	}
	if err != nil {
		if s.settle(ctx, in.UserID, err) {
			return nil, &RefundedError{Err: err}
		}
		return nil, &FailedError{Err: err}
	}

	text := reply.Text
	if llm.LeaksSecret(text, prep.target.Persona) {
		utils.LogWarnCtx(ctx, "suspect reply leaked a secret; replaced", "user_id", in.UserID, "story_id", in.StoryID, "suspect_id", in.SuspectID)
		text = llm.LeakReply
	}
	if ctx.Err() != nil {
		utils.LogInfoCtx(ctx, "client left before the answer; it is stored in their log", "user_id", in.UserID, "story_id", in.StoryID)
	}
	utils.LogInfoCtx(ctx, "suspect answered", "user_id", in.UserID, "story_id", in.StoryID,
		"suspect_id", in.SuspectID, "model", reply.Model, "latency_ms", time.Since(started).Milliseconds(), "tokens", reply.TotalTokens)

	ev := chat.Event{
		EventID: uuid.NewString(), UserID: in.UserID, CaseID: in.StoryID, PlayID: lock.CaseID,
		SuspectID: in.SuspectID, SuspectName: prep.target.Name, UserQuery: in.Question, AIResponse: text,
		Model: model, Timestamp: time.Now().UTC(),
	}
	s.publish(ctx, ev)
	return &Result{SuspectID: in.SuspectID, Reply: text, TokensRemaining: remaining, MessageID: ev.EventID}, nil
}

// countsAsOutage reports whether a model error means the upstream is down or out
// of credit (as opposed to the model answering with something unusable).
func countsAsOutage(err error) bool {
	return errors.Is(err, llm.ErrUpstream) || errors.Is(err, llm.ErrTimeout)
}

// settle decides whether a failed call gets its token back and does it. A call
// that failed before the model answered (timeout, upstream error) is refunded,
// within the player's daily refund allowance; an unusable answer (empty, filtered,
// a tool call) is not, since the provider has already generated and billed it. It
// reports whether the token went back.
func (s *Service) settle(ctx context.Context, userID string, cause error) bool {
	if !countsAsOutage(cause) {
		utils.LogWarnCtx(ctx, "model gave no usable answer; token kept", "user_id", userID, "err", cause)
		return false
	}
	if s.guard != nil && !s.guard.AllowRefund(ctx, userID) {
		utils.LogWarnCtx(ctx, "model call failed; refund allowance for today is used up", "user_id", userID, "err", cause)
		return false
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := s.wallet.RefundQuestion(rctx, userID); err != nil {
		utils.LogErrorCtx(ctx, "token refund failed", "user_id", userID, "cause", cause, "err", err)
		return false
	}
	utils.LogWarnCtx(ctx, "model call failed; token refunded", "user_id", userID, "err", cause)
	return true
}

// publish queues the exchange for MongoDB without making the player wait for it.
// A failed attempt is retried with a short backoff; an exchange the queue still
// refuses is kept in a small list and tried again with the next one (and on Wait).
func (s *Service) publish(ctx context.Context, ev chat.Event) {
	pctx := context.WithoutCancel(ctx)
	s.pending.Add(1)
	go func() {
		defer s.pending.Done()
		if err := s.publishRetry(pctx, ev); err != nil {
			s.keepFailed(pctx, ev, err)
			return
		}
		s.retryFailed(pctx)
	}()
}

// publishRetry sends one event, up to three attempts.
func (s *Service) publishRetry(ctx context.Context, ev chat.Event) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		utils.LogErrorCtx(ctx, "encode chat event failed", "event_id", ev.EventID, "err", err)
		return nil // retrying cannot fix an unencodable event
	}
	var last error
	for attempt, wait := 0, time.Duration(0); attempt < 3; attempt, wait = attempt+1, wait*3+200*time.Millisecond {
		time.Sleep(wait)
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		last = s.events.Publish(c, chat.Topic, payload)
		cancel()
		if last == nil {
			return nil
		}
		utils.LogWarnCtx(ctx, "publish chat event failed; retrying", "event_id", ev.EventID, "user_id", ev.UserID, "attempt", attempt+1, "err", last)
	}
	return last
}

func (s *Service) keepFailed(ctx context.Context, ev chat.Event, cause error) {
	s.failedMu.Lock()
	defer s.failedMu.Unlock()
	if len(s.failed) >= maxFailedEvents {
		utils.LogErrorCtx(ctx, "chat event lost: retry list is full", "event_id", ev.EventID, "user_id", ev.UserID, "err", cause)
		return
	}
	s.failed = append(s.failed, ev)
	utils.LogErrorCtx(ctx, "publish chat event failed; kept for retry", "event_id", ev.EventID, "user_id", ev.UserID, "waiting", len(s.failed), "err", cause)
}

// retryFailed gives the events the queue refused earlier one more go.
func (s *Service) retryFailed(ctx context.Context) {
	s.failedMu.Lock()
	batch := s.failed
	s.failed = nil
	s.failedMu.Unlock()
	for _, ev := range batch {
		payload, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = s.events.Publish(c, chat.Topic, payload)
		cancel()
		if err != nil {
			s.keepFailed(ctx, ev, err)
		}
	}
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
	// Keep the newest turns that fit the character budget, so the input size (and the
	// cost of a call) stays bounded however long the earlier answers were.
	turns := make([]llm.Turn, 0, len(msgs))
	used := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		used += utf8.RuneCountInString(msgs[i].UserQuery) + utf8.RuneCountInString(msgs[i].AIResponse)
		if used > historyChars && len(turns) > 0 {
			break
		}
		turns = append(turns, llm.Turn{Question: msgs[i].UserQuery, Answer: msgs[i].AIResponse})
	}
	slices.Reverse(turns)
	return turns
}

// Wait blocks until background publishes have finished and gives the exchanges the
// queue refused earlier a last try; call it on shutdown.
func (s *Service) Wait() {
	s.pending.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.retryFailed(ctx)
}
