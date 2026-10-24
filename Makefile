.DEFAULT_GOAL := sdk

GENERATOR ?= ./flux
FLUX_REVISION := 7bf0d07b74871baa601b6388b53560ec6f6cbe01
COMMON_INPUT ?= docs/api/common
PAAS_INPUT ?= docs/api/paas
SDK_GO_PATH ?= pkg
PAAS_GO_PATH ?= pkg/http
OAPI_OUTPUT_PATH ?= docs/oapi

.PHONY: sdk sdk-http sdk-oapi sdk-proto sdk-grpc check-flux

check-flux:
	@version="$$($(GENERATOR) version)"; \
	case "$$version" in \
	  "flux v0.4.0 ($(FLUX_REVISION))"|"flux dev ($(FLUX_REVISION))") ;; \
	  *) printf '%s\n' "Expected clean Flux $(FLUX_REVISION); got: $$version" >&2; exit 1 ;; \
	esac

sdk-http: check-flux
	$(GENERATOR) generate --target go --in $(COMMON_INPUT) --go-generated-path $(SDK_GO_PATH)
	$(GENERATOR) generate --target go --in $(PAAS_INPUT) --go-generated-path $(PAAS_GO_PATH)

sdk-oapi: check-flux
	$(GENERATOR) generate --target oapi --in $(COMMON_INPUT) --oapi-generated-path $(OAPI_OUTPUT_PATH)
	$(GENERATOR) generate --target oapi --in $(PAAS_INPUT) --oapi-generated-path $(OAPI_OUTPUT_PATH)

sdk-proto sdk-grpc:
	@printf '%s\n' 'proto/gRPC generation is deferred; existing docs/proto and pkg/grpc are preserved.' >&2
	@exit 1

sdk: sdk-http sdk-oapi
