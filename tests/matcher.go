package tests

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
	ctx, ok := x.(grpc.ServerStream)
	if !ok {
		return false // Not a context.Context
	}
	md, _ := metadata.FromIncomingContext(ctx.Context())

	// Verify all expected key-value pairs are present in the context
	for key, expectedValue := range m.expectedValues {
		actualValue := md.Get(key.(string))
		if len(actualValue) == 0 {
			return false
		}
		if actualValue[0] != expectedValue {
			return false
		}
	}
	return true
}
