# Семантическая проверка PaaS DTO — 2026-10-20

## Объём и порядок

Пользователь разрешил следующий цикл: четыре шага, затем одно строгое ревью Astra.
Реализован пакет pkg/http/paas/validation для уже утверждённых правил DTO.
Новые API, форматы ID, stages, PaaS-метаданные и runtime не проектировались.

1. Envelope и Diagnostic: непустые ID, 1–64 файла, уникальные имена до 512 байт
   UTF-8 без controls; severity, координаты источника и JSON Pointer.
2. Public Plan и PreviewResponse: идентичность group/kind/namespace/name без
   API version, стабильный порядок/уникальность, action/changed_fields,
   INITIAL_MIGRATION и две допустимые формы preview.
3. Operation/ErrorResponse: пять state, формат RFC3339 UTC Z, terminal finished_at,
   объяснение FAILED/RECOVERY_REQUIRED, OUTCOME_UNKNOWN и COMPONENT_BUSY.
4. Интеграционные проверки явного вызова на service/consumer boundary,
   полный набор проверок, сохранность артефактов и документация.

## Результаты

- Go 1.26.5 darwin/arm64; packages-go v0.3.0; golangci-lint 2.11.0.
- Для всего модуля PASS: build -mod=readonly, test -mod=readonly -count=1,
  test -mod=readonly -race, vet -mod=readonly, tidy-diff, fmt --diff,
  golangci-lint run (0 issues).
- Повторный make sdk с закреплённым Flux — PASS. SHA-256 всех 41 файлов снимка
  до этапа совпали: исходные контракты, generated common/PaaS HTTP/OpenAPI,
  прежние тесты/adapter, gRPC/proto, Makefile, go.mod и go.sum не изменялись.
- Строгая валидация OpenSpec и git diff --check — PASS.
  Логи и baseline: /private/tmp/paas-sdk-validation.khXqA4.
  Временный каталог может быть очищен, он не является зависимостью проекта.
- После исправления P2 и дополнения boundary/non-mutation тестов весь набор
  build/test/race/vet/tidy-diff/fmt/lint повторно PASS (0 issues), strict OpenSpec
  и diff-check PASS; повторная сверка 41 SHA-256 не выявила изменений.
  Логи: post-review-test.log, post-review-race.log, post-review-preservation.json
  в указанном временном каталоге.
- Новые проверки включают UTF-8 byte boundary, controls/дубликаты, координаты
  документа, JSON Pointer escaping, API version против identity, malformed dates,
  отсутствие/наличие finished_at, неизвестный результат и конфликтующие поля ошибки.

## Границы

Вызов явный: generated handlers/client не устанавливают валидаторы автоматически.
Функции возвращают первую ошибку без значений пользовательских полей и не меняют
вход. Они проверяют один снимок, не историю переходов или безопасность завершения.
Существование/формат opaque ID, clock/TTL, ownership и target bindings, readiness,
внутренний план, БД, Kubernetes и безопасный YAML parser остаются за daemon.
Diagnostic не является детектором секретов или терминальным renderer.
План не исправляется и не сортируется валидатором. Context-free проверка API version
не означает поддержки произвольного вида/версии Kubernetes.
HTTP integration использует in-process transport со stub, не TCP/SSH/UDS.
Пункт 2.3 целиком остаётся открытым для других API/метаданных; runtime не реализован.

## Ревью

Первое строгое ревью GPT-6 Astra low после четырёх шагов: CHANGES REQUIRED,
один P2 — context-free ErrorResponse ошибочно закрывал неизвестные code
с conflicting_operation_id. Исправлено: COMPONENT_BUSY требует непустой ID;
любой переданный ID непустой; PLAN_CHANGED как известный неблокирующий код
не принимает blocker ID; неизвестные code не закрыты общей проверкой формы.
Добавлена регрессия для неизвестного code с непустым/пустым ID.

Пользователь отдельно запросил актуализацию delta specs/change и дополнительный
SDD-аудит Astra: проверить всё, что должно быть реализовано в пределах SDK-объёма.
Матрица ниже использована для дополнительного аудита после полного набора проверок.

## Матрица SDD: требование → реализация → проверка

Все пути ниже относительно paas-common. Нормативный источник — delta spec
independent-deployments, требования «DTO и HTTP-ответы вертикального сценария
однозначны» и «SDK предоставляет явные проверки семантики DTO без runtime».
В tasks SDK разделён на 2.3.1, 2.3.2 и открытый остаток 2.3.3.

| Объём | Требование | Реализация | Доказательство |
| --- | --- | --- | --- |
| 2.3.1 | Flux spec, три статических endpoint и typed statuses | docs/api/paas/deployments.yaml, operations.yaml, types.yaml; pkg/http/paas/client и server | client/contract_test.go: TestApplyTypedStatuses, TestOperationQueryAndStates, TestOperationTypedErrors; OpenAPI structural smoke из первого отчёта |
| 2.3.1 | Required/non-null/optional omission, YAML/query | Generated types/client/server | client/contract_test.go: TestPreviewYAMLAndOptionalFields, TestNonNullCollectionsAndOptionalZero |
| 2.3.1 | Общий ErrorResponse для decoder/body-limit, безопасный cause | transport/errors.go и runtime hook | client/contract_test.go: TestTransportErrorsBeforeService, TestGeneratedClientDecodesTransportErrors; transport/errors_test.go: TestEncodeErrorHidesCause |
| 2.3.1/2.3.2 | Повторяемость и сохранность common/gRPC/proto/зависимостей | Makefile и закреплённый Flux | SHA-256 baseline/preservation; первый отчёт: 14 generated и 14 baseline, текущий: 41 pre-existing файлов |
| 2.3.2 | Envelope limits и имена; непустые opaque ID | validation/request.go: PreviewRequest, ApplyRequest, OperationQuery | request_plan_test.go: TestEnvelopeBoundaries |
| 2.3.2 | Severity, code/message, координаты и JSON Pointer | validation/request.go: Diagnostic | request_plan_test.go: TestDiagnosticCoordinates |
| 2.3.2 | Public Plan: identity/order/action/changed_fields/additional actions | validation/plan.go: Plan | request_plan_test.go: TestPlanActionsAndIdentity, TestPlanValidationDoesNotRewriteInput |
| 2.3.2 | Две формы preview и expires_at | validation/response.go: PreviewResponse | response_test.go: TestPreviewResponseShapes |
| 2.3.2 | Пять state, timestamps, finished_at, объяснение ошибок/unknown result | validation/response.go: Operation | response_test.go: TestOperationSnapshotRules, TestTimestampWireFormat |
| 2.3.2 | ErrorResponse, COMPONENT_BUSY и открытый code catalog | validation/response.go: ErrorResponse | response_test.go: TestErrorResponseRelationships, включая регрессию P2 |
| 2.3.2 | Явные вызовы до подготовки и после decoding | validation/integration_test.go: fixture service/consumer | TestSemanticEnvelopeAtServiceBoundary, TestConsumerRejectsSemanticallyInvalidResponse |
| 2.3.2 | Неизменность снимка; ошибки без пользовательских значений | validation/*.go, без assignments/sort к входу | TestEnvelopeBoundaries, TestPlanValidationDoesNotRewriteInput и code inspection |
| 2.3.1/2.3.2 | Build/test/race/vet/tidy/fmt/lint, актуальные артефакты | Модуль и OpenSpec change | Команды и результаты выше; SDD-аудит ниже |

Runtime-требования delta specs сохраняются, но не закрываются этими подэтапами:
YAML parser/128 документов/depth 64/aliases; БД/идемпотентность/TTL/квоты/очистка;
ownership/target bindings/internal plan; переходы state/Job/readiness/блокировки;
redaction/terminal rendering и TCP/SSH/UDS. Они остаются в открытых задачах
1.1/1.3/2.3.3/3.x–7.x. Их отсутствие не скрыто статусом SDK или stub-тестами.


## Итог дополнительного SDD-аудита

GPT-6 Astra low: **APPROVE**, новых блокирующих P1/P2 нет; прежний P2 устранён.
Архитектор сопоставил delta spec, proposal/design/tasks с кодом и тестами
по 2.3.1/2.3.2, проверил границы обоих четырёхшаговых циклов и не обнаружил
утраченных SDK-обязательств при разделении общего 2.3.
Подпункт 2.3.2 закрыт. Общий SDK остаётся незавершённым из-за открытого 2.3.3;
весь change — 10/30 выполненных задач, статус in-progress.

Аудит подтвердил generated endpoints/typed responses, required/non-null/omission,
YAML/query, безопасный transport ErrorResponse, все восемь функций validation,
явные service/consumer boundaries, неизменность входа и открытый каталог code.
Архитектор самостоятельно пересчитал SHA-256 baseline: 41/41 совпадают;
прочитал post-review-test.log/post-review-race.log. Полный набор Go-проверок
и генерацию в ходе аудита повторно не запускал: приведённые выше PASS получены
исполнителем. OpenAPI проверен структурно; независимая формальная валидация
в этом этапе не заявляется. Runtime и проверки TCP/SSH/UDS остаются открытыми.
