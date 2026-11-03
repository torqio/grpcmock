package mocker

import (
	"sync"
	"sync/atomic"
	"testing"
)

// The generated mock server calls Returns() more than once per RPC (once to check the
// length, once to read the values), so one invocation must see one doAndReturn result,
// even when other invocations of the same expected call run concurrently.
func TestCallV2WithFuncIsEvaluatedOncePerInvocation(t *testing.T) {
	m := NewMocker()
	var n atomic.Int64
	m.AddExpectedCallWithFuncV2("Method", []any{Any()}, func() []any {
		v := n.Add(1)
		return []any{v, nil}
	})

	const calls = 200
	var wg sync.WaitGroup
	seen := make(chan int64, calls)
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			call, err := m.CallV2("Method", "req")
			if err != nil {
				t.Error(err)
				return
			}
			first := call.Returns()
			second := call.Returns()
			if first[0] != second[0] {
				t.Errorf("one invocation saw two different results: %v then %v", first[0], second[0])
			}
			seen <- second[0].(int64)
		}()
	}
	wg.Wait()
	close(seen)

	unique := map[int64]bool{}
	for v := range seen {
		unique[v] = true
	}
	if got := n.Load(); got != calls {
		t.Errorf("doAndReturn ran %d times for %d calls", got, calls)
	}
	if len(unique) != calls {
		t.Errorf("%d calls got only %d distinct results", calls, len(unique))
	}
	if got := m.GetCallCount("Method"); got != calls {
		t.Errorf("GetCallCount = %d, want %d", got, calls)
	}
}

func TestDefaultCallWithFuncNilResultIsNotReevaluated(t *testing.T) {
	m := NewMocker()
	var n atomic.Int64
	m.SetDefaultCallWithFunc("Method", func() []any {
		n.Add(1)
		return nil
	})
	call, err := m.CallV2("Method")
	if err != nil {
		t.Fatal(err)
	}
	call.Returns()
	call.Returns()
	if got := n.Load(); got != 1 {
		t.Errorf("doAndReturn ran %d times for one call, want 1", got)
	}
}
