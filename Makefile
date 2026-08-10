

GENERATOR ?= ./flux
COMMON_INPUT ?= docs/api/common
SDK_GO_PATH ?= pkg/http
PROTO_PATH ?= docs/proto
COMMON_PROTO_OUTPUT ?= $(PROTO_PATH)
OAPI_OUTPUT_PATH ?= docs/oapi
COMMON_GRPC_OUTPUT ?= pkg/grpc
COMMON_OAPI_OUTPUT ?= docs/oapi
PROTOC ?= protoc
GO_BIN ?= $(shell GOTOOLCHAIN=local go env GOPATH)/bin
PROTOC_GEN_GO ?= $(GO_BIN)/protoc-gen-go
PROTOC_GEN_GO_GRPC ?= $(GO_BIN)/protoc-gen-go-grpc
GO_MODULE ?= $(shell awk '$$1 == "module" { print $$2; exit }' go.mod)
PROTOC_GO_PACKAGE_OPTIONS ?= --go_opt=Mhealth.proto=$(GO_MODULE)/pkg/grpc/health --go_opt=Mready.proto=$(GO_MODULE)/pkg/grpc/ready
PROTOC_GRPC_PACKAGE_OPTIONS ?= --go-grpc_opt=Mhealth.proto=$(GO_MODULE)/pkg/grpc/health --go-grpc_opt=Mready.proto=$(GO_MODULE)/pkg/grpc/ready

.PHONY: sdk sdk-http sdk-proto sdk-grpc sdk-oapi

sdk-http:
	$(GENERATOR) generate --target go --in $(COMMON_INPUT)  --go-generated-path $(SDK_GO_PATH)

sdk-proto:
	$(GENERATOR) generate --target proto --in $(COMMON_INPUT) --proto-generated-path $(PROTO_PATH)

sdk-grpc: sdk-proto
	mkdir -p $(COMMON_GRPC_OUTPUT)
	$(PROTOC) --proto_path=$(PROTO_PATH) --plugin=protoc-gen-go=$(PROTOC_GEN_GO) --plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) --go_out=. --go_opt=paths=import --go_opt=module=$(GO_MODULE) $(PROTOC_GO_PACKAGE_OPTIONS) --go-grpc_out=. --go-grpc_opt=paths=import --go-grpc_opt=module=$(GO_MODULE) $(PROTOC_GRPC_PACKAGE_OPTIONS) $(COMMON_PROTO_OUTPUT)/*.proto

sdk-oapi:
	$(GENERATOR) generate --target oapi --in $(COMMON_INPUT)  --oapi-generated-path $(OAPI_OUTPUT_PATH)


sdk: sdk-http sdk-grpc sdk-oapi
