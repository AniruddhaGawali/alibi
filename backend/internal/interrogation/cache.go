package interrogation

import (
	"context"
	"sync"
	"time"

	"backend/internal/llm"
)

// prepTTL is how long a case, its suspect and the suspect's system prompt are
// reused. They change only when an admin edits the story, so a minute of
// staleness is invisible and saves two Postgres reads and the prompt build on
// every question.
const prepTTL = time.Minute

// maxPrepared bounds the cache; when it is full everything is dropped.
const maxPrepared = 512

// prepared is everything about one (case, suspect) pair that a question needs and
// that does not depend on the player.
type prepared struct {
	cfg      *CaseConfig
	target   *Suspect
	locked   string // system prompt without the suspect's secrets, until the detective touches on them
	unlocked string // system prompt with the secrets (llm.Unlocked)
	expires  time.Time
}

type prepCache struct {
	ttl time.Duration
	now func() time.Time
	mu  sync.Mutex
	m   map[string]*prepared
}

func newPrepCache(ttl time.Duration) *prepCache {
	return &prepCache{ttl: ttl, now: time.Now, m: map[string]*prepared{}}
}

func (c *prepCache) get(key string) *prepared {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.m[key]
	if p == nil || !c.now().Before(p.expires) {
		delete(c.m, key)
		return nil
	}
	return p
}

func (c *prepCache) put(key string, p *prepared) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= maxPrepared {
		clear(c.m)
	}
	p.expires = c.now().Add(c.ttl)
	c.m[key] = p
}

// prepare loads (or reuses) the case, the target suspect and the system prompt.
func (s *Service) prepare(ctx context.Context, storyID, suspectID string) (*prepared, error) {
	key := storyID + "/" + suspectID
	if p := s.cache.get(key); p != nil {
		return p, nil
	}
	c, err := s.store.Case(ctx, storyID)
	if err != nil {
		return nil, err
	}
	target, others, err := s.store.Suspect(ctx, storyID, suspectID)
	if err != nil {
		return nil, err
	}
	p := &prepared{
		cfg:    c,
		target: target,
		// Both prompts are identical from call to call, so providers can cache their prefix.
		locked:   llm.BuildSystemPrompt(llm.CaseInfo{Title: c.Title, Plot: c.Plot}, target.Persona, others, false),
		unlocked: llm.BuildSystemPrompt(llm.CaseInfo{Title: c.Title, Plot: c.Plot}, target.Persona, others, true),
	}
	s.cache.put(key, p)
	return p, nil
}
