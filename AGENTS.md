# Repository Guidelines

## Project Structure & Module Organization

This repository contains PaaS contracts, generated SDK artifacts, OpenSpec, and shared project documentation. It is part of a workspace with three independent Git repositories; the workspace root is not a Git repository.

- This repository: Go module for HTTP and gRPC contracts. Edit source specifications in `docs/api/common/` and `docs/api/paas/`. Generated artifacts live in `pkg/http/`, `pkg/grpc/`, `docs/proto/`, and `docs/oapi/`.
- Sibling repositories `../paas-ctl/` and `../paas-daemon/`: currently template scaffolds with workflow definitions and initialization scripts; application code and Go modules are not present yet.
- Each repository has `.github/` automation and `.golangci.yml` configuration.

## Build, Test, and Development Commands

Run these commands from this repository root, using Go 1.26.5 or newer:

- `go build ./...`: compile all packages.
- `go test -count=1 ./...`: run tests without cached results.
- `go test -race ./...`: check tests for data races.
- `go vet ./...`: run static checks.
- `go mod tidy -diff`: detect dependency-file drift without modifying files.
- `golangci-lint fmt` and `golangci-lint run`: format handwritten code and run configured linters.
- `make sdk`: regenerate common HTTP, protobuf, gRPC, and OpenAPI artifacts. Individual targets include `sdk-http`, `sdk-grpc`, and `sdk-oapi`.

Generation requires Flux, `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc`. The default `./flux` binary is currently absent; supply it or use `make sdk GENERATOR=/path/to/flux`. There is no application run command yet.

## Coding Style & Naming Conventions

Use idiomatic Go names, tabs for Go indentation, and two spaces in YAML. Configured formatters are `gofumpt` and `gci`. Never manually edit generated files, including `*_gen.go` and `*.pb.go`; update source contracts and regenerate. Current Makefile defaults generate common contracts only.

## Testing Guidelines

No `*_test.go` files or coverage threshold currently exist. Use Go's standard `testing` package, colocate `*_test.go` files with code, and name tests `TestBehavior`. Add focused checks for new handwritten behavior. Verify regenerated artifacts with build, test, and lint commands.

## Commit & Pull Request Guidelines

SDK history uses Conventional Commits, such as `chore(deps): bump ...`; follow `type(scope): description`. Keep PRs focused, explain behavior or contract changes, link relevant issues, and report validation results and generation prerequisites.

## Product Version Boundaries

- Ветка продукта PaaS `0.x.x` поддерживает только одну ноду k3s.
- Мультинод — обязательное условие выпуска продукта PaaS `1.0.0` и развития `1.x.x`.
- Не выпускать `1.0.0` только по наличию breaking change, пока мультинод не реализован и не проверен.
- Мультинод не означает автоматически HA: требования отказоустойчивости согласовываются отдельно.

## Automatic Versioning Policy

- Во всех трёх репозиториях (`paas-common`, `paas-ctl`, `paas-daemon`) требуется автоматический расчёт следующей версии релиза по сообщениям коммитов.
- Правило проекта только для `0.x.x`: по умолчанию **minor**; `fix:` / `fix(scope):` — **patch**; breaking change (`!` после type/scope либо footer `BREAKING CHANGE:`) — **minor**, включая `fix!:`. Например, `0.3.2` → `0.4.0`.
- Переход на `1.0.0` — отдельное явно согласованное решение после реализации и проверки поддержки мультинода; его нельзя выпускать автоматически из-за breaking commit.
- Начиная с `1.0.0`, API проектировать с обратной совместимостью. Изменения API не должны требовать breaking bump в обычном развитии `1.x.x`.
- При нескольких изменениях выбирается максимальный требуемый bump: major > minor > patch. Если есть обычное изменение без patch/major-маркера, оно требует minor.
- Используй Conventional Commits. Не предполагай, что стандартная стратегия Release Please реализует minor по умолчанию: это нужно явно настроить и проверить.
- Служебные release-коммиты не должны запускать бесконечную цепочку новых релизов.
- Проверка локального workflow не доказывает успешный запуск GitHub Actions: отдельно проверяются caller, конфигурация, доступность используемой версии action, необходимые secrets и результат CI.
- Автоматизация специального правила breaking bump относится только к `0.x.x`; не переносить его на `1.x.x`.
- Наличие автоматизации не даёт агенту разрешения на commit, push, создание PR/MR, merge или публикацию релиза.

## Agent-Specific Instructions

Git access is read-only without user permission. Obtain separate explicit approval for each commit, push, and PR/MR creation. All architectural decisions require user approval before implementation. Follow the `ponytail` skill: prefer existing patterns, standard libraries, and the smallest sufficient change.

## Planning Files

- `openspec/`: central OpenSpec project for paas-common, paas-ctl, and paas-daemon. Run OpenSpec commands from this repository root.
- `docs/backlog/`: separate PaaS, Flux, and packages-go backlogs.

## OpenSpec Workflow

Для разработки изменений проекта обязателен расширенный цикл OpenSpec. До реализации необходимо изучить актуальные спецификации и связанные changes, оформить новый change или актуализировать существующий и получить явное одобрение пользователя на архитектуру и текущий этап. Нельзя пропускать OpenSpec и сразу переходить к реализации.

Обязательный порядок работы:

1. `$openspec-explore` — изучить текущий проект, обсудить требования и варианты решения.
2. `$openspec-propose` — оформить change: `proposal.md`, спецификации, `design.md` и `tasks.md` по схеме проекта. Явно указать затронутые репозитории и границы изменения.
3. Представить архитектуру пользователю и получить явное одобрение до реализации. Предложенные решения не считать согласованными.
4. `$openspec-apply-change` — реализовывать только согласованный этап небольшими проверяемыми шагами. После этапа сообщить результат проверок и дождаться разрешения на следующий этап.
5. `$openspec-update-change` — при изменении требований или плана согласованно обновить документы change; новые архитектурные решения повторно согласовать до реализации.
6. `$openspec-sync-specs` — при необходимости перенести требования change в основные спецификации без архивирования; `$openspec-archive-change` — завершить и архивировать change после выполнения задач и проверок.

Подготовка документов и реализация — отдельные этапы: после оформления change остановиться для рассмотрения пользователем. Вызовы навыков можно задавать явно или обычным текстом. Документы писать на русском, сохраняя структурные заголовки OpenSpec и ключевые слова SHALL/MUST на английском. Этот процесс не заменяет отдельные разрешения на commit, push и PR/MR.
