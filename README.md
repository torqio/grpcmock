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