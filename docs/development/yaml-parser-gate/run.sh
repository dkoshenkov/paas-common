#!/bin/sh
set -eu
cd "$(dirname "$0")"
export GOTOOLCHAIN=go1.27.3
go version
go mod verify
go test -race -count=1 -v -run 'TestDepthCounterexamples|TestAliasesAndMerge|TestDocumentBudget|TestNoNodeBudget|TestHTTPWireLimit' ./...
# Required contract intentionally fails: a successful reproduction is NOT acceptance.
go test -count=1 -v -run '^TestRequiredEarlyDepth64$' ./...
