package mocker

type OptionFunc func(expectedCall *singleExpectedCall)

func WithExpectedTimesCalled(expectedTimesCalled int) OptionFunc {
	return func(expectedCall *singleExpectedCall) {
		expectedCall.expectedCalls = expectedTimesCalled
	}
}