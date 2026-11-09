# Локальная проверка release PR workflow

Из корня workspace:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 paas-common/tests/release-workflow/probe.py
PYTHONDONTWRITEBYTECODE=1 python3 paas-common/tests/release-workflow/real-git-probe.py
```

Нужны Bash, Python 3, Git и Go версии из `paas-common/go.mod`. При необходимости можно задать `GOCACHE`; иначе сквозной тест создаёт собственный кэш во временном каталоге. Оба сценария изолированы от GitHub. Первый исполняет точные shell-фрагменты трёх workflow с подменёнными `gh`, `git` и `go`: проверяет merge-base для отставшей обычной ветки, detached checkout, использование временного credential helper, сигнал release, pin/anchor, ошибки diff и смену SHA. Ошибка verifier для дополнительного файла в этом тесте моделируется.

Второй создаёт временные Git-репозитории, закреплённый источник verifier и синтетический release PR. Запускаются те же shell-фрагменты, настоящий `git` и `go run ./scripts/release-version --verify-pr`; `gh` подменён локальной фикстурой. Проверяется чтение PR-объектов без checkout, доверенный локальный main ref, проверка pin, якоря и итоговая повторная сверка. Репозитории и commit теста находятся только во временном каталоге. Проверки verifier с реальными Git-объектами, лишними файлами и некорректной историей находятся в `scripts/release-version/verify_test.go`.

Workflows применяют `pull_request_target` и проверяют default branch и актуальные SHA base/head и main через read-only API перед работой и после неё. Имена и labels PR являются только дополнительными сигналами кандидата; изменения защищённых release-файлов выявляются из tree diff от merge-base. Неизвестные дополнительные файлы проверяет verifier. PR tree не checkout и не исполняется. Внешнего release token нет: используется ограниченный `GITHUB_TOKEN` для API и однократного Git fetch, без сохранения credentials и без передачи токена `go run`.

Для `paas-ctl` и `paas-daemon` переменная репозитория `PAAS_RELEASE_TOOLS_SHA` должна указывать полный проверенный SHA опубликованного `dkoshenkov/paas-common`; workflow также требует достижимость SHA из `main` common. Достижимость сама по себе не заменяет review и выбор оператором. Пустое значение пропускается только для обычных PR. Для последующих выпусков `PAAS_RELEASE_BASE` содержит доверенное `version@40-hex-release-commit`; bootstrap и встроенная зафиксированная граница common 1.1.0 обрабатываются verifier. Обновление этого якоря после реального release относится к следующему этапу.

Это локальная подготовка. Согласно [документации события pull_request_target](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#pull_request_target), `GITHUB_SHA` относится к default branch (workflow требует, чтобы это был `main`); связь результата именно с проверенным PR head и enforcement через branch protection необходимо проверить в реальном GitHub. Локальные тесты не доказывают доступ к private repository, запуск Actions, политики секретов или настройки защиты. Существующий `paas-common/.github/workflows/release.yml` и reusable workflow не изменены и могут по-прежнему выполнять прежнюю публикационную логику; новый check сам её не отключает и не делает безопасной.
