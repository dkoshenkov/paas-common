# Приёмка внутреннего durable admission — 2026-11-05

Статус: внутренний этап 3.3.11–3.3.15 реализован и проверен на настоящем
PostgreSQL/UDS с race, отдельными процессами и fault injection. Пользователь
одобрил архитектуру ответом «делай» 2026-11-05. Выполнено 43/63 задач; общий 3.3,
production preview/apply и готовность PaaS не объявляются завершёнными.

## Источники и граница этапа

- [Proposal](../../openspec/changes/define-k3s-paas-foundation/proposal.md),
  [design](../../openspec/changes/define-k3s-paas-foundation/design.md),
  [independent-deployments](../../openspec/changes/define-k3s-paas-foundation/specs/independent-deployments/spec.md),
  [tasks](../../openspec/changes/define-k3s-paas-foundation/tasks.md): согласованный этап 3.3.11–3.3.15.
- База архитектуры: common `4bbf001`, daemon `7e2316d`, ctl `a7a50ca`.
  Эти ревизии не являются evidence новой реализации.
- [Приёмка предыдущего этапа](operation-history-check-2026-11-05.md): каталог и
  история 3.3.6–3.3.10; прежние PASS не переносятся на новый write path.
- [PostgreSQL: isolation](https://www.postgresql.org/docs/18/transaction-iso.html),
  [advisory locks](https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS),
  [DB clock](https://www.postgresql.org/docs/18/functions-datetime.html): основания
  READ COMMITTED, transaction-scoped lock и финальной проверки clock_timestamp.

Production POST preview/apply, CLI deploy, production DML credentials, scheduler,
executor и Kubernetes recovery не входят в этап. Runtime сохраняет SELECT-only
identity. Common SDK/API не расширяются. Fixture planner не доказывает полноту
реального normalization/policy profile или безопасность исполнения Kubernetes.

## Результаты архитектурного ревью

Ревью draft до реализации не выявило блокирующих противоречий после уточнений:
canonical comparison payload и typed claims отделены от будущего production
planner; вложенные routes разрешены по прежнему профилю; финальная проверка DB
clock выполняется после replan; ownership ledger отделён от истории; terminal
retirement допускается только для явно подготовленных безопасных fixtures.
Независимое common-ревью подтвердило 38/63 задач, strict `issues=[]` и чистый
`git diff --check` на рассмотренном draft. Это проверка документов, не runtime.

Preview сохраняет 64 unused records / 256 MiB / TTL 30 минут. Внутренний active
профиль: 16 records / 256 MiB; общий write deadline 2s. History-only ограничения
не применяются к private preview: в частности, diagnostic file name preview
допускает 512 UTF-8 bytes, а публичная история имеет собственный предел 256.
Квоты не ограничивают tombstones, WAL/MVCC/TOAST, индексы или backup.

## Матрица доказательств

Локальные PASS относятся только к внутреннему fixture-профилю. Логи и параметры
среды приложены ниже; skip live DB не считается PASS. Daemon CI на итоговом HEAD подтверждён; Dependency review в нём временно NOT RUN по прежнему разрешению, govulncheck выполнен.

| Задача | Проверка / требуемое доказательство | Статус |
| --- | --- | --- |
| 3.3.11 | Goose v3→v4 и повторная миграция; nullable scalars/BYTEA, обычные индексы, отсутствие новых бизнес-constraints/triggers | Локально PASS: MigrationV4; full suite |
| 3.3.11 | Versioned envelope/canonical payload/typed claims; Go-validation, duplicate/NULL/corruption fail-closed | Локально PASS: MalformedRecords, CanonicalJSON, Claims |
| 3.3.11 | SELECT-only startup/readiness; прежние GET по UDS; generated SDK/API без изменений | Локально PASS: UDS narrow grants; contract diff=0 |
| 3.3.12 | Два отдельных writer process; один advisory key; чтения видят commit предыдущего writer после ожидания | Локально PASS: ConcurrentProcesses, TTLAndLock |
| 3.3.12 | Общий deadline 2s, cancellation/rollback, отсутствие внешних вызовов под lock | Локально PASS: TTLAndLock, OwnershipAndRollback |
| 3.3.13 | Immutable input/full plan/diagnostics, pinned serializer/version, отсутствие потери null/отсутствующих полей | Локально PASS: CanonicalJSON, EnvelopeAndAccounting |
| 3.3.13 | Unused count/bytes boundaries с UTF-8/escaping; expired count до cleanup; нет live eviction | Локально PASS: Quotas, ByteQuotas/unused, QuotaRaceAndCleanup |
| 3.3.13 | Active count/bytes отдельно от unused; history-only limits не ограничивают preview | Локально PASS: ActiveQuota, ByteQuotas/active, EnvelopeAndAccounting |
| 3.3.13–14 | TTL истёк при ожидании lock и во время replan; clock_timestamp перед финальным решением | Локально PASS: TTLAndLock |
| 3.3.14 | Существующая связь до TTL/квот; concurrent replay одного preview без новой операции | Локально PASS: ReplayAndUnknownCommit, ConcurrentProcesses |
| 3.3.14 | Full comparison и typed claims; component/resource/owner conflicts; согласованный route profile с разрешённым nesting | Локально PASS: RecheckAndClaims, NormalizedRoutes, OwnershipAndRollback |
| 3.3.14 | Атомарные operation/link/retained snapshot/occupancy/reservations и освобождение unused quota | Локально PASS: ReplayAndUnknownCommit, QuotaRaceAndCleanup |
| 3.3.14 | Backend COMMIT выполнен, ответ потерян на DB protocol boundary; independent connection/retry разрешает тот же ID | Локально PASS: ReplayAndUnknownCommit; real COMMIT proxy |
| 3.3.15 | Cleanup/apply race; bounded batch; ровно однократное освобождение quota; retired ID не используется снова | Локально PASS: QuotaRaceAndCleanup, RetirementAndCorruption |
| 3.3.15 | Kill/restart сохраняет активный input/claims/резервы; нет фиктивного worker или state transition | Локально PASS: ProcessDeath |
| 3.3.15 | Безопасные terminal fixtures; очищены payloads, ownership отдельно; история очищается по прежней retention policy | Локально PASS: RetirementAndCorruption, OwnershipAndRollback |
| 3.3.15 | Старые каталог/история через real PostgreSQL/UDS, restart, SELECT-only grants | Локально PASS: Full suite; UDS narrow grants |
| 3.3.15 | Go build/test/race/vet/tidy/fmt/lint всех затронутых модулей; Linux build по общему набору | Локально PASS: Daemon checks.json; full/focused race; ctl checks.json |
| 3.3.15 | Итоговый OpenSpec strict и diff-check после реализации; соответствие всех пяти задач evidence | Локально PASS: strict.json issues=[]; diff-check PASS; пять задач закрыты после сверки evidence |
| 3.3.15 | CI на точных итоговых head SHA; независимое содержательное review | PASS: daemon 227e5d7 / run42030105874 SUCCESS; независимое Astra re-review без блокеров |

## Приложенное evidence и точные ревизии

- Daemon: `227e5d79ba07ea334ef9ee407d25148376599d29` (tests), предыдущий core
  commit `21b7e79`; рабочая копия daemon чистая на момент сбора evidence.
- Common: `6355d038884bec7452c77938e5045961cc8df58f` плюс текущие документальные
  правки одобрения/приёмки. `git diff --quiet 4bbf001 -- docs/api docs/oapi pkg/http`
  вернул 0: исходные и generated HTTP-контракты не изменились.
- Ctl: `d82da7994705f7080f2942076483c63d0deeeb80`; изменений этого этапа нет.
- [Полный race-набор](durable-admission-evidence/admission-full-race.log) прошёл
  до финального ужесточения validator. После него
  [финальный focused race](durable-admission-evidence/admission-final/admission-race.log)
  прошёл за 20.167s; включая exact-byte quota, отдельные процессы, process death
  и потерю подтверждения действительно выполненного COMMIT.
- [Daemon checks](durable-admission-evidence/admission-final/checks.json): build,
  vet, tidy, fmt, lint, diff — exit 0.
  [Ctl checks](durable-admission-evidence/ctl-stage-v4/ctl-checks.json): build,
  race, vet, tidy, fmt, lint — exit 0. Linux build подтверждён текущей Linux-средой.
- [UDS с узкими grants](durable-admission-evidence/ctl-stage-v4/daemon-uds-narrow-grants.log):
  предварительная проверка суженных grants на v3. Финальные v4 GET/restart,
  private SELECT denial, DML denial и POST404 подтверждены полным race-логом
  и daemon CI. Runtime SELECT ограничен каталогом, историей и Goose version. Startup
  не требует доступа к private payloads и не расширяет права.
- [OpenSpec strict](durable-admission-evidence/strict.json): PASS, `issues=[]`.
- [Версии и команды](durable-admission-evidence/environment.md),
  [SHA-256 исходников](durable-admission-evidence/source-sha256.json).
  Credentials и fixture URL в evidence не включены.

Для нормализации route claims daemon использует `golang.org/x/net/idna`:
существующая зависимость `golang.org/x/net v0.61.0` переведена из indirect в direct,
её версия не повышалась. Это не утверждение об отсутствии всех dependency alerts.

Независимое повторное Astra review после исправлений не выявило блокеров.
[Daemon CI](https://github.com/dkoshenkov/paas-daemon/actions/runs/42030105874)
на `227e5d79ba07ea334ef9ee407d25148376599d29` — SUCCESS: build/test/race/vet/tidy,
fmt/lint/govulncheck/actionlint и live PostgreSQL. [Machine-readable результат](durable-admission-evidence/daemon-ci.json).
Job dependency-review выводит явное предупреждение NOT RUN, не является выполненной
проверкой зависимостей. Документальные изменения common публикуются в
[draft PR #24](https://github.com/dkoshenkov/paas-common/pull/24), реализация —
[draft daemon #5](https://github.com/dkoshenkov/paas-daemon/pull/5). Merge не выполнен.

## Сохранённые ограничения

- Fixture planner не проверяет полный Kubernetes policy profile; production POST,
  CLI deploy, catalog writers, executor/recovery, k3s установка/end-to-end и TUI
  остаются отдельными этапами. Production writer credentials не выдавались.
- Cleanup/retirement вызывается только внутренними тестами, scheduler отсутствует.
  Безопасность внешнего terminal outcome не выводится из одного поля state.
- Постоянные ID tombstones и ownership не входят в квоты payload; WAL/MVCC/backup
  не очищаются этими методами. Строгая защита памяти от враждебного DB server не заявляется.
- Приёмка — исходный код и проверенное внутреннее поведение, без merge/deploy.
