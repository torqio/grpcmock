package mocker

import (
	"sync"

	"github.com/google/uuid"
)

type ReturnFunc func() []any

type SingleExpectedCall struct {
	args          []any
	returns       []any
	returnFunc    ReturnFunc
	cachedReturns []any
	id            string
	actualCalls   int
	isDefault     bool
	mu            *sync.RWMutex
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

func newSingleExpectedCallWithFunc(args []any, returnFunc ReturnFunc) SingleExpectedCall {
	var mu sync.RWMutex
	return SingleExpectedCall{
		args:       args,
		returnFunc: returnFunc,
		id:         uuid.NewString(),
		mu:         &mu,
	}
}

func (s *SingleExpectedCall) IsDefault() bool {
	return s.isDefault
}

func (s *SingleExpectedCall) Returns() []any {
	if s.returnFunc != nil {
		// Cache the result to prevent multiple executions
		if s.cachedReturns == nil {
			s.cachedReturns = s.returnFunc()
		}
		return s.cachedReturns
	}
	return s.returns
}

func (s *SingleExpectedCall) setDefault() {
	s.isDefault = true
}

func (s *SingleExpectedCall) call() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.actualCalls++

	s.cachedReturns = nil
}

func (s *SingleExpectedCall) timesCalled() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.actualCalls
}
