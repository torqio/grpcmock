package mocker

import (
	"sync"

	"github.com/google/uuid"
)

type SingleExpectedCall struct {
	args        []any
	returns     []any
	id          string
	actualCalls int
	isDefault   bool
	mu          *sync.RWMutex
}

func newSingleExpectedCall(args []any, returns []any) SingleExpectedCall {
	var mu sync.RWMutex
	return SingleExpectedCall{
		args:    args,
		returns: returns,
		id:      uuid.NewString(),
		mu:      &mu,
	}
}

func (s *SingleExpectedCall) IsDefault() bool {
	return s.isDefault
}

func (s *SingleExpectedCall) Returns() []any {
	return s.returns
}

func (s *SingleExpectedCall) setDefault() {
	s.isDefault = true
}

func (s *SingleExpectedCall) call() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.actualCalls++
}

func (s *SingleExpectedCall) timesCalled() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.actualCalls
}
