# SDK списка операций — 2026-10-20

Пользователь одобрил предложение списка операций, поручил использовать OpenSpec
и писать delta. Выполнено пять итераций без ревью. Новый нормативный источник —
independent-deployments, Requirement «SDK предоставляет список сохранённых операций».
Контракт перенесён туда полностью; документ-предложение сохранён как история.

## Пять итераций

1. Delta содержит маршрут, DTO/query/статусы, порядок, cursor/filter и retention,
   ограничения проверки и runtime. Proposal/design/tasks согласованы с delta.
2. В Flux operations.yaml добавлен List, в types.yaml — OperationPage.
   Go HTTP client/server/types и OpenAPI обновлены закреплённым Flux.
3. Добавлены явные OperationListQuery/OperationPage с переиспользованием
   catalogQuery/catalogPage и существующего Operation. Новых зависимостей нет.
4. Проверены typed ответы, query/service/consumer boundaries, все state,
   cursor/filter и путь list → get со сменой состояния/возможным 404.
5. Проверены полный модуль, повторяемость, сохранность и OpenAPI; оформлена
   матрица SDD. Ревью реализации не проводилось по указанию пользователя.

## Поведение и совместимость

GET /operations/list принимает optional state/limit/cursor; state отсутствует —
все пять состояний, иначе ровно одно из них. Limit: default 50, диапазон 1–100.
Cursor до 1024 decoded UTF-8 байт, непрозрачен и привязан к state/списку.
Ответ 200 — OperationPage с non-null items из прежних Operation и optional
next_cursor. Typed ошибки 400/503 используют прежний ErrorResponse.
Неверный state/limit/cursor shape отклоняются явной validation перед чтением.
Duplicate query и malformed int отклоняет generated decoder до service.
Проверки не меняют вход, не сортируют его и не раскрывают значения полей.

Список упорядочен по operation_id, не по времени. Проверяется один снимок
страницы; scope cursor, история и полнота обхода не устанавливаются validator.
Consumer отдельно сравнивает requested state и limit. State в последующем Get
может измениться, а очищенная история может дать 404. Чтение не продлевает TTL
и не запускает восстановление. Отдельная summary-модель/новые state не вводились.

Go API: OperationsServer теперь требует метод List; OperationsMiddlewares
содержит дополнительное поле List. Реализации сервера должны добавить метод;
позиционные struct literals middleware требуют адаптации. Две существующие
тестовые реализации получили только неиспользуемый List; прежние сценарии
остались без изменения. Wire contract Get сохранён.

Baseline перед этапом: 58 файлов SDK/source/deps/Makefile; 50 совпадают SHA-256.
Восемь ожидаемых изменений: operations.yaml/types.yaml, operations OpenAPI,
generated operations client/server и общий types, два прежних fixture-файла.
Автоматическое сравнение до/после подтвердило 90 прежних Go declarations/methods,
прежний Get handler и все прежние OpenAPI schemas/path Get. Исключения сравнения
Go declarations — расширенный server interface/middleware и функция регистрации;
её Get handler сравнивался отдельно. Common HTTP/gRPC/proto, каталог,
preview/apply, прежние semantic validators, зависимости и Makefile сохранены.

## Матрица SDD

Пути относительно paas-common; tests находятся в pkg/http/paas/validation.

| Требование delta | Реализация | Доказательство |
| --- | --- | --- |
| Static GET, optional state/limit/cursor; 200/400/503 | docs/api/paas/operations.yaml; generated operations client/server | TestOperationListWireAndSelection, TestOperationListBoundariesAndErrors |
| OperationPage non-null items/optional omission | types.yaml, generated types | TestOperationPageJSONAndNonMutation: missing/null и пустой [] без next_cursor |
| Пять state и пагинация, отсутствие vs zero | validation/operation_list.go: OperationListQuery; прежние pagination helpers | TestOperationListQueryBounds: все state, empty/unknown/case, -1/0/1/50/100/101, UTF-8 cursor 1024/1025, controls/invalid bytes |
| Decoder до service | Generated decoder + общий transport.EncodeError | Duplicate state/limit/cursor и malformed limit дают 400, calls/reads=0, проверен ErrorResponse |
| Service validation до чтения | Явный OperationListQuery в fixture | Invalid state/range/empty cursor дают typed 400 INVALID_REQUEST, reads=0 |
| Snapshot каждого элемента | OperationPage вызывает прежний Operation | TestOperationPageSnapshots: все пять state, invalid state/finished_at/nil diagnostics |
| Non-null/порядок/уникальность/100/next_cursor | OperationPage + catalogPage | TestOperationPageSnapshots: empty/nil/duplicate/unsorted, empty/long next, empty page+next, 100/101 |
| Неизменность и безопасная ошибка | Новые функции не мутируют вход | TestOperationListQueryBounds; TestOperationPageJSONAndNonMutation: valid/unsorted/duplicate, отсутствие значения ID/state в ошибке |
| Точный query round-trip | Generated client → in-process mux → fixture | TestOperationListWireAndSelection: absent params, каждый state, limit 1/100, cursor с +%&?, смена limit |
| Cursor scope и ошибки | Declared 400 ErrorResponse | Fixture отклоняет смену/удаление state с cursor и cursor другого списка; настоящий DB scope ещё не реализован |
| Пустой результат vs недоступность | Declared 200/503 | TestOperationListBoundariesAndErrors: empty [] без next, DEPENDENCY_UNAVAILABLE не скрывается пустым списком |
| Проверки после decoding и requested state/limit | Явный consumer fixture | TestOperationListConsumerChecksDecodedPage: malformed snapshot, wrong filter, превышение requested limit; DTO не исправляется |
| List → прежний Get | Существующий Get client/handler | TestOperationListWireAndSelection: тот же ID, допустимый новый state, затем typed 404 fixture |
| Сохранность и повторяемость | Baseline/declaration/path/schema checks + Flux | 50/58 SHA, 90 declarations, Get handler/schemas сохранены, 20/20 повторных generated hashes совпали |

## Результаты проверок

Go 1.26.5 darwin/arm64, packages-go v0.3.0, golangci-lint 2.11.0.
GOWORK=off/GOTOOLCHAIN=local/GOPROXY=off, временные writable caches.

- build -mod=readonly ./... — PASS.
- test -mod=readonly -count=1 -v ./... — PASS.
- test -mod=readonly -race ./... — PASS.
- vet -mod=readonly ./... — PASS.
- go mod tidy -diff — PASS.
- golangci-lint fmt --diff/run — PASS, 0 issues.
- make sdk с Flux 8197264816e0a0f4f5fb681c7346e26ad2e1916c дважды — PASS,
  20 generated артефактов идентичны.
- OpenAPI structural smoke — PASS: GET, три optional query, статусы,
  required items и refs на прежний Operation. Независимая формальная валидация
  OpenAPI не заявляется.
- Strict OpenSpec и whitespace/diff проверки — PASS.

Baseline, before copies, test.log/race.log, preservation.json, repeat.json,
openapi-smoke.json: /private/tmp/paas-oplist-3dbg1xl7. Каталог временный и может
быть очищен, проект не зависит от него.

## Открытые границы

Тесты используют in-process transport и fixture без TCP/SSH/UDS/БД/Kubernetes.
Fixture cursor сопоставляет фиксированные строки в памяти; он не реализует
настоящую пагинацию, продолжение после удаления позиции или конкурентное чтение.
Эти свойства, очистка, сохранение незавершённых записей, отсутствие повторов,
TTL и side effects требуют runtime-тестов daemon. Они сохранены в delta и
открытых задачах, не считаются выполненными проверками SDK.

Лимиты диагностики и ресурсные ограничения реальной выдачи ещё открыты в 1.1;
100 элементов не ограничивают размер полного ответа. App-wide миграции и
подготовка приложения не получают фиктивный component_id ради прежнего Operation.
Логи, создание приложений, Namespace/права, PaaS-метаданные, daemon/CLI/TUI
и live проверки остаются открытыми. Общий 2.3.3 не закрывается.

Пять подзадач SDK завершены: 20/40 задач change, in-progress. Счётчик вырос
из-за выделения пяти подзадач этого этапа. Commit/push/PR не выполнялись.
