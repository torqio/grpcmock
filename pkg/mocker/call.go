package mocker

import (
	"sync"

	"github.com/google/uuid"
)

// DoAndReturn represents a function that can dynamically generate return values
type DoAndReturn func() []any

type SingleExpectedCall struct {
	args          []any
	returns       []any
	doAndReturn   DoAndReturn
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

func newSingleExpectedCallWithFunc(args []any, doAndReturn DoAndReturn) SingleExpectedCall {
	var mu sync.RWMutex
	return SingleExpectedCall{
		args:        args,
		doAndReturn: doAndReturn,
		id:          uuid.NewString(),
		mu:          &mu,
	}
}

func (s *SingleExpectedCall) IsDefault() bool {
	return s.isDefault
}

func (s *SingleExpectedCall) Returns() []any {
	if s.doAndReturn != nil {
		// Cache the result to prevent multiple executions
		if s.cachedReturns == nil {
			s.cachedReturns = s.doAndReturn()
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
	// Clear cached returns when called to allow fresh execution on next call
	s.cachedReturns = nil
}

func (s *SingleExpectedCall) timesCalled() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.actualCalls
}
