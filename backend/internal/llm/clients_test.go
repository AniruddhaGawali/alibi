package llm

import "testing"

func TestNewClientsOpenRouterOptional(t *testing.T) {
	c, err := NewClients("http://localhost:11434", "", 0)
	if err != nil || c.Ollama == nil || c.OpenRouter != nil {
		t.Fatalf("err=%v ollama=%v openrouter=%v", err, c.Ollama, c.OpenRouter)
	}
	c, err = NewClients("http://localhost:11434", "sk-test", 0)
	if err != nil || c.OpenRouter == nil {
		t.Fatalf("err=%v openrouter=%v", err, c.OpenRouter)
	}
}

func TestNewClientsRejectsBadHost(t *testing.T) {
	if _, err := NewClients("http://bad host", "", 0); err == nil {
		t.Fatal("expected error for invalid OLLAMA_HOST")
	}
}
