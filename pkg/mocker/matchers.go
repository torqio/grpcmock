package mocker

import (
	"reflect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func Any() Matcher             { return &anyMatcher{} }
func Eq(x interface{}) Matcher { return &eqMatcher{x} }

type anyMatcher struct{}

func (a *anyMatcher) Matches(x interface{}) bool {
	return true
}

type eqMatcher struct {
	x interface{}
}

func (e eqMatcher) Matches(x interface{}) bool {
	if e.x == nil || x == nil {
		return reflect.DeepEqual(e.x, x)
	}

	// If inputs are proto messages, use proto.Equal
	if match, isProto := protoMatches(e.x, x); isProto {
		return match
	}

	// Check if types assignable and convert them to common type
	x1Val := reflect.ValueOf(e.x)
	x2Val := reflect.ValueOf(x)

	if x1Val.Type().AssignableTo(x2Val.Type()) {
		x1Converted := x1Val.Convert(x2Val.Type())
		return reflect.DeepEqual(x1Converted.Interface(), x2Val.Interface())
	}

	return false
}

func protoMatches(a, b interface{}) (isMatching, isProto bool) {
	aProto, ok := a.(protoreflect.ProtoMessage)
	if !ok {
		return isMatching, false
	}

	bProto, ok := b.(protoreflect.ProtoMessage)
	if !ok {
		return isMatching, false
	}

	return proto.Equal(aProto, bProto), true
}
