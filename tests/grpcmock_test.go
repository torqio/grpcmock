package tests

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"gotest.tools/assert"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/torqio/grpcmock/pkg/mocker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const testSrvAddr = ":8881"

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

func startGrpcServer(t *testing.T, testServer *ExampleServiceMockServer) *grpc.Server {
	lis, err := net.Listen("tcp", testSrvAddr)
	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
		lis.Close()
	})

	go func() {
		require.NoError(t, err)
		RegisterExampleServiceServer(srv, testServer)
		require.NoError(t, srv.Serve(lis))
	}()

	return srv
}

func TestGRPCMock(t *testing.T) {
	ctx := context.Background()
	testServer := NewExampleServiceMockServerT(t)
	_ = startGrpcServer(t, testServer)

	conn, err := grpc.Dial(testSrvAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
