# Транспортное ядро ctl/daemon — 2026-10-21

Change: `define-k3s-paas-foundation`, задачи 3.0.1–3.0.5.
Пять итераций выполнены в согласованных границах UDS → OpenSSH → UDS.
Первая была сохранена до прерывания запуска; остальные завершены после продолжения.
Независимое ревью, commit, push и PR не выполнялись.

## Пять итераций

1. Сверены существующие Go-модули и delta. README обоих модулей описывают
   запуск, подготовку socket, ограничения этапа и временный local SDK replace.
2. Daemon проверен через настоящий локальный UDS. Исправлен вызов отсутствующего
   middlewarex.ServiceUnavailable: существующий TimeoutError отображается runtime
   в безопасный 503. До подключения store это фиксированная fail-closed readiness,
   не PostgreSQL ping. Каталог и UID/GID проверяются без ослабления прав.
3. CLI проверен с подставным SSH-процессом. Явно отключены IdentityAgent,
   password и keyboard-interactive authentication. Сохраняются отдельная identity,
   BatchMode, strict host key и запреты multiplexing/config/proxy hooks/agent forwarding.
   Ошибки cleanup и вывода статуса возвращаются вызывающему коду.
4. Проверены socket ownership, URL/Host/proxy/redirect, ошибочные аргументы,
   запуск/потеря процесса, cancellation/timeout, malformed health и cleanup.
   Подменённый socket сохраняется; неочищаемый каталог даёт ошибку.
5. Build/test/race/vet/tidy/fmt/lint трёх модулей прошли; ctl/daemon также
   собираются для Linux amd64. Сохранность прежнего SDK проверена SHA-256.

## Матрица SDD

Пути относительно workspace paas.

| Требование shared-infrastructure | Код | Проверка |
| --- | --- | --- |
| Go-модули и temporary local replace | paas-ctl/go.mod, paas-daemon/go.mod, README | GOWORK=off build/tidy; зависимости сохранены |
| Non-root команда daemon | paas-daemon/cmd/paas-daemon/main.go; daemon.CheckIdentity | TestIdentity проверяет текущую non-root identity; ветка отказа root проверена чтением кода, процесс от root не запускался |
| Каталог 0750, UID/GID, без symlink; занятый путь отклоняется | paas-daemon/internal/daemon/server.go: Listen | TestSocketOwnershipAndRefusal: неверные пути/права, файл, symlink в socket/каталоге, активный и stale socket |
| Socket 0660 и очистка только своего inode | Listen/cleanup | TestSocketOwnershipAndRefusal: режим и сохранение подменённого socket; исходный inode удерживается под другим именем |
| Generated health, safe ready 503, без фиктивных PaaS endpoints | Handler/service | TestHealthReadyAndUnwiredRoutes: 200/503, ErrorResponse без внутренней причины, прочие маршруты 404 |
| Локальный UDS и graceful shutdown по cancellation | Serve | TestServeUnixAndShutdown: generated health client через Unix dialer, listener останавливается, socket удаляется |
| Фиксированный endpoint без proxy/redirect | paas-ctl/internal/connection/client.go | TestUnixOnlyTransport: proxy env, 302 без перехода, чужие URL/scheme/Host, отказ до dial |
| OpenSSH args и каталог forwarding 0700 | connection.sshArgs/open | TestSSHArguments; TestSessionLifecycle |
| Health probe, timeout/ошибка запуска/потеря процесса, reap/cleanup | connection.open/Session.Close | TestSessionLifecycle, TestSessionFailuresCleanUp, TestSessionDetectsChildExit; malformed response не подтверждает health |
| Подмена endpoint не удаляется | Session.Close | TestSessionPreservesReplacement: чужой inode остаётся; ошибка очистки каталога возвращается |
| Status различает health и readiness | paas-ctl/cmd/paas-ctl/main.go | TestStatusCommand с подставным executable: 503 → not ready + error; fixture 200 → ready; ошибка writer возвращается |
| Ошибочный CLI input не показывает успех | CLI run | TestInvalidArguments: команда, флаг, лишний аргумент, порт, host |

Fixture-каталоги короткие, создаются в /tmp и канонизируются там, где нужна
проверка цепочки без symlink. Fixture daemon явно выставляет текущую группу:
на macOS /tmp наследует wheel, что не соответствует требуемому GID процесса.
Production-проверка владельца/группы не ослаблялась.

## Проверки

Go 1.26.5 darwin/arm64, packages-go v0.3.0, golangci-lint 2.11.0.
GOWORK=off, GOTOOLCHAIN=local, GOPROXY=off; использованы существующие caches.
Новых зависимостей и изменений go.mod/go.sum нет.

| Проверка | paas-common | paas-ctl | paas-daemon |
| --- | --- | --- | --- |
| go build -mod=readonly ./... | PASS | PASS | PASS |
| go test -mod=readonly -count=1 -v ./... | PASS | PASS | PASS |
| go test -mod=readonly -race ./... | PASS | PASS | PASS |
| go vet -mod=readonly ./... | PASS | PASS | PASS |
| go mod tidy -diff | PASS | PASS | PASS |
| golangci-lint fmt --diff | PASS | PASS | PASS |
| golangci-lint run | PASS, 0 issues | PASS, 0 issues | PASS, 0 issues |
| GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -mod=readonly ./... | отдельно не повторялось | PASS | PASS |

Первый daemon check выявил ошибку сборки и группу fixture; после исправлений
тесты прошли. Sandbox запрещал bind UDS: проверка выполнена с разрешённым доступом,
без skip. Первый lint CLI выявил ошибки вывода и условие hostname; после исправлений
проверки ctl/daemon повторены полностью. Прежние файлы paas-common, кроме tasks.md,
сохранены по SHA-256 относительно снимка перед этапом. Generated SDK/API YAML
не менялись, повторная генерация не требовалась.

Логи и results.json по каждому модулю, baseline.json и Linux build logs находятся
в `/private/tmp/paas-transport-20261021/`. Это временные локальные файлы;
постоянное описание результата — этот отчёт и тесты репозиториев.

## Границы результата

Проверены настоящий локальный UDS и generated HTTP client/server. Fake-process
и executable shim подменяют OpenSSH, но не проверяют SSH-аутентификацию.
Не проверялись реальный sshd/host key/серверный forwarding/удалённый socket,
разные пользователи и группы, Pod/hostPath/RBAC, k3s, PostgreSQL, сигналы
установленному daemon и GitHub Actions. Linux-сборка не доказывает выполнение
тестов на Linux. Команда daemon требует заранее подготовленный фиксированный
/run/paas/daemon/api.sock; установка не добавлялась.

Root/paas и локальному пользователю оператора доверяем согласно delta.
Этап не является усилением против конкурентной подмены путей доверенным
владельцем/root. SIGKILL/panic не обещают очистки; stale socket остаётся
доверенной подготовке после проверки остановки прежнего экземпляра.

Readiness остаётся 503; прикладные endpoints не подключены. PostgreSQL store,
установка, SSH-ключи/отзыв, TUI и деплои остаются задачами 3.x–7.x.
Следующий этап отдельно согласуется. Specs не синхронизированы, change не архивирован.
