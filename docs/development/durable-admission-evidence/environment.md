# Среда и команды проверки

```text
Evidence captured 2026-11-05
GOTOOLCHAIN=go1.27.3: go version go1.27.3 linux/amd64
golangci-lint 2.15.0 built with go1.27.3
OpenSpec 1.16.0
PostgreSQL 18.6 (see ctl-stage-v4/daemon-uds-narrow-grants.log)
Pinned image postgres:18.6-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873
Full and final admission tests: persistent local fixture container paas-admission-tests; PAAS_TEST_POSTGRES_URL supplied separately, credentials omitted.
Daemon cwd commands after sourcing /workspace/.paas-env/activate and exporting GOTOOLCHAIN=go1.27.3 and GOLANGCI_LINT_CACHE=/workspace/.paas-env/lint-cache-v2.15:
Full (before final validator tightening): go test -count=1 -race -v ./...
Final focused: go test -count=1 -race -v ./internal/store -run 'TestAdmission|TestPostgresAdmission'
```
