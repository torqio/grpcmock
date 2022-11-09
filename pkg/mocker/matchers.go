package mocker

import "reflect"

func Any() Matcher             { return &anyMatcher{} }
func Eq(x interface{}) Matcher { return &eqMatcher{x} }

type anyMatcher struct{}

func (a *anyMatcher) Matches(x interface{}) bool {
	return true
}

// eqMatcher implementation took from gomock.Eq as it's small and straightforward, and I wanted to avoid gomock dependency
type eqMatcher struct {
	x interface{}
}

func (e eqMatcher) Matches(x interface{}) bool {
	// In case, some value is nil
	if e.x == nil || x == nil {
		return reflect.DeepEqual(e.x, x)
	}

	// Check if types assignable and convert them to common type
	x1Val := reflect.ValueOf(e.x)
	x2Val := reflect.ValueOf(x)

	if x1Val.Type().AssignableTo(x2Val.Type()) {
		x1ValConverted := x1Val.Convert(x2Val.Type())
		return reflect.DeepEqual(x1ValConverted.Interface(), x2Val.Interface())
	}

	return false
}
