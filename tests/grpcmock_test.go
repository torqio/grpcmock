package tests

import (
	"context"
	"errors"
	"io"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/torqio/grpcmock/pkg/mocker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"stackpulse.dev/testing/grpctest"
)

// Those tests won't compile unless you run `make test`. This is because they need the test proto to be compiled
// and the `make test` command is make sure of that.
// The reason we are not pre-compile and push it as part of the repo is that we want a fresh compilation of the proto
// as part of the test, because it tests the plugin code generation (which generated as part of the proto compilation).

type exampleReqMatcher struct {
	req string
}

func (e exampleReqMatcher) Matches(x any) bool {
	req, ok := x.(*ExampleMethodRequest)
	if !ok {
		return false
	}

	return req.Req == e.req
}

func TestGRPCMock(t *testing.T) {
	ctx := context.Background()
	testServer := NewExampleServiceMockServerT(t)
	srv := grpctest.NewTestServer(ctx, t, testServer, grpctest.WithoutMiddlewares())

	conn, err := grpc.Dial(srv.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := NewExampleServiceClient(conn)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	defaultRes := "default"
	testServer.Configure().ExampleMethod().DefaultReturn(&ExampleMethodResponse{Res: defaultRes}, nil)

	totalExpectedCalls := int32(0)

	tests := []struct {
		name      string
		callCount int
	}{
		{
			name:      "single call",
			callCount: 1,
		},
		{
			name:      "multiple calls",
			callCount: 10,
		},
	}
	for _, tc := range tests {
		tc := tc
		// Running the tests multiple times in parallel to make sure they work in parallel
		for i := 0; i < 50; i++ {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				currentReq := uuid.NewString()
				call := testServer.Configure().ExampleMethod().On(mocker.Any(), exampleReqMatcher{req: currentReq}).Return(&ExampleMethodResponse{Res: currentReq}, nil)

				for i := 0; i < tc.callCount; i++ {
					res, err := client.ExampleMethod(ctx, &ExampleMethodRequest{Req: currentReq})
					require.NoError(t, err)
					assert.Equal(t, currentReq, res.GetRes())
				}

				assert.Equal(t, tc.callCount, call.TimesCalled())

				call.Delete()

				res, err := client.ExampleMethod(ctx, &ExampleMethodRequest{Req: currentReq})
				require.NoError(t, err)
				assert.Equal(t, defaultRes, res.GetRes())

				atomic.AddInt32(&totalExpectedCalls, int32(tc.callCount+1))
			})
		}
	}
	assert.Equal(t, int(totalExpectedCalls), testServer.Configure().ExampleMethod().TimesCalled())
}

func TestGRPCMockStreamResponse(t *testing.T) {
	ctx := context.Background()
	testServer := NewExampleServiceMockServerT(t)
	srv := grpctest.NewTestServer(ctx, t, testServer, grpctest.WithoutMiddlewares())

	conn, err := grpc.Dial(srv.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := NewExampleServiceClient(conn)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	defaultStreamRes := []*ExampleMethodResponse{{Res: "default"}}
	testServer.Configure().ExampleStreamResponse().DefaultReturn(defaultStreamRes, nil)

	totalExpectedCalls := int32(0)

	tests := []struct {
		name           string
		retStreamCount int
		callCount      int
	}{
		{
			name:           "single call",
			retStreamCount: 2,
			callCount:      1,
		},
		{
			name:           "multiple calls",
			retStreamCount: 10,
			callCount:      10,
		},
	}
	for _, tc := range tests {
		tc := tc
		// Running the tests multiple times in parallel to make sure they work in parallel
		for i := 0; i < 50; i++ {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				currentReq := uuid.NewString()

				retStream := make([]*ExampleMethodResponse, 0, tc.retStreamCount)
				for i := 0; i < tc.retStreamCount; i++ {
					retStream = append(retStream, &ExampleMethodResponse{Res: currentReq + "_" + strconv.Itoa(i)})
				}
				call := testServer.Configure().ExampleStreamResponse().On(&ExampleMethodRequest{Req: currentReq}).Return(retStream, nil)

				for i := 0; i < tc.callCount; i++ {
					stream, err := client.ExampleStreamResponse(context.Background(), &ExampleMethodRequest{Req: currentReq})
					require.NoError(t, err)

					for _, expectedRes := range retStream {
						res, err := stream.Recv()
						require.NoError(t, err)
						assert.Equal(t, expectedRes.GetRes(), res.GetRes())
					}
					_, err = stream.Recv()
					require.Error(t, err)
					assert.True(t, errors.Is(err, io.EOF))
				}

				assert.Equal(t, tc.callCount, call.TimesCalled())

				call.Delete()

				stream, err := client.ExampleStreamResponse(context.Background(), &ExampleMethodRequest{Req: currentReq})
				require.NoError(t, err)
				for _, expectedRes := range defaultStreamRes {
					res, err := stream.Recv()
					require.NoError(t, err)
					assert.Equal(t, expectedRes.GetRes(), res.GetRes())
				}
				_, err = stream.Recv()
				require.Error(t, err)
				assert.True(t, errors.Is(err, io.EOF))

				atomic.AddInt32(&totalExpectedCalls, int32(tc.callCount+1))
			})
		}
	}
	assert.Equal(t, int(totalExpectedCalls), testServer.Configure().ExampleStreamResponse().TimesCalled())
}
