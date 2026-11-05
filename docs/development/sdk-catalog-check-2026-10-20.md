# SDK каталога — 2026-10-20

Пользователь одобрил предложенный каталог и поручил пять итераций без ревью.
Реализован только SDK каталога чтения в paas-common. Ревью реализации в этом
цикле не проводилось. APPROVE Astra в предыдущем документе относится к предложению.

## Пять выполненных итераций

1. Принятый контракт перенесён из sdk-catalog-proposal-2026-10-20.md в delta
   independent-deployments. Proposal/design/tasks фиксируют разрешение и границы.
2. Добавлены Flux spec catalog.yaml/catalog_types.yaml и active imports;
   закреплённым Flux сгенерированы Go client/server/types и OpenAPI.
3. Добавлены восемь явных query/summary/page валидаторов в validation/catalog.go
   с граничными проверками и сохранением входа.
4. Проверены generated client/server, все 15 declared response variants,
   decoder/service/consumer boundaries и выбор app/component → preview.
5. Проверены весь модуль, повторяемость, сохранность и структура OpenAPI;
   оформлен этот отчёт с матрицей SDD. Ревью исключено по указанию пользователя.

## Контракт и совместимость

GET /apps/list, /apps/get, /components/list, /components/get используют query,
общий ErrorResponse и новые CatalogTypes DTO. Прежний legacy App из apps.yaml
не переиспользуется; apps/logs/common_types drafts остаются вне active graph.
Список возвращает non-null items, optional next_cursor; ComponentPage имеет
app_id даже для пустого списка. Отсутствующий параметр отклоняет generated decoder,
пустой ID и limit вне диапазона — явный вызов validation в fixture service.

Запросы принимают непустой opaque ID. Ответы проверяют существующий label-compatible
формат ID, name 1–128 UTF-8 байт без controls, cursor до 1024 байт, limit 1–100,
порядок/уникальность и внутреннюю принадлежность. DefaultCatalogLimit=50 используется
сервисом при отсутствии limit, не подставляется в generated query. DTO-проверка
не знает исходного запроса: consumer отдельно сравнивает app_id/component_id
и размер страницы с запрошенным limit. Вход не сортируется и не исправляется.

Снимок перед этапом: 47 файлов SDK/исходников/зависимостей/Makefile.
45/47 совпадают побайтно, включая прежние validators/tests, endpoint clients/servers,
OpenAPI preview/apply/get, common HTTP, gRPC/proto, Go deps и Makefile.
Ожидаемые изменения: docs/api/paas/api.yaml (два imports) и общий generated types.
Все прежние declarations/methods generated types сохранены побайтно; новые
Catalog declarations добавлены перед ними. Новые generated файлы создавались
Flux, вручную не редактировались. Две генерации дают одинаковые 20 артефактов.

## Матрица SDD

Нормативный источник: independent-deployments, «SDK предоставляет каталог
идентичностей приложений и компонентов». Пути относительно paas-common.

| Требование | Реализация | Проверка |
| --- | --- | --- |
| Четыре static GET, query, typed 200/400/404/503 | docs/api/paas/catalog.yaml; generated apps/components | TestCatalogTypedStatuses: все 15 variants, общий ErrorResponse и код сбоя зависимости |
| AppSummary/ComponentSummary/pages required/non-null/omission | catalog_types.yaml, generated types | TestCatalogJSONShape; пустая ComponentPage через client/server |
| Optional limit/cursor и default 50 | Generated query; DefaultCatalogLimit, явный validator; fixture | TestCatalogBoundaries и TestCatalogSelectionToPreview: explicit 0, 1/100, cursor round-trip; consumer/request count |
| Query presence до lookup | Generated decoder + явные query validators | TestCatalogQueryBeforeLookup: missing/duplicate/malformed query, empty ID/range и счётчик lookup |
| Неизвестное app vs чужой компонент | Declared 404 ErrorResponse | Fixture двух приложений, TestCatalogQueryBeforeLookup: APP_NOT_FOUND/COMPONENT_NOT_FOUND |
| IDs/name/cursor byte limits и controls | validation/catalog.go | TestCatalogBoundaries: UTF-8, invalid bytes, controls, label grammar и opaque query IDs |
| Страницы: non-null, максимум 100, порядок, уникальность, next_cursor | AppPage/ComponentPage | TestCatalogPagesAndNonMutation, TestCatalogMaximumPage; допустимы одинаковые display names |
| ComponentPage app binding и consumer/request binding | ComponentPage; explicit consumer fixture | Foreign element rejected; TestCatalogSelectionToPreview отклоняет wrong requested app и превышенный requested limit |
| Вызов проверки после decoding | Explicit consumer validation | TestCatalogSelectionToPreview: generated client декодирует unsorted page, validator отклоняет |
| Выбранные ID передаются в прежний preview | Existing generated deployments + envelope fixture | TestCatalogSelectionToPreview, подготовка только после PreviewRequest validation |
| Неизменность старых контрактов и generated boundaries | Baseline + source-only regeneration | 45 файлов SHA совпали, старый generated types suffix идентичен, active imports ровно два additions |
| Повторяемость и качество | Полный модуль и закреплённый Flux | Результаты ниже |

## Результаты проверок

Go 1.26.5 darwin/arm64, packages-go v0.3.0; golangci-lint 2.11.0.
GOWORK=off, GOTOOLCHAIN=local, writable временные GOMODCACHE/GOCACHE,
GOPROXY=off. Новые зависимости не добавлялись.

- go build -mod=readonly ./... — PASS.
- go test -mod=readonly -count=1 -v ./... — PASS.
- go test -mod=readonly -race ./... — PASS.
- go vet -mod=readonly ./... — PASS.
- go mod tidy -diff — PASS.
- golangci-lint fmt --diff и run — PASS, 0 issues.
- make sdk с Flux 8197264816e0a0f4f5fb681c7346e26ad2e1916c дважды — PASS.
- OpenAPI structural smoke: четыре GET, statuses, query required/optional,
  разрешение response refs, отсутствие nullable — PASS. Независимая формальная
  OpenAPI-валидация не заявляется.
- Строгая валидация OpenSpec и whitespace/diff проверки — PASS.

Baseline, копии до этапа, test.log, race.log, preservation.json, repeat.json
и openapi-smoke.json: /private/tmp/paas-catalog-4j_sx_u9. Каталог временный,
не является зависимостью проекта и может быть очищен.

## Открытый runtime и остаток API

In-process transport и fixtures не проверяют TCP/SSH/UDS, PostgreSQL или Kubernetes.
Настоящего каталога и создания записей ещё нет. Fixture cursor — условная позиция
для проверки wire-передачи/ошибок, не реализация DB-pagination. Полный обход,
scope cursor, продолжение после удаления позиции, конкурентные изменения,
глобальная уникальность/непереиспользование ID и неизменность принадлежности
должны проверяться runtime-тестами daemon. Каталог не подтверждает readiness.
Validators не выполняют lookup, не декодируют cursor, не санитизируют терминал.

Общий 2.3.3 остаётся открытым: создание/изменение app/component и подготовка
Namespace/прав, конфигурация мигратора/DB grants/сетевых связей/доменов,
версии/health, миграционные операции, список операций/логи и явные удаления.
1.1/1.3 и задачи daemon/CLI/TUI/live также не закрываются. Пять SDK-подзадач
завершены, change in-progress: 15/35 выполненных задач. Рост счётчика связан
с явным выделением пяти подзадач, а не с завершением всей 2.3.3.
