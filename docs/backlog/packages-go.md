# Backlog packages-go для PaaS

- Источник потребностей: `openspec/changes/define-k3s-paas-foundation`.
- PaaS использует существующие `configx`, `logx` и `middlewarex/httpx`;
  исправление чтения JSON body принято через зависимость по PR #13.
- Обновление зависимости в `paas-common` относится к PaaS; изменения API библиотеки
  потребуют отдельного change и согласования в packages-go.

## PGO-001 — Совместимость runtime при обновлении SDK

- Статус: обновление проверено на Go 1.27.3; все три модуля проходят
  build/test/race/vet/tidy/fmt/lint/govulncheck и daemon PostgreSQL/UDS/restart
  fixture. Сканер сообщил `No vulnerabilities found` во всех модулях.
- `paas-common` обновлён с `v0.2.0` до `v0.3.0` на этапе 2.2 OpenSpec.
  Теперь `paas-common` и `paas-daemon` закрепляют
  `v0.3.1-0.20261024142719-ec269923c64b` по merged commit PR #13.
  `paas-ctl` получает runtime через зависимость от `paas-common`.
- Generated HTTP SDK использует существующий `middlewarex/httpx`.
  PaaS-формат transport errors подключается через `WithErrorEncoder`
  и `pkg/http/paas/transport.EncodeError`; расширение API библиотеки для него
  не требуется. Семантические валидаторы DTO остаются явными вызовами в PaaS.
- Проверки этапа обновления и сохранённых gRPC-пакетов зафиксированы в
  OpenSpec tasks 2.2 и README. Проверка 2026-10-24:
  `go test -mod=readonly -count=1 ./pkg/http/...` прошла на Go 1.27.1
  с `GOWORK=off`, без local replace packages-go.
- Подтверждённых несовместимостей, требующих изменения исходников библиотеки,
  в текущем HTTP-профиле не обнаружено. Текущие версии, ограничения проверки
  и команды записаны в
  [отчёте обновления](../development/sdk-runtime-upgrade-2026-10-24.md).

## PGO-002 — Общая HTTP-инструментация

- Статус: кандидат после выбора схемы мониторинга PaaS; API не согласован.
- Потребность: одинаковые метрики HTTP для бизнесовых сервисов и control plane.
- Работа: проверить существующие middleware и готовую инструментацию; выносить
  в packages-go только недостающую общую часть с реальными потребителями.
- Приёмка: выбранная интеграция используется в PaaS и примере бизнесового сервиса
  без дублирования; PaaS-специфичные labels и управление БД остаются в PaaS.

## PGO-003 — Чтение фактического JSON body

- Статус на 2026-10-24: исправление
  [PR #13](https://github.com/dkoshenkov/packages-go/pull/13) влито в main
  и закреплено в PaaS по merged commit; отдельного релиза пока нет.
- JSONDecoder больше не определяет пустоту по ContentLength. Проверяется
  фактический поток: непустое body с нулевой metadata длиной читается,
  действительно пустой поток с неизвестной длиной допускается при AllowEmptyBody.
  Whitespace-only input остаётся ошибкой; empty body не вызывает UnmarshalJSON.
- Добавлены regression tests и описание runtime-профиля. Tests/race/vet/tidy-diff,
  formatter и lint новых изменений проходят; TCP-тест в sandbox пропускается.
- До обновления PaaS HTTP-тесты прошли с исправленным runtime через replacement
  во временном проверочном модуле. Теперь рабочие зависимости обновлены;
  полный новый baseline проверен на Go 1.27.3.
