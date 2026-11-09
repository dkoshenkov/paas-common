# paas-common

Контракты, сгенерированные HTTP/gRPC SDK-артефакты и общие определения PaaS. Исходное описание API хранится в `docs/api`.
Сгенерированный код не редактируется вручную.

OpenSpec всей платформы находится в `openspec/`, отдельные backlog-файлы —
в `docs/backlog/`. Команды OpenSpec запускаются из корня этого репозитория.

Текущая продуктовая линия `1.x.x` поддерживает одну ноду; мультинод запланирован
для `v2` и требует отдельного согласования. API в `1.x.x` развивается с сохранением
обратной совместимости.

[Однонодовая PaaS в схемах](docs/single-node.md) показывает путь запроса к
работающему API, целевое размещение компонентов в k3s и порядок деплоя.

## Структура

- `docs/api/common` — общие Flux spec; `http/api.yaml` — entrypoint HTTP-профиля.
- `docs/api/paas/api.yaml` — активные контракты каталога, настроек, БД, деплоя, миграций, истории, доменов и общих сервисов.
- `docs/api/github/api.yaml` — отдельный контракт получения публичных SSH-ключей GitHub для ctl.
- `docs/proto` — сгенерированные protobuf-описания.
- `docs/oapi/http` — сгенерированные OpenAPI-описания health/ready.
- `docs/oapi/paas` — OpenAPI утверждённого сценария PaaS.
- `pkg/http` — сгенерированные HTTP-клиенты, серверы и модели.
- `pkg/grpc` — сгенерированные Go-типы protobuf и gRPC-клиенты/серверы.

## Требования

- Go 1.27.3 или новее.
- `github.com/dkoshenkov/packages-go v0.3.1-0.20261024142719-ec269923c64b`
  (merged commit PR #13 до публикации релиза).
- Flux v0.7.0, commit `791064940350c2561c1c98d3a559e64e90e23d43`. Makefile сверяет путь Go-модуля и его контрольную сумму в готовом бинарнике либо точный commit и чистое дерево для локальной сборки.

По умолчанию Makefile ожидает `./flux`; бинарник в репозитории отсутствует.
Передайте путь через `GENERATOR=/path/to/flux`. `protoc` и его Go-плагины
для текущего HTTP/OpenAPI-профиля не нужны.

Обновление baseline от 2026-10-24 полностью проверено на Go 1.27.3, включая
PostgreSQL/UDS и govulncheck. Результаты и команды — в
[отчёте обновления](docs/development/sdk-runtime-upgrade-2026-10-24.md).

## Генерация

Из корня `paas-common` установите закреплённый Go-модуль Flux во временный каталог. При приватном доступе потребуются настроенные учётные данные Git и GOPRIVATE для Flux.

```sh
flux_build_dir=$(mktemp -d /private/tmp/paas-flux.XXXXXX)
GOWORK=off GOBIN="$flux_build_dir" go install github.com/dkoshenkov/flux/cmd@v0.7.0
make sdk GENERATOR="$flux_build_dir/cmd"
```

`make sdk` генерирует common и PaaS HTTP/OpenAPI. Исходные health/ready используют
актуальный DSL Flux без `proto`/`number`, с явными TypeRef. Entrypoint в подкаталоге
`http` соответствует discovery текущего Flux и сохраняет Go-пакеты в `pkg/http`.
OpenAPI перенесён из корня `docs/oapi` в `docs/oapi/http`.

Отдельные команды с тем же `GENERATOR`:

```sh
make sdk-http   # HTTP-клиенты, серверы и модели в pkg/http
make sdk-oapi   # OpenAPI-описания в docs/oapi/http и docs/oapi/paas
```

Существующие `docs/proto` и `pkg/grpc` сохраняются без регенерации.
`make sdk-proto` / `make sdk-grpc` завершаются с явной диагностикой:
новый proto/gRPC-профиль отложен до отдельного согласования.
`common/migration.yaml` остаётся неподключённым историческим черновиком;
команды Up/Down не добавляются в SDK этой генерацией.

## Первый PaaS HTTP SDK

`pkg/http/paas/{client,server,types}` и `docs/oapi/paas` генерируются из
`docs/api/paas/{deployments,operations,catalog,catalog_types,settings,settings_types,migrations,migration_types,types}.yaml` через entrypoint
`docs/api/paas/paas/api.yaml`. Common SDK остаётся в прежних Go-пакетах.
Активный SDK включает:

- `POST /deployments/preview`: пакет `app_id`, `component_id`, `files[{name,content}]`;
- `POST /deployments/apply`: `preview_id`, типизированные `200`/`202` и ошибки;
- `GET /components/images` показывает управляемые контейнеры и их образы. Для сокращённого обновления `POST /deployments/image/preview` принимает `app_id`, `component_id`, `deployment_name`, `container_name`, `image`. Оператор подтверждает полученный `preview_id` через обычный `POST /deployments/apply`. Для смены образа нужен подтверждённый успешный rollout этого экземпляра. Маршрут объявлен в отдельном Flux-сервисе `imagepreview`, поэтому прежние реализации `DeploymentsServer` сохраняют совместимость.
- `GET /operations/get?operation_id=...`: операция либо типизированная ошибка;
- `GET /operations/list`: список операций с фильтром state и пагинацией;
- `GET /apps/list`, `GET /apps/get`: чтение каталога приложений;
- `GET /components/list`, `GET /components/get`: чтение каталога компонентов;
- `POST /apps/create`, `POST /components/create` регистрируют по `request_id`;
- `GET /components/policy`, `POST /components/set-policy` читают и изменяют полный снимок политики;
- `GET /apps/migrator`, `POST /apps/set-migrator`, `POST /apps/remove-migrator` работают с образом мигратора и revision.

Каждый POST settings принимает до 1 MiB исходного JSON и возвращает типизированный 413 при превышении. Сервис settings имеет отдельный generated-интерфейс. Предыдущие реализации остальных сервисов сохраняют свои методы. До включения writers обновите читателей истории до версии, поддерживающей `Diagnostic.file` до 512 байт UTF-8. Семантику settings проверяют явные функции в `pkg/http/paas/validation`. Для корректно оформленной записи режима allowlist они возвращают `ErrEgressAllowlistUnavailable`.

Сервис `migrations` добавляет отдельные preview/apply ручного up и деплоя с миграцией. Для двух типов целей доступны `GET /operations/inspect`, `/operations/history` и `/operations/migration-log`. `OperationRecord` проверяет новую запись до 64 KiB, `ReadableOperationRecord` также принимает проверенную legacy-проекцию с добавленным полем target_type (до 65 562 байт). `MigrationLogPage` сверяет operation_id, after и limit запроса. Две обычные команды деплоя и прежние get/list сохраняют свой контракт. [Приёмка SDK](docs/development/migration-sdk-check-2026-11-08.md).

**Несовместимость с прежним черновиком:** активный deployment-контракт больше не
содержит Compose, path-параметры, List/Get/Create/Rollback. `apps.yaml` и `logs.yaml`
остаются историческими неподключёнными черновиками; их операции и ручной rollback
в новый SDK не входят. Это реализация минимального согласованного объёма 2.3,
а не завершение всех API платформы.

Обязательные поля DTO и non-null коллекции проверяются generated-кодом.
Пустые массивы передаются как `[]`; optional поля отсутствуют, а не равны `null`.
State/severity/action остаются строками: допустимые значения, две формы preview,
лимиты YAML и переходы операции должен проверять daemon. Генерация DTO эти
семантические проверки не реализует; для части правил доступны явные функции
`pkg/http/paas/validation`, описанные ниже. Идемпотентность,
хранение/очистка preview и применение Kubernetes остаются открытыми.
Служебные миграции PostgreSQL-каталога реализованы в `paas-daemon`;
запуск миграций пользовательских приложений остаётся отдельным этапом.

Для единого `ErrorResponse` установить encoder при регистрации handlers:

```go
runtime := httpx.NewRuntime(httpx.WithErrorEncoder(
    httpx.ErrorEncoderFunc(transport.EncodeError),
))
deployments.RegisterDeploymentsHandlersWithRuntime(mux, deploymentService, runtime)
operations.RegisterOperationsHandlersWithRuntime(mux, operationService, runtime)
```

Здесь `transport` — `github.com/dkoshenkov/paas-common/pkg/http/paas/transport`,
а `httpx` — `github.com/dkoshenkov/packages-go/middlewarex/httpx`.
Адаптер возвращает безопасные `400 INVALID_REQUEST`, `413 BODY_TOO_LARGE`,
`503 DEPENDENCY_UNAVAILABLE`; неожиданные внутренние ошибки — generic `500`.
Domain errors, включая `COMPONENT_BUSY` и `ACCEPTANCE_UNKNOWN`, сервис возвращает
через generated response variants с согласованным code. Обычная регистрация
без этого runtime использует стандартный encoder packages-go и не обеспечивает
PaaS-формат ошибок транспорта. `404/405` неизвестных маршрутов ServeMux находятся
вне данного endpoint-контракта.

SDK принимает внедрённый `*http.Client`; транспорт UDS/SSH выбирается в ctl,
серверный TCP listener в SDK не создаётся.

## Семантическая проверка DTO

Пакет `github.com/dkoshenkov/paas-common/pkg/http/paas/validation` проверяет
согласованные правила, которым не нужны БД, Kubernetes или разбор YAML.
Вызовы явные: generated handlers/client не подключают их автоматически.

| Функция | Что проверяет |
| --- | --- |
| `PreviewRequest` | Непустые app/component ID, 1–64 файла, уникальные UTF-8 имена до 512 байт без управляющих символов. |
| `ApplyRequest`, `OperationQuery` | Наличие непустого opaque ID, без lookup/TTL. |
| `Diagnostic` | Severity, code/message, связи file/document/line/column и синтаксис JSON Pointer. |
| `Plan` | Non-null массивы, уникальность/порядок ресурсов по group/kind/namespace/name, action/changed_fields, INITIAL_MIGRATION и отсутствие дубликатов дополнительных действий. |
| `PreviewResponse` | Ровно одна из двух форм ответа, diagnostics, public plan, наличие ID и формат expires_at. |
| `Operation` | Пять state, формат timestamps, finished_at терминальных состояний, объяснение FAILED/RECOVERY_REQUIRED, связь OUTCOME_UNKNOWN с восстановлением. |
| `ErrorResponse` | Непустые code/message, diagnostics и ID блокирующей операции для COMPONENT_BUSY, DATABASE_BUSY и ROUTE_BUSY. |

Функции возвращают первую ошибку с путём поля и не изменяют снимок. Тексты
ошибок не включают значения пользовательских полей. На сервере envelope-проверку
нужно вызывать до подготовки пакета, возвращая объявленный `400 ErrorResponse`.
Потребитель может проверять полученный DTO до использования, например:

```go
if err := validation.Operation(response.Body); err != nil {
    return err
}
```

Это проверка одного снимка. Формат и существование ID, переходы state, безопасное
снятие блокировок, TTL по времени БД, сравнение внутреннего плана, принадлежность
дополнительного действия приложению запроса и Kubernetes-правила остаются за daemon.
Валидаторы не проверяют число/глубину YAML-документов, aliases/merge и wire body:
первые требуют безопасного парсера, body ограничен generated transport.
Проверка Diagnostic не обнаруживает секреты в произвольном тексте и не заменяет
редактирование чувствительных значений и безопасный вывод в терминал.
Каталог error code открыт: общая проверка требует непустого blocker ID, если он
передан, и не отклоняет неизвестный code только из-за наличия ID. Уместность
ссылки на блокирующую операцию для таких кодов определяет сервис; известный
неблокирующий PLAN_CHANGED ссылки не принимает.

Проверки:

```sh
GOWORK=off go build -mod=readonly ./...
GOWORK=off go test -mod=readonly -count=1 ./...
GOWORK=off go test -mod=readonly -race ./...
GOWORK=off go vet -mod=readonly ./...
GOWORK=off go mod tidy -diff
golangci-lint fmt --diff
golangci-lint run
```

`TestCommonHTTPRoundTrip` проверяет generated client → generated handler → JSON
response для health/ready через in-process transport без открытия TCP-порта.
Повторный `make sdk` должен
оставлять байты generated-файлов неизменными. Тесты в `pkg/http/paas/client`
проверяют типизированные ответы, YAML/query, omission/non-null и decoder/body-limit
ошибки до сервиса тем же in-process способом. Stub не доказывает идемпотентность
или состояние daemon; TCP/SSH/UDS и runtime платформы этими тестами не проверяются.


### Каталог приложений и компонентов

Flux spec `docs/api/paas/catalog.yaml` и `catalog_types.yaml` генерирует GET
`/apps/list`, `/apps/get`, `/components/list`, `/components/get` в том же PaaS SDK.
Используйте AppListQuery/AppGetQuery/ComponentListQuery/ComponentGetQuery и
AppSummary/ComponentSummary/AppPage/ComponentPage из `pkg/http/paas/validation`
явно: generated-код не вызывает их автоматически. List-query допускает отсутствие
limit (сервис использует 50); переданный limit — 1–100. Cursor непрозрачен.
Consumer дополнительно сверяет ID ответа и число элементов с запросом.

Каталог содержит ID и отображаемые имена. Запись не подтверждает readiness или
готовность Namespace. В `paas-daemon` реализованы PostgreSQL-хранилище,
миграции каталога v1–v2 и четыре GET каталога; завершена приёмка 3.3.1–3.3.5.
Production API создания/изменения/удаления записей ещё отсутствует.
Прежние Compose apps/logs drafts не подключены. Контракт каталога добавляет четыре
маршрута и новые DTO; старый legacy App не переиспользуется. Отчёт:
[SDK каталога](docs/development/sdk-catalog-check-2026-10-20.md) и
[приёмка PostgreSQL-каталога](docs/development/storage-validation-check-2026-10-22.md).


### Список операций

`GET /operations/list` возвращает OperationPage из прежних Operation.
Query state/limit/cursor необязательны: отсутствие state выбирает все пять
состояний, limit по умолчанию 50, допустимый диапазон 1–100. Порядок обхода —
возрастание operation_id, без обещания хронологической сортировки. Cursor
непрозрачен и привязан к фильтру; смена limit допускается.

Вызывайте validation.OperationListQuery перед чтением хранилища и
validation.OperationPage после decoding явно. Consumer дополнительно сверяет
requested state/limit. Новый список не фиксирует snapshot на весь обход;
последующий Get может показать новый state или 404 после очистки истории.
Go OperationsServer теперь требует List; существующие реализации должны добавить
этот метод. Wire contract Get сохранён. Daemon читает историю из PostgreSQL (миграция v3). Перед выдачей он вызывает
`validation.HistoryOperation` для каждой прочитанной строки, включая следующую
за страницей. Запись операций при apply и фоновая очистка остаются открытыми.
Подробности контракта — в отчёте
[SDK списка операций](docs/development/sdk-operation-list-check-2026-10-20.md).
