# gRPCMock

gRPCMock is a protobuf plugin that allows you to create implementation
of a mock server for each RPC service defined in your protobuf file.

## Usage
### Generate plugin code
First, install the plugin

```bash
go install github.com/torqio/grpcmock/protoc-gen-grpcmock
```
Make sure you can execute `protoc-gen-grpcmock` (go binaries should be in the `PATH` environment variable)
Next, add the plugin to your compilation tool.

If you use `protoc`:
```bash
protoc -I/path/to/your/proto --go_out=paths=source_relative:/path/to/your/proto --go-grpc_out=paths=source_relative,require_unimplemented_servers=false:/path/to/your/proto --grpcmock_out=paths=source_relative:/path/to/your/proto /path/to/your/proto/test.proto 
```

If you use `buf`:
<pre>
<code class="yaml">
version: v1
plugins:
  - plugin: buf.build/protocolbuffers/go
    out: .
    opt:
      - paths=source_relative
  - plugin: buf.build/grpc/go
    out: .
    opt:
      - paths=source_relative
      - require_unimplemented_servers=false
  - <b>plugin: grpcmock
    out: .
    opt:
      - paths=source_relative</b>
</code>
</pre>

Now, after the compilation you'll see `<proto_file_name>_grpcmock.pb.go` file for each protobuf file with gRPC.<br/>
Each file

### Use the mock server
#### Init server in tests
For the sake of the example, let's assume we have a protobuf file `svc.proto` with the following content:
(you can find the file in the `tests` directory)
```proto
syntax = "proto3";
package grpcmock.example;
option go_package = "github.com/torqio/grpcmock/tests";

service ExampleService {
  rpc ExampleMethod(ExampleMethodRequest) returns (ExampleMethodResponse);
}

message ExampleMethodRequest {
  string req = 1;
}
message ExampleMethodResponse {
  string res = 1;
}
```
Compile the proto using `buf` <br/>
After the compilation, you'll see a new file `svc_grpcmock.pb.go`.

In your test file, first create a new mock server instance:
```go
testServer := NewExampleServiceMockServerT(t)
```
Then, create a normal gRPC server and register the mock server as a handler (you can assign
middlewares to the server as well):
```go
lis, err := net.Listen("tcp", testSrvAddr)
require.NoError(t, err)
srv := grpc.NewServer()
go func() {
	// Register the mock server as a handler
    RegisterExampleServiceServer(srv, testServer) 
    // Start the server
    require.NoError(t, srv.Serve(lis))
}()
```
<br/>

#### Configure mock server return values
Now, you can use the mock server to define the behavior of the server:
```go
// Define the default return value for an RPC
testServer.Configure().ExampleMethod().DefaultReturn(&ExampleMethodResponse{Res: "default-response"}, nil)
// Define the return value for an RPC with a specific request
testServer.Configure().ExampleMethod().On(mocker.Any(), &ExampleMethodRequest{Req: "some-request-that-should-be-matched"}).Return(&ExampleMethodResponse{Res: "response-that-will-be-returned-if-request-matched"}, nil)
```

As you can see in the code block above, you have 2 main functions to define the behavior of the mock server:
1. DefaultReturn - defines the default return value for an RPC. If the request doesn't match any of the defined
   return values, the default return value will be returned.
2. On+Return - defines the return value for an RPC with a specific request. If the RPC request matches the defined request,
   the defined return value will be returned.<br/>The parameters given to the `On` function can implement [mocker.Matcher](pkg/mocker/mocker.go) (line #9) interface.
   If the parameter doesn't implement the `Matcher` interface, the `eqMatcher` will be used by default.<br/>
   The `eqMatcher` is a matcher that checks if the request is equal to the given parameter using `reflect.DeepEqual` or by using `proto.Equal` in case the 2 compared objects are protobuf messages.
