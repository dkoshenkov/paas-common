# paas-common

Контракты, сгенерированные HTTP/gRPC SDK-артефакты и общие определения PaaS. Исходное описание API хранится в `docs/api`.
Сгенерированный код не редактируется вручную.

OpenSpec всей платформы находится в `openspec/`, отдельные backlog-файлы —
в `docs/backlog/`. Команды OpenSpec запускаются из корня этого репозитория.

## Структура

- `docs/api/common` — общие API-спецификации.
- `docs/api/paas` — PaaS API-спецификации.
- `docs/proto` — сгенерированные protobuf-описания.
- `docs/oapi` — сгенерированные OpenAPI-описания.
- `pkg/http` — сгенерированные HTTP-клиенты, серверы и модели.
- `pkg/grpc` — сгенерированные Go-типы protobuf и gRPC-клиенты/серверы.

## Требования

- Go 1.26.5 или новее.
- `protoc`.
- `protoc-gen-go`.
- `protoc-gen-go-grpc`.

По умолчанию Makefile ожидает `./flux`; бинарник в репозитории отсутствует.
Передайте путь через `GENERATOR=/path/to/flux`. Совместимость версий и полная
воспроизводимая генерация отслеживаются в `docs/backlog/flux.md` (FLUX-002).

## Генерация

Полная генерация общих SDK-артефактов:

```sh
make sdk
```

Отдельные этапы:

```sh
make sdk-http   # HTTP-клиенты, серверы и модели в pkg/http
make sdk-grpc   # protobuf в docs/proto и Go gRPC-код в pkg/grpc
make sdk-oapi   # OpenAPI-описания в docs/oapi
```
