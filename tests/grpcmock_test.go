package tests

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/torqio/grpcmock/pkg/mocker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Those tests won't compile unless you run `make test`. This is because they need the test proto to be compiled
// and the `make test` command is make sure of that.
// The reason we are not pre-compile and push it as part of the repo is that we want a fresh compilation of the proto
// as part of the test, because it tests the plugin code generation (which generated as part of the proto compilation).

func startGrpcServer(t *testing.T, testServer *ExampleServiceMockServer) string {
	lis, err := net.Listen("tcp", ":0")
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

	return lis.Addr().String()
}

func TestGRPCMockUnary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testServer, err := NewExampleServiceMockServer()
	require.NoError(t, err)
	addr := startGrpcServer(t, testServer)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
				call := testServer.Configure().ExampleMethod().On(mocker.Any(), &ExampleMethodRequest{Req: currentReq}).Return(&ExampleMethodResponse{Res: currentReq}, nil)

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

	t.Cleanup(func() {
		assert.Equal(t, int(totalExpectedCalls), testServer.Configure().ExampleMethod().TimesCalled())

		// Remove all and call again, expect error
		testServer.Configure().ExampleMethod().Reset()
		_, err = client.ExampleMethod(ctx, &ExampleMethodRequest{Req: uuid.NewString()})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no matching expected call nor default return for method ExampleMethod with given arguments")
	})
}

// Testing the case where the server is sending a stream of responses and the client is sending a single request.
// The test is checking that the server is returning the expected responses stream for each request is receives.
// If there is no expected request matched, the server will return the default responses stream.
// If there is no default responses stream, the server will return an error.
func TestGRPCMockStreamResponse(t *testing.T) {
	t.Parallel()

	testServer, err := NewExampleServiceMockServer()
	require.NoError(t, err)
	addr := startGrpcServer(t, testServer)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

				ctxKey := uuid.NewString()
				ctx := context.WithValue(context.Background(), "key", ctxKey)
				call := testServer.Configure().ExampleStreamResponse().
					On(&ExampleMethodRequest{Req: currentReq},
						NewContextMatcher(map[interface{}]interface{}{"key": ctxKey})).
					Return(retStream, nil)

				for i := 0; i < tc.callCount; i++ {
					stream, err := client.ExampleStreamResponse(ctx, &ExampleMethodRequest{Req: currentReq})
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
	t.Cleanup(func() {
		assert.Equal(t, int(totalExpectedCalls), testServer.Configure().ExampleStreamResponse().TimesCalled())

		// Remove all and call again, expect error
		testServer.Configure().ExampleStreamResponse().Reset()
		stream, err := client.ExampleStreamResponse(context.Background(), &ExampleMethodRequest{Req: uuid.NewString()})
		require.NoError(t, err)
		for {
			_, err = stream.Recv()
			if err != nil {
				break
			}
		}
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no matching expected call nor default return for method ExampleStreamResponse with given arguments")
	})
}

// Testing the case where the client is sending a stream of requests and the server is sending a single response.
// The test is checking that, given a stream of requests, the server is returning the expected response if it gets an expected request as part of the requests stream.
// If there is no expected request matched, the server will return the default response.
// If there is no default response, the server will return an error.
func TestGRPCMockStreamRequest(t *testing.T) {
	t.Parallel()

	testServer, err := NewExampleServiceMockServer()
	require.NoError(t, err)
	addr := startGrpcServer(t, testServer)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := NewExampleServiceClient(conn)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	defaultRes := "default"
	testServer.Configure().ExampleStreamRequest().DefaultReturn(&ExampleMethodResponse{Res: defaultRes}, nil)

	totalExpectedCalls := int32(0)

	tests := []struct {
		name           string
		reqStreamCount int
		matchOnReq     int
		callCount      int
	}{
		{
			name:           "single call",
			reqStreamCount: 2,
			matchOnReq:     0,
			callCount:      1,
		},
		{
			name:           "multiple calls",
			reqStreamCount: 10,
			matchOnReq:     5,
			callCount:      10,
		},
	}
	for _, tc := range tests {
		tc := tc
		// Running the tests multiple times in parallel to make sure they work in parallel
		for i := 0; i < 1; i++ {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				currentReq := uuid.NewString()
				ctxKey := uuid.NewString()
				ctx := context.WithValue(context.Background(), "key", ctxKey)

				reqStream := make([]*ExampleMethodRequest, 0, tc.reqStreamCount)
				for i := 0; i < tc.reqStreamCount; i++ {
					reqStream = append(reqStream, &ExampleMethodRequest{Req: currentReq + "_" + strconv.Itoa(i)})
				}
				call := testServer.Configure().ExampleStreamRequest().
					On(&ExampleMethodRequest{Req: currentReq + "_" + strconv.Itoa(tc.matchOnReq)},
						NewContextMatcher(map[interface{}]interface{}{"key": ctxKey})).
					Return(&ExampleMethodResponse{Res: currentReq}, nil)

				for i := 0; i < tc.callCount; i++ {
					stream, err := client.ExampleStreamRequest(ctx)
					require.NoError(t, err)

					for _, req := range reqStream {
						err := stream.Send(req)
						require.NoError(t, err)
					}
					res, err := stream.CloseAndRecv()
					require.NoError(t, err)
					assert.Equal(t, currentReq, res.GetRes())
				}

				assert.Equal(t, tc.callCount, call.TimesCalled())

				call.Delete()

				stream, err := client.ExampleStreamRequest(context.Background())
				require.NoError(t, err)
				for _, req := range reqStream {
					err := stream.Send(req)
					require.NoError(t, err)
				}
				res, err := stream.CloseAndRecv()
				require.NoError(t, err)
				assert.Equal(t, defaultRes, res.GetRes())

				// The expected calls are the amount of calls for the streaming request * the expected match request on the stream
				// (because after a successful matching, the receiving on the server will end) + the amount of streaming messages
				// because we call it again with expecting the default, and the default will receive all the messages before returning the default.
				expectedCallCount := tc.callCount*(tc.matchOnReq+1) + tc.reqStreamCount
				atomic.AddInt32(&totalExpectedCalls, int32(expectedCallCount))
			})
		}
	}
	t.Cleanup(func() {
		assert.Equal(t, int(totalExpectedCalls), testServer.Configure().ExampleStreamRequest().TimesCalled())

		// Remove all and call again, expect error
		testServer.Configure().ExampleStreamRequest().Reset()
		stream, err := client.ExampleStreamRequest(context.Background())
		require.NoError(t, err)
		err = stream.Send(&ExampleMethodRequest{Req: uuid.NewString()})
		require.NoError(t, err)
		_, err = stream.CloseAndRecv()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no matching expected call nor default return for method ExampleStreamRequest with given arguments")
	})
}

// Testing the case where the server is sending a stream of responses and the client is sending a stream of requests.
// The test is checking that the server is returning the expected responses stream for each request is receives in the stream.
// If there is no expected request matched for a given request in the stream, the server will return the default responses stream.
// If there is no default responses stream and there was no a single match in the whole stream of requests, the server will return an error.
func TestGRPCMockStreamRequestResponse(t *testing.T) {
	t.Parallel()

	testServer, err := NewExampleServiceMockServer()
	require.NoError(t, err)
	addr := startGrpcServer(t, testServer)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := NewExampleServiceClient(conn)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	defaultStreamRes := []*ExampleMethodResponse{{Res: "default"}}
	testServer.Configure().ExampleStreamRequestResponse().DefaultReturn(defaultStreamRes, nil)

	totalExpectedCalls := int32(0)

	tests := []struct {
		name           string
		reqStreamCount int
		matchOnReq     int
		retStreamCount int
		callCount      int
	}{
		{
			name:           "single call",
			reqStreamCount: 2,
			matchOnReq:     0,
			retStreamCount: 2,
			callCount:      1,
		},
		{
			name:           "multiple calls",
			reqStreamCount: 10,
			matchOnReq:     5,
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

				reqStream := make([]*ExampleMethodRequest, 0, tc.reqStreamCount)
				for i := 0; i < tc.reqStreamCount; i++ {
					reqStream = append(reqStream, &ExampleMethodRequest{Req: currentReq + "_" + strconv.Itoa(i)})
				}
				retStream := make([]*ExampleMethodResponse, 0, tc.retStreamCount)
				for i := 0; i < tc.retStreamCount; i++ {
					retStream = append(retStream, &ExampleMethodResponse{Res: currentReq + "_" + strconv.Itoa(i)})
				}
				ctxKey := uuid.NewString()
				ctx := context.WithValue(context.Background(), "key", ctxKey)
				call := testServer.Configure().ExampleStreamRequestResponse().On(
					&ExampleMethodRequest{Req: currentReq + "_" + strconv.Itoa(tc.matchOnReq)},
					NewContextMatcher(map[interface{}]interface{}{"key": ctxKey})).
					Return(retStream, nil)

				for i := 0; i < tc.callCount; i++ {
					stream, err := client.ExampleStreamRequestResponse(ctx)
					require.NoError(t, err)

					for j, req := range reqStream {
						// For each request, we expect a stream of responses (either the retStream or the default, only one of the requests should match the retStream, the rest will return the default)
						err := stream.Send(req)
						require.NoError(t, err)

						expctedRetStream := defaultStreamRes
						if j == tc.matchOnReq {
							expctedRetStream = retStream
						}
						for _, expectedRes := range expctedRetStream {
							res, err := stream.Recv()
							require.NoError(t, err)
							assert.Equal(t, expectedRes.GetRes(), res.GetRes())
						}
					}
					err = stream.CloseSend()
					time.Sleep(10 * time.Millisecond) // Allow the server to receive and send the EOF
					require.NoError(t, err)
					_, err = stream.Recv()
					require.Error(t, err)
					assert.True(t, errors.Is(err, io.EOF))
				}

				assert.Equal(t, tc.callCount, call.TimesCalled())

				call.Delete()

				expectedCallCount := tc.callCount * tc.reqStreamCount
				atomic.AddInt32(&totalExpectedCalls, int32(expectedCallCount))

			})
		}
	}
	t.Cleanup(func() {
		assert.Equal(t, int(totalExpectedCalls), testServer.Configure().ExampleStreamRequestResponse().TimesCalled())

		// Remove all and call again, expect error
		testServer.Configure().ExampleStreamRequestResponse().Reset()
		stream, err := client.ExampleStreamRequestResponse(context.Background())
		require.NoError(t, err)
		err = stream.Send(&ExampleMethodRequest{Req: uuid.NewString()})
		require.NoError(t, err)
		_, err = stream.Recv()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no matching expected call nor default return for method ExampleStreamRequestResponse with given arguments")
	})
}
