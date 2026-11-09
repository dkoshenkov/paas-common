# Локальная проверка release-please CLI

Из корня `paas-common`:

```sh
mkdir -p /private/tmp/paas-release-pinned
cp tests/release-please-cli/package.json tests/release-please-cli/package-lock.json /private/tmp/paas-release-pinned/
npm ci --prefix /private/tmp/paas-release-pinned --no-audit --no-fund
RELEASE_PLEASE_CLI=/private/tmp/paas-release-pinned/node_modules/.bin/release-please python3 tests/release-please-cli/probe.py
```

`npm ci` получает зафиксированный в lockfile `release-please@17.11.2` и зависимости из npm. Для запуска fixture нужен локальный TCP listener. Тест передаёт CLI только адреса локальной имитации GitHub и фиктивный токен; запросы к GitHub, GitHub Actions и реальные записи Git не выполняются. Используется `release-type: go` и пути `configs/release-please-config.json` и `configs/.release-please-manifest.json`, как в common. Для ctl/daemon bootstrap-версия и конфигурация ещё не согласованы: результаты этой go-фикстуры не подтверждают их фактическую конфигурацию.

В прогоне 8 ноября 2026 `npm ci` и тест завершились с кодом 0, stderr обоих был пуст. Первые семь сценариев — настоящий CLI в `--dry-run`; проверяются число предлагаемых PR и содержимое итогового manifest. Для fix `1.1.0 → 1.1.1` и bootstrap с пустыми тегами/release, `bootstrap-sha` и явным `1.0.0` создаётся по одному предложению. В фикстуре коммит за границей release исключается. Без найденного SHA границы CLI всё равно предлагает PR и включает более старый коммит; отдельная проверка полноты истории обязательна.

С фиксированным `--release-as 1.2.0` стандартный changelog пропускает `chore` и CLI предлагает 0 PR. Явная секция `{type: chore, hidden: false}` даёт 1 PR. Для нестандартного `other:` поведение аналогично: PR появляется только после явного включения именно этого типа. Полностью неконвенциональный `update dependency metadata` даёт 0 PR даже при включённых секциях: парсер рассматривает 0 коммитов. Это не соответствует требованию minor для каждого неизвестного пользовательского изменения и не устраняется конечным списком типов.

Отдельный **не dry-run** сценарий выполняет реальное создание и повторный запуск CLI против stateful mock: имитация приняла один PR и один tree с manifest `1.1.1`; второй запуск распознал неизменённый PR, не создал второй PR или tree. Это не подтверждает GitHub Actions, реальный GitHub API, разрешения, параллельные запуски или все варианты восстановления после частичного сбоя. В частности, fixture не доказывает публикацию тега/release и не реализует проверку 2.x — эти ограничения относятся к внешнему gate.
