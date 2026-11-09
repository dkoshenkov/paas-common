.DEFAULT_GOAL := sdk

GENERATOR ?= ./flux
FLUX_VERSION := v0.7.0
FLUX_REVISION := 791064940350c2561c1c98d3a559e64e90e23d43
FLUX_MODULE_SUM := h1:fGQepNVU0lxM+NYAja8MyKs9DyfnN1kO9O2Qr/Kvy+8=
COMMON_INPUT ?= docs/api/common
PAAS_INPUT ?= docs/api/paas
GITHUB_INPUT ?= docs/api/github
SDK_GO_PATH ?= pkg
PAAS_GO_PATH ?= pkg/http
OAPI_OUTPUT_PATH ?= docs/oapi

.PHONY: sdk sdk-http sdk-oapi sdk-proto sdk-grpc check-flux

check-flux:
	@binary="$$(command -v "$(GENERATOR)")" && \
	go version -m "$$binary" | awk -v release='$(FLUX_VERSION)' -v rev='$(FLUX_REVISION)' -v sum='$(FLUX_MODULE_SUM)' '\
	  $$1 == "path" && $$2 == "github.com/dkoshenkov/flux/cmd" {program = 1} \
	  $$1 == "mod" && $$2 == "github.com/dkoshenkov/flux" {module = 1; published = ($$3 == release && $$4 == sum)} \
	  $$1 == "build" && $$2 == "vcs.revision=" rev {revision = 1} \
	  $$1 == "build" && $$2 == "vcs.modified=false" {clean = 1} \
	  $$1 == "=>" {replaced = 1} \
	  END {exit !(program && module && !replaced && (published || (revision && clean)))}' || \
	  { echo "Expected verified Flux $(FLUX_VERSION) module or clean revision $(FLUX_REVISION)" >&2; exit 1; }

sdk-http: check-flux
	"$(GENERATOR)" generate --target go --in $(COMMON_INPUT) --go-generated-path $(SDK_GO_PATH)
	"$(GENERATOR)" generate --target go --in $(PAAS_INPUT) --go-generated-path $(PAAS_GO_PATH)
	"$(GENERATOR)" generate --target go --in $(GITHUB_INPUT) --go-generated-path $(PAAS_GO_PATH)
	rm -f $(PAAS_GO_PATH)/github/server/github/server_gen.go
	rmdir $(PAAS_GO_PATH)/github/server/github $(PAAS_GO_PATH)/github/server 2>/dev/null || true

sdk-oapi: check-flux
	"$(GENERATOR)" generate --target oapi --in $(COMMON_INPUT) --oapi-generated-path $(OAPI_OUTPUT_PATH)
	"$(GENERATOR)" generate --target oapi --in $(PAAS_INPUT) --oapi-generated-path $(OAPI_OUTPUT_PATH)

sdk-proto sdk-grpc:
	@printf '%s\n' 'proto/gRPC generation is deferred; existing docs/proto and pkg/grpc are preserved.' >&2
	@exit 1

sdk: sdk-http sdk-oapi
