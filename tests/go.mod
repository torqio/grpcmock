module github.com/torqio/grpcmock/example

go 1.19

require (
	github.com/torqio/grpcmock v0.0.0
	google.golang.org/grpc v1.54.0
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
)

replace github.com/torqio/grpcmock v0.0.0 => ../
