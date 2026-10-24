# Обновление Flux и Go runtime — 2026-10-24

Версии обновлены в локальных рабочих деревьях и проверены на Go 1.27.3.
Полный набор проверок пройден. Пользователь повторно запустил ограниченные средой
UDS/SSH, PostgreSQL и govulncheck проверки у себя и прислал успешный вывод.

## Изменения

- `paas-common`, `paas-daemon`, `paas-ctl`: Go 1.27.3 в `go.mod` и README.
  CI использует `go-version-file`; golangci-lint закреплён на v2.15.0
  в активном common workflow и трёх reusable workflows.
- `paas-common` и `paas-daemon`: packages-go
  `v0.3.1-0.20261024142719-ec269923c64b`, merged commit
  [PR #13](https://github.com/dkoshenkov/packages-go/pull/13).
  Исправление читает фактический JSON body независимо от ContentLength.
  Релизного тега с исправлением на момент обновления нет.
- `paas-common/Makefile`: Flux v0.4.0, revision
  `7bf0d07b74871baa601b6388b53560ec6f6cbe01`,
  [PR #10](https://github.com/dkoshenkov/flux/pull/10).
- `paas-common`: x/net v0.60.0, x/sys v0.48.0, x/text v0.42.0 — версии,
  требуемые обновлённым packages-go. Checksums взяты из его go.sum.
  Уже более новые версии этих модулей в paas-daemon сохранены.
- Checksums packages-go вычислены `golang.org/x/mod/zip` и `dirhash` по
  исходникам tree `4ed29d46eed056e77c4303511169b5cbc41adb65`, совпадающим с
  merged commit. Контрольное вычисление для v0.3.0 совпало с опубликованным
  checksum. После загрузки зависимости `go mod verify` прошёл во всех трёх
  модулях; `go mod tidy -diff` не показывает изменений.

Активные entrypoints `docs/api/common/api.yaml` и `docs/api/paas/api.yaml`
сохраняют прежний набор routes. Legacy apps/logs с Compose-полями и path
placeholders не импортируются ими и не менялись. Новый GitHub-клиент
добавляется в отдельном согласованном этапе.

## Выполненные проверки

Проверено на Go 1.27.3 darwin/arm64, golangci-lint v2.15.0.

| Проверка | paas-common | paas-daemon | paas-ctl |
| --- | --- | --- | --- |
| go mod tidy -diff, go mod verify | PASS | PASS | PASS |
| go build, go vet | PASS | PASS | PASS |
| formatter diff, lint | PASS, 0 issues | PASS, 0 issues | PASS, 0 issues |
| Полный go test -count=1 | PASS | PASS с PostgreSQL fixture | PASS |
| Полный go test -race | PASS | PASS с PostgreSQL fixture | PASS |
| Linux amd64 build | Не требуется | PASS | PASS |

Полный `paas-daemon/scripts/test-postgres.sh` прошёл с PostgreSQL 18.6 и
проверил PostgreSQL, Unix socket и перезапуск процесса. Полный ctl race suite
прошёл с SSH fixture.

- Чистый `flux v0.4.0 (7bf0d07b74871baa601b6388b53560ec6f6cbe01)` принят
  проверкой Makefile. Два запуска `make sdk` сохранили все 20 HTTP/OpenAPI
  и 6 gRPC/proto файлов, включая сравнение с состоянием до запуска.
- `openspec validate define-k3s-paas-foundation --strict` и `git diff --check`
  трёх репозиториев: PASS.
- Найдено и исправлено одно замечание errcheck в daemon DB-тесте: после
  принудительного Kill результат Wait теперь записывается в test log.

- Пользовательский прогон `govulncheck ./...` прошёл во всех трёх модулях;
  каждый завершился `No vulnerabilities found.`

Commit, push, PR и release не выполнялись. Прежние локальные изменения
трёх репозиториев сохранены. OpenSpec-подэтап 2.4 завершён и валиден.

## Команды для проверки

Запускать из терминала с рабочим DNS и доступом к GitHub. Блок использует
чистый отдельный worktree Flux; текущая ветка Flux остаётся на месте.
`go mod tidy` синхронизирует metadata рабочих PaaS-модулей.

```sh
(
  set -eu
  cd /Users/dm/Desktop/my-projects
  export GOWORK=off
  export GOTOOLCHAIN=go1.27.3
  go version

  PAAS_CHECK_DIR=$(mktemp -d /tmp/paas-sdk-check.XXXXXX)
  git -C flux fetch origin tag v0.4.0
  git -C flux worktree add --detach "$PAAS_CHECK_DIR/flux-source" v0.4.0
  (
    cd "$PAAS_CHECK_DIR/flux-source"
    go build -mod=readonly -o "$PAAS_CHECK_DIR/flux" ./cmd
  )
  "$PAAS_CHECK_DIR/flux" version
  make -C paas/paas-common sdk GENERATOR="$PAAS_CHECK_DIR/flux"

  GOBIN="$PAAS_CHECK_DIR/bin" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
  GOBIN="$PAAS_CHECK_DIR/bin" go install golang.org/x/vuln/cmd/govulncheck@v1.4.0

  for project in paas-common paas-daemon paas-ctl; do
    (
      cd "paas/$project"
      go mod tidy
      go mod verify
      go build -mod=readonly ./...
      go test -mod=readonly -count=1 ./...
      go test -mod=readonly -race ./...
      go vet -mod=readonly ./...
      go mod tidy -diff
      "$PAAS_CHECK_DIR/bin/golangci-lint" fmt --diff
      "$PAAS_CHECK_DIR/bin/golangci-lint" run
      "$PAAS_CHECK_DIR/bin/govulncheck" ./...
      git diff --check
    )
  done

  (cd paas/paas-daemon && GOOS=linux GOARCH=amd64 go build -mod=readonly -o "$PAAS_CHECK_DIR/paas-daemon-linux" ./cmd/paas-daemon)
  (cd paas/paas-ctl && GOOS=linux GOARCH=amd64 go build -mod=readonly -o "$PAAS_CHECK_DIR/paas-ctl-linux" ./cmd/paas-ctl)
  (cd paas/paas-common && openspec validate define-k3s-paas-foundation --strict)
)
```

Этот блок проверяет SDK и существующие unit/transport сценарии. Полная повторная
приёмка PostgreSQL-каталога требует отдельного запуска `paas-daemon/scripts/test-postgres.sh`
по инструкции в README daemon; успешный пропуск DB-тестов её не заменяет.
