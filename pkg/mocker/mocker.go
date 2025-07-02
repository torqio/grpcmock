package mocker

import (
	"fmt"
	"sync"
	"testing"
)

type ErrNoMatchingCalls struct {
	Method string
}

func (e ErrNoMatchingCalls) Error() string {
	return fmt.Sprintf("no matching expected call nor default return for method %v with given arguments. "+
		"Use Configure().%v() to configure an expected call or default return value", e.Method, e.Method)
}

type Matcher interface {
	// Matches returns whether x is a match.
	Matches(x any) bool
}

type Mocker struct {
	callCount     map[string]int
	expectedCalls map[string][]*SingleExpectedCall

	// default calls giving the option to supply a default return value for a method which will be returned
	// in case no other calls in expectedCalls matched
	defaultCalls map[string]*SingleExpectedCall

	mu sync.RWMutex
	t  *testing.T
}

func NewMocker() *Mocker {
	return &Mocker{
		callCount:     make(map[string]int),
		expectedCalls: make(map[string][]*SingleExpectedCall),
		defaultCalls:  make(map[string]*SingleExpectedCall),
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

func (m *Mocker) findMatchingCall(method string, args ...any) (*SingleExpectedCall, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	calls, ok := m.expectedCalls[method]

	// Try to find a matching call
	for _, call := range calls {
		if len(call.args) != len(args) {
			return nil, fmt.Errorf("got unexpected length of argument for methhod %v. Expected %d args, got %d", method, len(call.args), len(args))
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
	call, ok := m.defaultCalls[method]

	if ok {
		return call, nil
	}

	return nil, ErrNoMatchingCalls{method}
}

// Deprecated: For BC grpcmocks
// AddExpectedCall add a call to the expected call chain with the given expected args and the values to return
func (m *Mocker) AddExpectedCall(method string, args []any, returns []any) DeletableCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	newCall := newSingleExpectedCall(args, returns)
	m.expectedCalls[method] = append(m.expectedCalls[method], &newCall)

	return DeletableCall{
		method: method,
		call:   newCall,
		mocker: m,
	}
}

// AddExpectedCallV2 add a call to the expected call chain with the given expected args and the values to return
func (m *Mocker) AddExpectedCallV2(method string, args []any, returns []any) *RegisteredCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	newCall := newSingleExpectedCall(args, returns)
	m.expectedCalls[method] = append(m.expectedCalls[method], &newCall)

	return &RegisteredCall{
		method: method,
		call:   &newCall,
		mocker: m,
	}
}

// AddExpectedCallWithFuncV2 add a call to the expected call chain with the given expected args and a function to generate return values
func (m *Mocker) AddExpectedCallWithFuncV2(method string, args []any, returnFunc ReturnFunc) *RegisteredCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	newCall := newSingleExpectedCallWithFunc(args, returnFunc)
	m.expectedCalls[method] = append(m.expectedCalls[method], &newCall)

	return &RegisteredCall{
		method: method,
		call:   &newCall,
		mocker: m,
	}
}

// SetDefaultCall sets a default call for the provided method that will return the provided values
func (m *Mocker) SetDefaultCall(method string, returns []any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := newSingleExpectedCall([]any{}, returns)
	call.setDefault()
	m.defaultCalls[method] = &call
}

// SetDefaultCallWithFunc sets a default call for the provided method that will use a function to generate return values
func (m *Mocker) SetDefaultCallWithFunc(method string, returnFunc ReturnFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := newSingleExpectedCallWithFunc([]any{}, returnFunc)
	call.setDefault()
	m.defaultCalls[method] = &call
}

// Deprecated: For BC grpcmocks
func (m *Mocker) Call(method string, args ...any) ([]any, error) {
	matchedCall, err := m.findMatchingCall(method, args...)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount[method]++
	matchedCall.call()

	return matchedCall.returns, nil
}

// CallV2 try to find a matching call for the given method with the given arguments.
// It will return a struct contains the expected call along with its return values and other information.
// If no call was found, an error will be returned.
func (m *Mocker) CallV2(method string, args ...any) (*SingleExpectedCall, error) {
	m.mu.Lock()
	m.callCount[method]++
	m.mu.Unlock()

	matchedCall, err := m.findMatchingCall(method, args...)
	if err != nil {
		return nil, err
	}

	matchedCall.call()

	return matchedCall, nil
}

// GetCallCount returns how many times a given method was called by the mock
func (m *Mocker) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[method]
}

// ResetAll deletes all the expected calls and default calls of all methods for this mock server.
func (m *Mocker) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount = make(map[string]int)
	m.expectedCalls = make(map[string][]*SingleExpectedCall)
	m.defaultCalls = make(map[string]*SingleExpectedCall)
}

// ResetCall deletes all the expected call and the default call for a specific method
func (m *Mocker) ResetCall(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount[method] = 0
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

// RegisteredCall is used as a wrapper returned by Mocker.AddExpectedCall to allow a plain methods about the added call
// (like Delete(), TimesCalled(), etc..)
type RegisteredCall struct {
	method string
	call   *SingleExpectedCall
	mocker *Mocker
}

// Delete deletes this call from the expected call array.
func (d *RegisteredCall) Delete() {
	d.mocker.DeleteCall(d.method, d.call.id)
}

// TimesCalled returns how many times this specific mock call was called.
// Keep in mind that if you have multiple mock calls or default call added to the same method, there may be more calls
// to that method other than the times called returned from this specific mock call.
// To get the amount of times a method calls from all the added mock calls (including the default call) use
// `<mocker_server>.Configure().<method>.TimesCalled()`
func (d *RegisteredCall) TimesCalled() int {
	return d.call.timesCalled()
}

// Deprecated: For BC grpcmocks
// DeletableCall is used as a wrapper returned by Mocker.AddExpectedCall to allow a plain Delete() method which will
// delete that specific added call.
type DeletableCall struct {
	method string
	call   SingleExpectedCall
	mocker *Mocker
}

// Delete deletes this call from the expected call array.
func (d *DeletableCall) Delete() {
	d.mocker.DeleteCall(d.method, d.call.id)
}
