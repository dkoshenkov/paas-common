# Проверка Flux для PaaS — 2026-10-19

Исторический отчёт о Flux v0.3.0. С 2026-10-24 Makefile закрепляет v0.4.0;
текущее обновление и команды проверок описаны в
[sdk-runtime-upgrade-2026-10-24.md](sdk-runtime-upgrade-2026-10-24.md).

## Проверенное состояние

- Репозиторий: `../flux`, чистая ветка `main`; GitHub HEAD — `main`, ветки `master` нет.
- `git pull --ff-only origin main`: успешно, `Already up to date`.
- Commit: `8197264816e0a0f4f5fb681c7346e26ad2e1916c`.
- Свежий бинарник: `/private/tmp/paas-flux-build.c2CHQ0/flux`.
- `flux version`: `flux v0.3.0 (8197264816e0a0f4f5fb681c7346e26ad2e1916c)`.
- Сборка `go build -mod=readonly -o <binary> ./cmd` и полный
  `go test -mod=readonly -count=1 ./...` в Flux прошли. Отслеживаемый `../flux/flux`
  не перезаписывался; исходники Flux и зависимости PaaS не менялись.
- Probe выполнен во временном модуле `example.com/paas-flux-probe`
  с опубликованным `github.com/dkoshenkov/packages-go v0.3.0`, без local replace.
  Go: `go1.27.2 darwin/arm64`.

## Результаты

| Проверка | Наблюдение |
| --- | --- |
| Совместимый статический GET /health | Сгенерированы client/server/types/OpenAPI |
| Сборка generated-кода и реальный HTTP round-trip | PASS: generated client → httptest.Server с generated handler → typed response |
| Повторная генерация | PASS: SHA-256 всех четырёх generated-файлов совпали |
| OpenAPI статического маршрута | Есть /health, operationId AppsHealth и ссылка на response schema; это smoke-проверка, не независимый OpenAPI validator |
| GET /apps/{name} | Генерация отклонена: `path templates are not allowed` |
| Шаблон пути с явным in: path | Генерация отклонена: `path templates are not allowed` |
| Самостоятельный in: path на статическом пути | Отклонён: `unsupported in "path" (expected query)` |
| Поле number из текущих PaaS YAML | Отклонено: `unknown field "number"` |
| Короткий TypeRef App из текущих PaaS YAML | Отклонён: `invalid TypeRef "App"`; совместимый формат — ./api.yaml#App |
| --target proto из текущего Makefile PaaS | Отклонён: допустимы только all, go, oapi |

Отрицательные проверки дополнительно подтвердили отсутствие сгенерированных
Go/OpenAPI-файлов после отказа. PASS теста ожидаемого отказа не означает поддержку
path-параметров. Для /apps/{name} код не создан, поэтому его client/server round-trip,
кодирование path-значения и OpenAPI path parameter не проверены и не объявлены PASS.

## Минимальное воспроизведение блокера path

Во временном Go-модуле создать `docs/spec/probe/api.yaml`:

```yaml
services:
  apps:
    endpoints:
      - name: Get
        method: GET
        path: /apps/{name}
        responses:
          "200": ./api.yaml#App
types:
  App:
    kind: struct
    fields:
      name: {type: string}
```

Запуск свежего бинарника из корня этого временного модуля:

```sh
/private/tmp/paas-flux-build.c2CHQ0/flux generate --target all --in docs/spec --go-generated-path internal/generated --oapi-generated-path docs/oapi
```

Результат: код 1, `path templates are not allowed: "/apps/{name}"`.
Если изменить путь на `/health`, совместимый контрольный контракт генерируется.
Добавление `number: 1` в options поля либо замена `./api.yaml#App` на `App`
воспроизводят независимые несовместимости текущего DSL PaaS.

Текущие полные evidence во временном каталоге (не постоянные project dependencies):

- `/private/tmp/paas-flux-path.YfUErm/verify.py` — runner пяти отрицательных проверок,
  положительной генерации, SHA-256 повторяемости, OpenAPI smoke и HTTP round-trip.
- `/private/tmp/paas-flux-path.YfUErm/report.json` — результаты runner.
- `/private/tmp/paas-flux-path.YfUErm/control/path_probe_test.go` — HTTP-тест.
- `/private/tmp/paas-flux-path.YfUErm/control/go-test.log` — успешный запуск Go-теста.
- `/private/tmp/paas-flux-path.YfUErm/cases/*/generate.log` — ожидаемые отказы.
- Проверка `--target proto` выполнена отдельно тем же бинарником на control-контракте.

## Следствие для SDD

Задача проверки 2.1 выполнена; FLUX-001 подтверждён как блокер генерации выбранных
PaaS-маршрутов. Для исправления нужна отдельная согласованная работа в Flux:
валидация path-параметров, типизированная передача в generated client/server,
кодирование значений и OpenAPI. Ручной транспорт и замена path на query в PaaS
не выполнялись.

### Дополнительная проверка API без path-параметров

По запросу пользователя изучены актуальные DSL и исходники Flux и проверен
альтернативный транспортный контракт во временном каталоге
`/private/tmp/paas-flux-query.8CJKMM`. Это проверяемый вариант для обсуждения,
не уже утверждённая замена production API:

- `GET /apps/get?name=...` — типизированный query, ответы 200/404.
- `POST /deployments/create` — JSON app/component/manifests, ответы 202/409.
- `GET /operations/logs?operation_id=...&cursor=...&limit=...` — optional query,
  массив строк и следующий cursor.

С тем же неизменённым Flux и опубликованным packages-go v0.3.0 сгенерированы
четыре файла. Выполнен `GOWORK=off go test -mod=mod -count=1 -v ./...` во временном
модуле с реальным `httptest.Server` и generated client/server. Все семь subtests
прошли: query round-trip с символами URL/Unicode, typed 404, точная передача
многострочного YAML в JSON и typed 202, typed 409, optional zero/empty и cursor,
отказ отсутствующего/повторного query до сервиса, отказ неизвестного body-поля
до сервиса. Специальные символы в имени проверяют транспорт, не допустимость
такого имени приложения на уровне бизнес-контракта.

Повторная генерация идентична по SHA-256; OpenAPI smoke подтверждает static paths,
query, requestBody и 202/409. Это не независимая формальная валидация OpenAPI.
Файлы: `spec/paas/{api,types}.yaml`, `query_probe_test.go`, `result.json`
в указанном временном каталоге. Generated-файлы вручную не менялись.

Вывод уточнён: для собственного PaaS HTTP API доработка Flux не обязательна,
если пользователь согласует static paths + query/JSON-body. FLUX-001 является
блокером именно path-формы, не бизнес-операций PaaS. Статус/логи можно получать
обычными запросами; streaming в доказанный профиль не входит. Полная схема
манифестов, её бизнесовая валидация и runtime платформы этим probe не реализованы.

Отдельное ограничение — внешний GitHub API. Согласованный ранее
`GET /users/{username}/keys` нельзя переопределить на query. Фиксированный
`GET /user/keys` может обойти отсутствие path-параметров, но требует авторизации
и возвращает ключи владельца credentials, а не произвольного username. Это
изменение пользовательского сценария требует отдельного согласования.
Источник: https://docs.github.com/en/rest/users/keys.

Текущие `docs/api/paas` — старый DSL и Compose-модель. Их адаптация относится
к согласованию/генерации новых контрактов, а не к добавлению поддержки `number`
в Flux. Доступность нового proto/gRPC target не подтверждена: он отсутствует
в CLI этой версии. Профиль первого SDK (HTTP/OpenAPI либо также новый gRPC)
нужно явно согласовать до изменения Makefile.

Установка k3s, UDS/SSH на сервере, Goose, SQL grants и платформенная изоляция
этой проверкой не проверялись. К реализации платформы переходить на основании
только положительного статического HTTP-теста нельзя.
