module github.com/torqio/grpcmock/tests

go 1.22

require (
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.8.4
	github.com/torqio/grpcmock v0.0.0
	google.golang.org/grpc v1.68.1
	google.golang.org/protobuf v1.35.2
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/torqio/grpcmock v0.0.0 => ../
