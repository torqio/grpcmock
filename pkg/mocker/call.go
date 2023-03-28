package mocker

import (
	"sync"

	"github.com/google/uuid"
)

type singleExpectedCall struct {
	args        []any
	returns     []any
	id          string
	actualCalls int
	mu          *sync.RWMutex
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

func (s *singleExpectedCall) call() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.actualCalls++
}

func (s *singleExpectedCall) timesCalled() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.actualCalls
}
