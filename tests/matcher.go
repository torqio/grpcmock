package tests

import (
	"context"
	"fmt"
)

// ContextMatcher is a custom Gomock matcher for context.Context
type ContextMatcher struct {
	expectedValues map[interface{}]interface{}
}

// NewContextMatcher creates a new instance of ContextMatcher with expected values.
func NewContextMatcher(expectedValues map[interface{}]interface{}) *ContextMatcher {
	return &ContextMatcher{
		expectedValues: expectedValues,
	}
}

// Matches checks whether the provided value is a context.Context with matching key-value pairs.
func (m *ContextMatcher) Matches(x interface{}) bool {
	ctx, ok := x.(context.Context)
	if !ok {
		return false // Not a context.Context
	}

	// Verify all expected key-value pairs are present in the context
	for key, expectedValue := range m.expectedValues {
		actualValue := ctx.Value(key)
		if actualValue != expectedValue {
			return false
		}
	}
	return true
}

// String provides a textual representation of the matcher.
func (m *ContextMatcher) String() string {
	return fmt.Sprintf("matches context with values: %v", m.expectedValues)
}
