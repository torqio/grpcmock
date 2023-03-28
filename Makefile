TEST_PROTOS_DIR = tests
TEST_PLUGIN_FILE = ${TEST_PROTOS_DIR}/protoc-gen-grpcmock-test

test:
	@echo "-> Compiling protoc plugin"
	@cd ./protoc-gen-grpcmock && go build -o ../${TEST_PLUGIN_FILE} .
	@echo "-> Compiling test protos"
	@cd ${TEST_PROTOS_DIR} && PATH=$(abspath ${TEST_PROTOS_DIR}):${PATH} buf generate
	@echo "-> Running plugin tests"
	@cd ${TEST_PROTOS_DIR} && gotestsum -race --format testname