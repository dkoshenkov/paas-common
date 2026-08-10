#!/usr/bin/env bash

set -euo pipefail

mode="local"
automation_repo=""
ref="main"
force=false
target_dir="."

usage() {
  cat <<'EOF'
Usage:
  bash scripts/init-workflows.sh [options]

Options:
  --mode local|central             Workflow reference mode. Default: local.
  --automation-repo OWNER/REPO     Central repository for --mode central.
  --ref REF                        Branch or tag for --mode central. Default: main.
  --target-dir PATH                Repository to initialize. Default: current directory.
  --force                          Overwrite generated files.
  -h, --help                       Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)
      mode="${2:?missing value for --mode}"
      shift 2
      ;;
    --automation-repo)
      automation_repo="${2:?missing value for --automation-repo}"
      shift 2
      ;;
    --ref)
      ref="${2:?missing value for --ref}"
      shift 2
      ;;
    --target-dir)
      target_dir="${2:?missing value for --target-dir}"
      shift 2
      ;;
    --force)
      force=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown option: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$mode" != "local" && "$mode" != "central" ]]; then
  printf '%s\n' '--mode must be local or central' >&2
  exit 2
fi

if [[ "$mode" == "central" && ! "$automation_repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  printf '%s\n' '--automation-repo must have the form OWNER/REPO in central mode' >&2
  exit 2
fi

if [[ ! "$ref" =~ ^[A-Za-z0-9._/-]+$ ]]; then
  printf '%s\n' '--ref contains unsupported characters' >&2
  exit 2
fi

cd "$target_dir"

if [[ "$mode" == "local" ]]; then
  go_quality_workflow='./.github/workflows/go-quality-reusable.yml'
  workflow_lint_workflow='./.github/workflows/workflow-lint-reusable.yml'
  dependency_review_workflow='./.github/workflows/dependency-review-reusable.yml'
  codeql_workflow='./.github/workflows/codeql-reusable.yml'
  release_workflow='./.github/workflows/release-reusable.yml'
else
  go_quality_workflow="${automation_repo}/.github/workflows/go-quality-reusable.yml@${ref}"
  workflow_lint_workflow="${automation_repo}/.github/workflows/workflow-lint-reusable.yml@${ref}"
  dependency_review_workflow="${automation_repo}/.github/workflows/dependency-review-reusable.yml@${ref}"
  codeql_workflow="${automation_repo}/.github/workflows/codeql-reusable.yml@${ref}"
  release_workflow="${automation_repo}/.github/workflows/release-reusable.yml@${ref}"
fi

render() {
  local destination="$1"
  local content

  if [[ -e "$destination" && "$force" != true ]]; then
    printf 'refusing to overwrite %s; use --force\n' "$destination" >&2
    exit 1
  fi

  mkdir -p "$(dirname "$destination")"
  content="$(cat)"
  content="${content//__GO_QUALITY_WORKFLOW__/$go_quality_workflow}"
  content="${content//__WORKFLOW_LINT_WORKFLOW__/$workflow_lint_workflow}"
  content="${content//__DEPENDENCY_REVIEW_WORKFLOW__/$dependency_review_workflow}"
  content="${content//__CODEQL_WORKFLOW__/$codeql_workflow}"
  content="${content//__RELEASE_WORKFLOW__/$release_workflow}"
  printf '%s\n' "$content" > "$destination"
}

render .github/workflows/go-quality.yml <<'EOF'
name: go-quality

on:
  pull_request:
  push:
    branches:
      - main

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  go-quality:
    uses: __GO_QUALITY_WORKFLOW__
EOF

render .github/workflows/workflow-lint.yml <<'EOF'
name: workflow-lint

on:
  pull_request:
  push:
    branches:
      - main

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  workflow-lint:
    uses: __WORKFLOW_LINT_WORKFLOW__
EOF

render .github/workflows/dependency-review.yml <<'EOF'
name: dependency-review

on:
  pull_request:

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  dependency-review:
    uses: __DEPENDENCY_REVIEW_WORKFLOW__
EOF

render .github/workflows/codeql.yml <<'EOF'
name: codeql

on:
  pull_request:
  push:
    branches:
      - main
  schedule:
    - cron: '30 1 * * 0'

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  actions: read
  contents: read
  packages: read
  security-events: write

jobs:
  codeql:
    uses: __CODEQL_WORKFLOW__
EOF

render .github/workflows/release.yml <<'EOF'
name: release

on:
  push:
    branches:
      - main

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false

permissions:
  contents: write
  issues: write
  pull-requests: write

jobs:
  release-please:
    uses: __RELEASE_WORKFLOW__
    secrets:
      token: ${{ secrets.RELEASE_PLEASE_TOKEN }}
EOF

render configs/release-please-config.json <<'EOF'
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/release-please-config.json",
  "packages": {
    ".": {
      "release-type": "go"
    }
  }
}
EOF

render configs/.release-please-manifest.json <<'EOF'
{
  ".": "0.1.0"
}
EOF

printf 'workflow configuration initialized in %s (%s mode)\n' "$PWD" "$mode"
