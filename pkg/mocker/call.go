package mocker

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type singleExpectedCall struct {
	args          []any
	returns       []any
	id            string
	expectedCalls int
	actualCalls   int
	mu            *sync.RWMutex
}

func newSingleExpectedCall(args []any, returns []any) singleExpectedCall {
	var mu sync.RWMutex
	return singleExpectedCall{
		args:    args,
		returns: returns,
		id:      uuid.NewString(),
		mu:      &mu,
	}
}

func (s singleExpectedCall) call() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.actualCalls++
}

func (s singleExpectedCall) assertExpectation() error {
	if s.expectedCalls == 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.expectedCalls != s.actualCalls {
		return fmt.Errorf("expected to be called %d times, but actually called %d times", s.expectedCalls, s.actualCalls)
	}

	return nil
}
