package llm

import (
	"context"
	"sync"
)

// FakeLLM is a test double for the generative language model.
type FakeLLM struct {
	mu           sync.Mutex
	response     string
	failWith     error
	calls        int
	lastPrompt   string
	responseFunc func(prompt string) string
}

// NewFakeLLM creates a FakeLLM with a default response.
func NewFakeLLM(defaultResponse string) *FakeLLM {
	return &FakeLLM{
		response: defaultResponse,
	}
}

// SetResponse updates the static canned response.
func (f *FakeLLM) SetResponse(resp string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.response = resp
}

// SetResponseFunc allows dynamic generation based on prompt content.
func (f *FakeLLM) SetResponseFunc(fn func(prompt string) string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responseFunc = fn
}

// SetError instructs the fake LLM to return an error on subsequent calls.
func (f *FakeLLM) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failWith = err
}

// Calls returns the invocation count.
func (f *FakeLLM) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// LastPrompt returns the most recent prompt passed to Generate.
func (f *FakeLLM) LastPrompt() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastPrompt
}

// Generate returns the configured canned response or injected error.
func (f *FakeLLM) Generate(ctx context.Context, prompt string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++
	f.lastPrompt = prompt

	if f.failWith != nil {
		return "", f.failWith
	}

	if f.responseFunc != nil {
		return f.responseFunc(prompt), nil
	}

	return f.response, nil
}
