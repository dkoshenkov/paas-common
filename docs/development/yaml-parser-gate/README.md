# YAML parser gate — BLOCKED

Изолированный reproducer, без изменений daemon и upstream. Go 1.27.3,
публичные API go.yaml.in/yaml/v4 v4.0.0. Полный разбор результата и следующие
решения: [отчёт](../parser-planner-gate-2026-11-05.md).

При доступном Go запустить из этого каталога:

```sh
./run.sh
```

В текущем executor сначала выполнить `source /workspace/.paas-env/activate`.
Скрипт закрепляет GOTOOLCHAIN=go1.27.3, проверяет модули, запускает контрпримеры
с race detector, затем обязательный контрактный тест. Ожидаемый код завершения
**1**: ранняя точная глубина 64 не обеспечена. PASS контрпримеров означает
воспроизведение нарушения, а не приёмку parser. Логи — gate.log и verification.log.

Получены деревья глубины 65, 128 и 66 при DepthValue(64). Alias disable работает;
merge disable не отклоняет merge key. Document callback останавливает документ129
до глубоко вложенного тела. Широкий документ строит300003 узла при одном depth
callback;262144 — только предложенный, не утверждённый дополнительный предел.
HTTP reader отдельно проверен на8388608/8388609 bytes до JSON decoding.

module-origin.json записывает Git origin и commit
f2f9563503f11b7a60da0c532175dcc693ce8d5a; go.sum — checksum исполняемой зависимости.
module.json содержит сведения локального module cache. SHA256SUMS проверяется
командой `sha256sum -c SHA256SUMS` из этого каталога.

Источники:
- https://github.com/yaml/go-yaml/blob/v4.0.0/internal/libyaml/scanner.go
- https://github.com/yaml/go-yaml/blob/v4.0.0/internal/libyaml/composer.go
- https://github.com/yaml/go-yaml/blob/v4.0.0/plugin/loaderlimits/plugin.go
- https://github.com/yaml/go-yaml/blob/v4.0.0/plugin/aliasdata/plugin.go
- https://github.com/yaml/go-yaml/blob/v4.0.0/plugin/mergekey/plugin.go

Нужен ранний unified structural-depth hook; проверка готового Node tree поздняя.
Fork/cgo или полная замена parser требуют отдельного решения и здесь не сделаны.
