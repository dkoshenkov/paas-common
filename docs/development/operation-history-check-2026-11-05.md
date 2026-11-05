# История операций: реализация и незавершённая приёмка — 2026-11-05

## Итог параллельного ревью и приёмки

Согласованный этап **3.3.6–3.3.10 завершён**, общий прогресс change — 38/58.
Общий 3.3, production writers, preview/apply, worker, recovery, k3s и deployment
остаются открытыми; их реализация этим результатом не разрешается.

Три независимых агента работали в отдельных репозиториях:

- common: 48 длинных descriptions редакционно разделены на краткое требование
  и именованные acceptance scenarios; исходные нормативные clauses сохранены
  с точностью до whitespace, названия requirements и 199 прежних scenarios
  сохранены. Два исторических Implementation Status перенесены в design.md.
- daemon: найден и исправлен off-by-one в wire size. Generated JSON encoder
  добавляет newline: проверка Get/List теперь резервирует его до headers.
  Live UDS regression: JSON 65535 байт → HTTP body 65536/200; JSON 65536 → generic 500.
  README уточнён; других production-дефектов в review не обнаружено.
- ctl: изменений не требуется; readiness, generated health/ready clients,
  UDS/SSH subprocess transport и cleanup проверены, checkout clean.

После сведения root повторил build/test/race/vet/tidy-diff/fmt-diff/lint всех
трёх модулей и Linux amd64 build daemon/ctl: **PASS**. Go 1.27.3,
golangci-lint 2.15.0. Полный live PostgreSQL 18.6/UDS/generated-client/restart
suite с race после wire fix: **PASS без skip**. Прежние 58 SDK/API-файлов
сверены с исходным HEAD: все хеши совпадают. Strict OpenSpec 1.16.0: **PASS,
0 issues**. Обе Mermaid-диаграммы сохранены.

Итоговые логи, независимые review, проверка сохранности спецификаций и SHA-256:
[final-parallel-review](operation-history-evidence/final-parallel-review/).

Локальный replace `../paas-common` сохранён: daemon требует совместного checkout
common. Реальная SSH-аутентификация и remote deployment не проверялись и не входят
в этап. В daemon/ctl находятся только reusable workflows без caller, поэтому
локальная приёмка не доказывает GitHub CI. Публикация использует existing PR
common #20 и daemon #3; merge/deploy не выполняются. Локальные `.agents/skills`
установлены для рабочего окружения и не включены в публикацию продукта.

Ниже сохранена хронология предыдущих проверок; прежние блокеры и выводы
заменены итогом этого раздела.

## Повторная приёмка в текущем окружении

Архив восстановлен в `/workspace/paas-restored`; проверены SHA-256, исходные HEAD,
режимы файлов и точный Git status всех трёх checkout. Интернет работает.
Локально в `.agents/skills` установлены шесть навыков OpenSpec 1.16.0 и
подтверждённый ponytail v5.3.3 (commit `7801d061243d7b753bad0e0b3c057cd250b11755`).
Это установка для проекта; личный каталог навыков недоступен для записи,
авторизация его поддерживаемого remote отсутствует.

Инструменты: Go 1.27.3 и golangci-lint 2.15.0, собранный Go 1.27.3.
Пересборка старого golangci-lint 2.11.0 не помогла: его анализаторы не понимают
export data Go 1.27. В репозиториях зависимости и конфигурация линтера сохранены.

| Проверка | Текущий результат |
| --- | --- |
| build/test/race/vet/tidy-diff трёх модулей | PASS |
| golangci-lint fmt/run трёх модулей | PASS |
| Linux amd64 build daemon/ctl | PASS |
| Полный live PostgreSQL/UDS suite с race | PASS, без skip |
| PostgreSQL | 18.6, прежний pinned digest `sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873` |
| Прежние SDK/API | 58/58 SHA-256 совпадают с HEAD и прежним evidence |
| OpenSpec без strict | PASS |
| OpenSpec strict | FAIL: те же 50 WARNING, 0 ERROR и 0 новых замечаний |

Ревью охватило validator и прежние DTO, SQL/cursor/транзакции, миграцию,
readiness, service/generated HTTP adapter, fixture permissions и restart.
Ошибок в production-коде не обнаружено. Дополнены meaningful acceptance tests:
105 операций и пять state через generated client/UDS, смена limit, удаление
cursor position, scope rejection, oversized snapshot до успешного ответа;
проверены четыре nullable колонки, отсутствие constraints и два обычных индекса.
Обе Mermaid-диаграммы сохранены в README daemon.

3.3.6–3.3.9 отмечены выполненными. 3.3.10 остаётся открытым из-за strict:
48 requirements длиннее 500 символов и два `Implementation Status` раздела,
которые archive не переносит в основную спецификацию. Это прежние замечания
четырёх spec-файлов; их наличие не объявлено успешной strict-проверкой.
Коммиты/push/PR отложены до завершения приёмки. Существующие PR common #20 и
 daemon #3 открыты, их HEAD совпадают с восстановленными checkout.

Текущие логи и результаты: [current-environment](operation-history-evidence/current-environment/).
Разделы ниже сохраняют исторический результат до восстановления окружения;
прежние toolchain/network блокеры в них сняты текущими проверками.

## Scope

Пользователь одобрил подэтап 3.3.6–3.3.10: «да, работай пока по openspec»
после предложения ограниченного validator, PostgreSQL v3, read-only Get/List
и проверок PostgreSQL/UDS/restart. Новых архитектурных решений не добавлено.
Задачи остаются открыты: полный production build и интеграционные тесты не прошли
из-за несовместимости инструментов окружения. Это не завершение PaaS или 3.3.

Проверены актуальные remote PR refs через `git ls-remote`, fetched HEAD:

| Репозиторий / PR | HEAD |
| --- | --- |
| paas-common #20 | a2ef8bfa0407471b8ec04474440f2d073141ad25 |
| paas-daemon #3 | 4eac5bd7f0c1ac811a16817057849f681010d201 |
| paas-ctl #3 | b96b3554811f96a385b70c28a6eae9d35e5fb181 |

Checkout находятся на detached HEAD этих PR. Commit/push/PR/merge/deploy не выполнялись.
Пользователь позднее разрешил атомарные commit, push и PR **после завершения задачи**.
Приёмка ещё не завершена, поэтому публикация отложена. После проверок следует
продолжить существующие PR; новые PR при необходимости — draft, без дубликатов.
Merge и deployment не разрешены.
`AGENTS.md` прочитан в common; в двух других репозиториях его нет.
`.agents/skills` и навыки ponytail/openspec-* в checkout и executor catalog отсутствуют;
соблюдён описанный в AGENTS цикл уже согласованного OpenSpec change.

## Changes

- common: явные `OperationHistoryID`, `OperationHistory`, `OperationHistoryPage`,
  проверка UTF-8/control characters, byte/count/JSON bounds без изменения прежних
  валидаторов, DTO, API source или generated SDK.
- daemon: Goose v3 добавляет nullable BYTEA-историю без constraints; readiness
  требует schema 3 и SELECT новой таблицы. Runtime остаётся read-only.
- Get/List читают ID/state/snapshot одной строки в repeatable-read транзакции,
  проверяют snapshot и его размер до decode, колонки, DTO, порядок/дубликаты
  с lookahead, canonical state-scoped cursor и soft delete.
- Сервис проверяет полный JSON ответа до успешных headers. Ошибки повреждённых
  записей не раскрывают snapshot/driver error. Preview/apply не подключены.
- Добавлены unit/integration сценарии; существующий UDS subprocess test расширен
  историей, generated clients, правами, corrupt canary и restart/no side effects.
- README описывает миграцию/grant и содержит sequence/flowchart Mermaid.
- В test-postgres.sh readiness проверяется по TCP localhost: временный init-сервер
  PostgreSQL слушает только Unix socket и раньше мог дать преждевременную готовность.

## Verification

| Проверка | Результат и границы |
| --- | --- |
| 58 прежних SDK/API файлов | SHA-256 совпадают с HEAD и evidence 2026-10-22 |
| Unit + race + vet валидаторов | PASS в изолированной копии stdlib-only types/validation на Go 1.26.5; не приёмка полного модуля |
| SQL v1 → v2 → v3 | PASS на pinned PostgreSQL 18.6; tombstone сохранён, 4 nullable колонки, 2 обычных индекса, 0 constraints, SELECT=true/INSERT=false |
| build/test/race/vet/tidy всех 3 модулей | BLOCKED: go.mod требует Go 1.27.3, установлен 1.26.5 |
| Linux amd64 build daemon/ctl | BLOCKED тем же toolchain |
| Полный PostgreSQL/UDS/restart suite | BLOCKED Go 1.27.3; DB fixture успешно запускается после исправления readiness |
| Стандартные fmt/lint | BLOCKED: golangci-lint собран Go 1.26, target 1.27.3 |
| Форматирование исходников | gofmt и gofumpt/gci через временную копию config с run.go=1.26; repository config сохранён |
| OpenSpec 1.16.0 strict | FAIL: 50 WARNING; список точно совпадает с чистым HEAD; 0 новых замечаний |
| git diff --check | PASS |

Изолированная проверка копирует `pkg/http/paas/types/*.go` и
`pkg/http/paas/validation/*.go` (без `*integration_test.go`) в отдельный каталог
с минимальным module `github.com/dkoshenkov/paas-common`, go 1.26.5.
В этой копии выполнялись `go test -count=1 -race ./...` и `go vet ./...`.
Production go.mod/go.sum, SDK и зависимости не изменялись.

SQL проверен непосредственно psql, **не через Goose Provider**. Повторный/concurrent
Goose up и реальные handlers/process restart пока не подтверждены запуском.
Наличие написанных integration-тестов не считается PASS или закрытием задач.

Логи, результаты команд и SHA-256: [operation-history-evidence](operation-history-evidence/).
`checks.json` содержит коды завершения обязательных команд. `sdk-sha256.json`
содержит 58 сравнений. `openspec-baseline.json` позволяет проверить прежние warnings.

## Blockers and resumption

Автозагрузка `GOTOOLCHAIN=go1.27.3 go version` дошла до перенаправления
proxy.golang.org на storage.googleapis.com и получила Forbidden. Этот host
не разрешён текущей network policy. Повтор или обход запрета не выполнялся.
Нужен Go 1.27.3 в окружении либо разрешённая загрузка toolchain через штатное
обновление сетевой конфигурации, плюс golangci-lint, собранный совместимым Go.

После исправления окружения:

1. Выполнить обязательные build/test/race/vet/tidy/fmt/lint во всех модулях.
2. Выполнить `paas-daemon/scripts/test-postgres.sh` на закреплённом image;
   проверить, что PostgreSQL/UDS/process tests не skipped.
3. Выполнить Linux amd64 build daemon/ctl.
4. Разобрать 50 прежних strict OpenSpec warnings либо восстановить закреплённую
   версию CLI прошлой приёмки; не объявлять текущую strict-проверку успешной.
5. Обновить evidence и только затем закрыть проверенные задачи 3.3.6–3.3.10.

Production writers, preview/apply, worker, cleanup, recovery и k3s/deploy вне этапа.
Следующий этап требует отдельного согласования по AGENTS.md.
