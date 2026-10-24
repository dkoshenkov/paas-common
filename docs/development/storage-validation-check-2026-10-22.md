# Приёмка PostgreSQL-каталога — 2026-10-22

## Результат и объём

Приёмка согласованного подэтапа 3.3.1–3.3.5 закрыта по поручению пользователя
«закрой приемку». В OpenSpec завершены пять подпунктов; итог — 30/50.
Общий пункт 3.3 остаётся открытым до durable preview/operations и восстановления.
Установка PostgreSQL/k3s, production provisioning и готовность деплоев не заявляются.

Правило хранения сохранено: данные проверяет Go-приложение, PostgreSQL хранит;
удаление каталога выполняется через UPDATE deleted_at. Схема v2 не содержит
CHECK/FK/NOT NULL/PK/UNIQUE каталога; обычные индексы, Goose history и grants сохранены.
Production API записи/удаления отсутствует. Будущие writers должны сериализовать
проверки уникальности/связей и запись. Архитектура текущего этапа не расширялась.

## Окружение и воспроизведение

- Go 1.26.5 darwin/arm64; GOWORK=off, GOTOOLCHAIN=go1.26.5, -mod=readonly.
- PostgreSQL 18.6, aarch64-unknown-linux-musl, Alpine GCC 15.2.0.
- Образ: `postgres:18.6-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873`.
- Docker Engine 29.4.0. Fixture публикует случайный порт только на 127.0.0.1,
  хранит PostgreSQL в tmpfs и удаляется trap после выполнения.
- Использованы отдельные fixture БД/роли; существующие сервисы не изменялись.
- Для Docker/UDS проверки выполнены вне ограничений sandbox. Ранее полученные
  permission denied не потребовали изменений зависимостей или ослабления тестов.

Из paas-daemon:

```sh
GOWORK=off GOTOOLCHAIN=go1.26.5 ./scripts/test-postgres.sh -count=1 -v
GOWORK=off GOTOOLCHAIN=go1.26.5 ./scripts/test-postgres.sh -count=1 -race -v
```

Runner печатает image digest и SELECT version(). Оба запуска завершились успешно,
DB-проверки не пропущены. Без PAAS_TEST_POSTGRES_URL прямой go test пропускает
DB-тесты и не заменяет этот runner.

## Матрица доказательств

| Требование | Проверка | Результат |
| --- | --- | --- |
| Bounded secret-file, явные URL/identity/sslmode, безопасные ошибки | TestSecretConfig, TestPostgresEnvironmentCannotRedirect, TestPostgresServiceEnvironmentRejected | PASS |
| Пустой/неизвестный parent, 105 apps, пагинация, смена limit и soft delete позиции | TestPostgresCatalogPersistenceAndPaging | PASS |
| Missing/old/future schema, SELECT grants, отказ соединений и восстановление, cancellation | TestPostgresSchemaAndRecovery | PASS |
| Concurrent/repeated up и сохранение данных; rollback SQL-ошибки | TestPostgresMigrations | PASS |
| Deadline при занятой Goose lock.DefaultLockID; успешный up после unlock; deadline SQL pg_sleep с rollback DDL | TestPostgresMigrations | PASS |
| Отсутствие CHECK/FK/NOT NULL/PK/UNIQUE, неверные/NULL имена, ambiguous Get, дубликаты lookahead, tombstones и скрытие компонентов удалённого parent | TestPostgresApplicationValidationAndSoftDelete | PASS |
| Существующая v1 → v2 с сохранением app/component; повторный up | TestPostgresUpgradePreservesCatalog | PASS |
| COLLATE C / Go byte ordering apps/components, валидные неуникальные lookup indexes | TestPostgresByteOrderingAndIndexes | PASS |
| Runtime role не может выполнить DDL или INSERT | TestPostgresCatalogPersistenceAndPaging | PASS |
| Четыре generated GET client/server по настоящему UDS; 105 компонентов, смена limit, soft delete after, чужой scope/ownership | TestPostgresCatalogOverUnixAndRestart | PASS |
| Запуск процесса до схемы не выполняет DDL; old/future schema дают 503; health остаётся 200 | TestPostgresCatalogOverUnixAndRestart | PASS |
| Отказ DB identity (NOLOGIN + terminate sessions): ready/catalog 503, health 200; после LOGIN ready 200 без restart | TestPostgresCatalogOverUnixAndRestart | PASS |
| Занятая таблица истории: ready ограничен общим deadline 2 секунды | TestPostgresCatalogOverUnixAndRestart | PASS |
| Corrupt DTO → generic 500, canary/DSN/SQLSTATE не попадают в HTTP | TestPostgresCatalogOverUnixAndRestart | PASS |
| Штатная остановка и новый процесс daemon, socket cleanup, сохранение каталога и Goose history, отсутствие side effects чтения | TestPostgresCatalogOverUnixAndRestart | PASS |
| Права/ownership/symlinks/socket replacement и graceful shutdown | Существующие daemon/ctl transport tests | PASS |

Подпроцесс использует production Serve, Open, LoadConfig и generated handlers,
но запускается из test binary с подготовленным временным socket path. Это реальный
процесс/UDS/PostgreSQL, но не установка executable на /run/paas/daemon/api.sock.
TestPostgresDaemonProcess — helper запуска дочернего процесса, сам по себе
не является отдельным доказательством DB-приёмки.

## Полная матрица проверок

| Проверка | paas-common | paas-ctl | paas-daemon |
| --- | --- | --- | --- |
| build | PASS | PASS | PASS |
| test -count=1 | PASS | PASS | PASS с PostgreSQL |
| test -count=1 -race | PASS | PASS | PASS с PostgreSQL |
| vet | PASS | PASS | PASS |
| tidy -diff | PASS | PASS | PASS |
| golangci-lint fmt --diff | PASS | PASS | PASS |
| golangci-lint run | PASS, 0 issues | PASS, 0 issues | PASS, 0 issues |
| Linux amd64 build | Не требуется в подэтапе | PASS | PASS |

Strict OpenSpec validate и git diff --check трёх репозиториев: PASS.
Linux проверен сборкой; Linux runtime/tests в эту матрицу не входят.

## Сохранность и артефакты

[storage-validation-evidence-2026-10-22.json](storage-validation-evidence-2026-10-22.json)
содержит exit codes, версию PostgreSQL/образ, имена успешных DB-тестов,
SHA-256 исходников/SQL/go.mod/go.sum/runner и каждого лога, а также полный снимок
58 SDK/API файлов. Все 58 файлов совпали до/после приёмки. Дополнительно все
семь файлов прежнего generated-sha256.json из /private/tmp/paas-sdk-22.4b386y
сохранились побайтно. Регенерация SDK не требовалась и не выполнялась.

Полные логи текущей проверки: `/private/tmp/paas-acceptance-20261022/`.
Это временный локальный каталог; durable матрица и контрольные суммы сохранены
в evidence JSON рядом с отчётом.

За эту приёмку дополнены только интеграционные тесты daemon/store, вывод версии
в существующем runner и документация/OpenSpec. Production Go-код/SQL не менялись.
Commit/push/PR, sync main specs и archive не выполнялись.

## Границы

Не проверены live SSH/sshd, k3s/RBAC/hostPath, установленный Linux daemon,
изоляция бизнесовых БД, установка production roles/Secrets, preview/apply/operations,
деплои, TUI и физическая остановка/перезапуск PostgreSQL server. Восстановление
соединений доказано отказом fixture runtime identity и принудительным закрытием
её DB sessions, а deadline — блокировкой таблицы. Версия fixture не определяет
production-версию установки.

## История промежуточной проверки

Первый запуск 2026-10-22 был ограничен sandbox: Docker socket permission denied,
Unix bind operation not permitted, пять DB-тестов пропущены. Тогда приёмка была
открыта, 25/50. Текущая проверка снимает этот блокер реальными DB/UDS тестами;
прежние логи остаются в `/private/tmp/paas-storage-verify-20261022/`.
