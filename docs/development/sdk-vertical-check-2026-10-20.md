# Проверка первого PaaS HTTP SDK — 2026-10-20

## Разрешённый объём

Пользователь разрешил продолжить минимальный этап 2.3 четырьмя последовательными
шагами и провести одно строгое ревью GPT-6 Astra low после четвёртого.
Объём — согласованные preview/apply/get, без реализации daemon, БД, Kubernetes,
миграций, TUI и дополнительных API. Commit/push/PR не разрешены.

## Четыре шага

1. Flux spec: active api.yaml подключает deployments/operations/types.
   Deployment draft с Compose заменён на preview/apply; apps/logs сохранены
   неподключёнными. Все DTO используют явные required/non-null правила.
2. Генерация: Makefile подключает PaaS entrypoint; Go — pkg/http/paas,
   OpenAPI — docs/oapi/paas. Common Go-пакеты и gRPC/proto сохранены.
3. Transport: handwritten EncodeError подключается через httpx runtime без
   правок generated-кода. Decoder/body-limit возвращают PaaS ErrorResponse.
   Проверены типизированные статусы, YAML/query, optional/non-null и безопасные ошибки.
4. Полные проверки, повторяемость, контроль старых артефактов и документация.
   Ревью архитектора запрашивается только после этого шага.

## Инструменты и результаты

- Flux v0.3.0, чистый revision 8197264816e0a0f4f5fb681c7346e26ad2e1916c;
  binary /private/tmp/paas-sdk-22.4b386y/flux. Flux не изменялся.
- Go 1.26.5 darwin/arm64, packages-go v0.3.0, golangci-lint 2.11.0.
- Временные writable module/build/lint caches, GOWORK=off, GOPROXY=off.
- Для всего модуля PASS: go build -mod=readonly ./..., go test -mod=readonly
  -count=1 ./..., go test -mod=readonly -race ./..., go vet -mod=readonly ./...,
  go mod tidy -diff, golangci-lint fmt --diff, golangci-lint run (0 issues).
  Strict OpenSpec validate и git diff --check — PASS.
- Повторная генерация: SHA-256 всех 14 generated HTTP/OpenAPI-файлов совпали.
  Все артефакты common/gRPC/proto из снимка до этапа совпали по SHA-256.
- OpenAPI structural smoke: маршруты, статусы, required query, body limit
  и разрешимость локальных schema refs — PASS. Это не независимая формальная
  валидация OpenAPI и не проверка бизнесовой семантики схем.
- Логи проверок и manifest: /private/tmp/paas-sdk-23.EcdG4D; временные файлы
  не являются постоянной зависимостью проекта и могут быть очищены.

## Границы доказательства

HTTP-проверки используют in-process transport с generated client/handlers и JSON.
Они не открывают TCP-порт и не проверяют сеть, SSH или UDS.
Stub возвращает фиксированные варианты ответа; он не доказывает идемпотентность,
переходы state, TTL, хранение/очистку или создание операции в PostgreSQL.
Закрытые наборы значений, две формы preview, допустимость ID/пакета, сортировка
плана, readiness и timestamps — обязательства будущего daemon; SDK обеспечивает
структуру DTO, required/non-null, query и заданные body-политики.
Apps/logs, PaaS-метаданные и полный каталог stages/codes не реализованы.

## Итог проверок и ревью

Все четыре шага выполнены, проверки прошли. После четвёртого шага проведено
одно строгое ревью GPT-6 Astra с reasoning low: APPROVE, блокирующих P1/P2 нет,
дополнительные исправления для этого SDK-этапа не требуются.
Архитектор прочитал код, контракты, planning-документы и сохранённые test/race logs,
самостоятельно сверил 14/14 generated и 14/14 baseline файлов по SHA-256.
Полный набор тестов и генерацию повторно не запускал; формального OpenAPI validator
в ревью не было. Одобрение относится к минимальным четырём шагам, без runtime.
Пункт 2.3 целиком остаётся открытым для остального API и метаданных.
