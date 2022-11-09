package mocker

import (
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type singleExpectedCall struct {
	args    []any
	returns []any
	id      string
}

func newSingleExpectedCall(args []any, returns []any) singleExpectedCall {
	return singleExpectedCall{
		args:    args,
		returns: returns,
		id:      uuid.NewString(),
	}
}

type Matcher interface {
	// Matches returns whether x is a match.
	Matches(x any) bool
}

type Mocker struct {
	expectedCalls map[string][]singleExpectedCall

	// default calls giving the option to supply a default return value for a method which will be returned
	// in case no other calls in expectedCalls matched
	defaultCalls map[string]singleExpectedCall

	mu sync.RWMutex
	t  *testing.T
}

func NewMocker() *Mocker {
	return &Mocker{
		expectedCalls: make(map[string][]singleExpectedCall),
		defaultCalls:  make(map[string]singleExpectedCall),
	}
}

// SetT sets the `t` attribute of Mocker to log ongoing errors
func (m *Mocker) SetT(t *testing.T) {
	m.t = t
}

// LogError will log the given err message in m.t, if set
func (m *Mocker) LogError(err error) {
	if m.t == nil {
		return
	}
	m.t.Helper()
	m.t.Errorf("grpcmock ERROR: %v", err)
}

func (m *Mocker) findMatchingCall(method string, args ...any) (singleExpectedCall, error) {
	m.mu.RLock()
	calls, ok := m.expectedCalls[method]
	defer m.mu.RUnlock()

	// Try to find a matching call
	for _, call := range calls {
		if len(call.args) != len(args) {
			return singleExpectedCall{}, fmt.Errorf("got unexpected length of argument for methhod %v. Expected %d args, got %d", method, len(call.args), len(args))
		}
		matches := true
		for i, arg := range call.args {
			matcher := Eq(arg)
			if v, ok := arg.(Matcher); ok {
				matcher = v
			}
			if !matcher.Matches(args[i]) {
				matches = false
				break
			}
		}

		if matches {
			return call, nil
		}
	}

	// No matching call, checking if we have default for that method
	m.mu.RLock()
	call, ok := m.defaultCalls[method]
	m.mu.RUnlock()

	if ok {
		return call, nil
	}

	return singleExpectedCall{}, fmt.Errorf("no matching expected call nor default retrurn for method %v with given arguments. "+
		"Use Configure().%v() to configure an expected call or default return value", method, method)
}

// AddExpectedCall add a call to the expected call chain with the given expected args and the values to return
func (m *Mocker) AddExpectedCall(method string, args []any, returns []any) DeletableCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	newCall := newSingleExpectedCall(args, returns)
	m.expectedCalls[method] = append(m.expectedCalls[method], newCall)

	return DeletableCall{
		method: method,
		call:   newCall,
		mocker: m,
	}
}

// SetDefaultCall sets a default call for the provided method that will return the provided values
func (m *Mocker) SetDefaultCall(method string, returns []any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.defaultCalls[method] = newSingleExpectedCall([]any{}, returns)
}

// Call try to find a matching call for the given method with the given arguments.
// It will return an array of the return values defined when you define the call with AddExpectedCall
// If no call was found, an error will be returned.
func (m *Mocker) Call(method string, args ...any) ([]any, error) {
	matchedCall, err := m.findMatchingCall(method, args...)
	if err != nil {
		return nil, err
	}

	return matchedCall.returns, nil
}

// ResetAll deletes all the expected calls and default calls of all methods for this mock server.
func (m *Mocker) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.expectedCalls = make(map[string][]singleExpectedCall)
	m.defaultCalls = make(map[string]singleExpectedCall)
}

// ResetCall deletes all the expected call and the default call for a specific method
func (m *Mocker) ResetCall(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.expectedCalls[method] = nil
	delete(m.defaultCalls, method)
}

// UnsetDefaultCall deletes the default call for a specific method.
func (m *Mocker) UnsetDefaultCall(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.defaultCalls, method)
}

// DeleteCall delete a specific call ID from a method
func (m *Mocker) DeleteCall(method, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := m.expectedCalls[method]
	callIndex := -1
	for i, call := range calls {
		if call.id == id {
			callIndex = i
			break
		}
	}

	if callIndex < 0 {
		return
	}
	m.expectedCalls[method] = append(calls[:callIndex], calls[callIndex+1:]...)
}

// DeletableCall is used as a wrapper returned by Mocker.AddExpectedCall to allow a plain Delete() method which will
// delete that specific added call.
type DeletableCall struct {
	method string
	call   singleExpectedCall
	mocker *Mocker
}

// Delete deletes this call from the expected call array.
func (d *DeletableCall) Delete() {
	d.mocker.DeleteCall(d.method, d.call.id)
}
