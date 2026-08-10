# Reusable workflows

Этот репозиторий содержит reusable workflows. Автоматические triggers здесь не определены: workflow запускается caller-файлом в repository-потребителе.

## Инициализация repository-потребителя

После создания репозитория на основе этого шаблона запусти из его корня:

```bash
bash scripts/init-workflows.sh
```

Скрипт создаст callers для Go quality, workflow lint, dependency review, CodeQL и Release Please, а также конфигурацию Release Please в `configs/`.

Существующие файлы не перезаписываются. Для явного перезаписывания используй `--force`.

По умолчанию reusable workflows вызываются локально:

```yaml
uses: ./.github/workflows/go-quality-reusable.yml
```

Для центрального automation-репозитория:

```bash
bash scripts/init-workflows.sh \
  --mode central \
  --automation-repo dkoshenkov/go-repo-template \
  --ref v1
```

В этом режиме caller ссылается на tag или branch центрального репозитория.

## Secrets и permissions

GitHub repository variables для базовой конфигурации не нужны. Стабильные значения зафиксированы в YAML и generated callers.

Для Release Please нужен только secret `RELEASE_PLEASE_TOKEN`. Caller передаёт его в reusable workflow через `secrets.token`.

Reusable workflows:

- `go-quality-reusable.yml` — `go mod tidy`, tests, build, race detector, `go vet`, `gofumpt`, `gci`, `golangci-lint`, `govulncheck`;
- `workflow-lint-reusable.yml` — `actionlint`;
- `dependency-review-reusable.yml` — проверка новых зависимостей в pull request;
- `codeql-reusable.yml` — CodeQL Code Scanning;
- `release-reusable.yml` — Release Please.

Deployment workflows требуют отдельного target-specific reusable workflow и GitHub Environment. Универсальный workflow с произвольной shell-командой через input не используется.
