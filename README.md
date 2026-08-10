# paas-sdk

SDK для HTTP- и gRPC-контрактов PaaS. Исходное описание API хранится в `docs/api`.
Сгенерированный код не редактируется вручную.

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

Генератор Flux находится в корне репозитория и запускается как `./flux`.

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
