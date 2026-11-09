## Why

Пять чтений в `paas-daemon` требуют согласованного снимка PostgreSQL. В библиотеке
`packages-go` есть транзакции `sqltx` для `database/sql`. Каталог и история используют
нативный pgxpool и завершают чтение откатом.

## What Changes

- Добавить в `packages-go` нативный `pgxtx.ReadSnapshot` с ограниченным временем освобождения транзакции.
- Проверить потребителя в `paas-daemon` на настоящем PostgreSQL и закрепить опубликованную версию библиотеки.
- В `paas-common` хранить решение и результаты проверки. Пользователь одобрил архитектуру и локальную интеграцию 7 ноября 2026 года.

## Capabilities

### New Capabilities

- `pgx-read-snapshots` — совместимый контракт чтения в одной транзакции pgx.

### Modified Capabilities

## Impact

Затронуты `packages-go` и код хранилища `paas-daemon`. Контракт HTTP, схема БД, миграционный lock Goose и API `sqltx` сохраняются. Пакет опубликован в `packages-go v0.6.0`.
