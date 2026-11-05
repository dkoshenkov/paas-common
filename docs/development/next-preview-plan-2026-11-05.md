# Следующий этап: настоящий план манифестов

Статус: предложение для согласования, не разрешение на реализацию.
Исходная точка — common #24 (`b00f868`) и daemon #5 (`227e5d7`): оба
draft, не слиты. Документ не закрывает задачи OpenSpec и не меняет контракты.
Он уточняет следующие шаги change `define-k3s-paas-foundation` после 3.3.15.

## Предлагаемая последовательность

1. Согласовать профиль настоящего parser/normalizer/planner и проверить
   техническую возможность ограниченного разбора YAML.
2. Реализовать внутренний чистый planner от явного снимка состояния.
3. Отдельно согласовать и реализовать авторитетную регистрацию конфигурации,
   чтение Kubernetes, production preview, cleanup и команду CLI preview.
4. После проектирования и проверки executor/recovery подключить public apply
   и клиентский сценарий подтверждения. Приём операции без способа её надёжно
   выполнить не считать завершённым deploy API.

Каждый следующий этап требует согласования. Для интеграции используются
проверенные зависимости #24/#5; их merge этим предложением не разрешается.

## Ближайший полезный объём

Чистая функция получает исходный `PreviewRequest` и типизированный снимок
состояния. Возвращает диагностику, полный приватный план для сравнения и
безопасную публичную проекцию. Никаких HTTP handlers, SQL writes, Kubernetes
запросов или операций выполнения в этом этапе.

Поддерживаются уже согласованные Deployment apps/v1, Service v1 типа ClusterIP,
ConfigMap v1 и Ingress networking.k8s.io/v1; один пакет — один Deployment.
Не включённые в пакет прежние ресурсы не удаляются и учитываются при проверке
результирующего состояния. Изменение Deployment не должно оставлять прежние
Service/Ingress/ConfigMap в противоречивом состоянии.

Снимок содержит явно подтверждённые app/component/Namespace identity, policy
revision, стабильные instance IDs, ownership, резервы, существующие ресурсы,
домены, Secret bindings и сведения о миграторе/общих сервисах. Отсутствие объекта
и неизвестность его состояния различаются. Неполный или противоречивый снимок
не может давать применимый план. Fixture только проверяет алгоритм и не становится
источником production-прав. При отсутствии `metadata.namespace` planner заполняет
его Namespace приложения, как требует текущая спецификация.

Полный план сохраняет значения, действия, зависимости, symbolic IDs, результат
нормализации, предупреждения и дополнительные действия, включая настоящие
предпосылки INITIAL_MIGRATION. Публичная проекция не раскрывает manifest values,
env, credentials или приватные данные через сообщения и changed-fields.
Нельзя подменять отсутствие реализации миграции фиктивным NO_CHANGE.

## Решения до реализации

1. **Версии и YAML.** Закрепить совместимые Kubernetes/k3s schema/defaulting и
   зависимости. Зафиксировать поведение duplicate/unknown keys, tags, scalar
   types, пустых документов и недопустимых полей. Сохранить действующие пределы:
   8 MiB escaped JSON, 64 файла, 128 документов, глубина 64 и запрет aliases/merge.
   Ограничения глубины должны действовать во время разбора. Публичный Decoder
   go-yaml v3.0.5 строит Node tree до пользовательской проверки: последующий
   обход не доказывает эту границу. Первый технический gate — выбрать API с
   подходящими ограничениями и доказать поведение на глубоком/широком YAML.
   Самописный YAML parser и ослабление требований не предлагаются.
2. **Policy profile.** Утвердить таблицу field path → источник значения →
   заполнение/отказ/WARN; отдельно PodSpec host/privileged/volume возможности,
   ServiceAccount, токены и потребители Secrets. Не придумывать UID контейнера,
   служебные имена, Traefik параметры или права из пользовательского YAML.
3. **Снимок и сравнение.** Утвердить полноту и авторитетность каждого поля,
   stale/unknown semantics, сохранение объектов вне пакета, нормализацию maps/lists,
   absent/null/default и server-owned полей. Исключения из сравнения задаются
   точными путями; поле с названием timestamp само по себе не исключается.
4. **Диагностика и бюджеты.** Установить отдельные пределы количества/байтов
   диагностик, полного плана и ответа, детерминированный порядок, координаты
   ошибок и поведение при переполнении. Лимиты истории операций 32/256/64 KiB
   сюда автоматически не переносятся. Нельзя усечь ERROR и выдать успешный план.

После этих решений обновляются proposal/design/specs/tasks OpenSpec и выносятся
на утверждение до production-кода. Документ не объявляет выбранными конкретные
версии или численные бюджеты, которых в согласованном проекте пока нет.

## Следующий внешний preview

Для production preview требуется versioned каталог конфигурации в PostgreSQL
с узким административным writer, Go-validation и общим advisory lock. Регистрация
описывает уже подготовленную среду; не устанавливает k3s/Namespace/RBAC и не
присваивает чужие Kubernetes-объекты. Точный bootstrap и полномочия согласуются
отдельно. Временный отказ от миграторов/Secrets не считается согласованным
сокращением общего продукта.

Kubernetes adapter использует явные endpoint/CA/identity и ограниченные чтения;
не получает Secret data, exec или workload mutations ради preview. Server-side
dry-run требует тех же прав, что реальная мутация. Поэтому предлагается локальная
закреплённая schema/defaulting + PaaS policy, с явной границей: это не гарантия
прохождения произвольных admission webhooks или успешного выполнения.
Источник: [Kubernetes API Concepts](https://kubernetes.io/docs/reference/using-api/api-concepts/#dry-run-authorization).

DB snapshot/revision → ограниченные Kubernetes reads вне lock → локальный план →
общий lock и повторная проверка revision → сохранение preview. Все связанные
writers обновляют revision под тем же lock. Сеть не входит в нынешние 2 секунды
транзакции. Это не создаёт общей транзакции DB/Kubernetes: будущий apply повторяет
планирование, executor проверяет UID/resourceVersion и обрабатывает конфликт.
Несколько Kubernetes reads также не образуют единый snapshot; нужен явный
контракт полноты наблюдения и отказа при противоречивом результате. UID и
resourceVersion сохраняются для будущих guards даже там, где не участвуют
в смысловом сравнении плана.

HTTP preview требует отдельного production profile вместо fixture-v1,
ограниченных DB write grants, периодического cleanup после restart и согласованных
deadlines. При отдельном preview-only этапе generated registration обоих POST
не должна случайно открыть apply; внешний apply остаётся недоступным.

Предлагаемый клиент: `paas-ctl preview --host HOST --identity KEY --app-id ID
--component-id ID --path FILE_OR_DIRECTORY`. Читает один bounded snapshot,
передаёт исходный YAML, показывает безопасный план/диагностику/ID/expiry.
До кода согласовать рекурсивный отбор YAML, порядок файлов, symlink/FIFO policy,
race-resistant чтение и deadlines. Нет скрытого apply или автоматического retry.
Preview ID не означает согласие пользователя на выполнение. Будущий отдельный
apply после рестарта CLI потребует проверяемой связи с показанным планом.

## Выполнение и параллелизация

Public apply зависит от durable execution journal, worker ownership/fencing,
crash recovery между PostgreSQL и Kubernetes, неизвестного исхода мутации,
переходов операции и подтверждённого освобождения резервов. Внутренний
fixture-admission из #5 этих гарантий не реализует.

После утверждения ближайшего объёма common ведёт контракты профиля, OpenSpec
и общие test vectors; daemon — parser/planner и adversarial/property tests;
ctl — независимую проверку требований клиента. Код CLI нужен лишь на этапе
публичного preview: искусственно менять третий репозиторий сейчас не требуется.

Приёмка внутреннего planner: эквивалентные inputs дают одинаковый полный план;
значимые изменения inputs/state меняют сравнение; unknown state не даёт успех;
оставленные ресурсы проверяются; ошибки/overflow fail closed; публичная проекция
не раскрывает тестовые секреты; лимиты действуют до неограниченного построения
дерева. Реальные Kubernetes/HTTP/deploy проверки остаются отдельными этапами.

## Независимое архитектурное ревью

2026-11-05: отдельные исследования common/daemon/ctl и независимое ревью
Astra medium подтвердили полезность чистого внутреннего planner. Уточнения
ревью включены выше: сохранённые ресурсы вне пакета, absent/unknown state,
заполнение отсутствующего namespace, сохранение execution guards и отсутствие
единого Kubernetes snapshot. Перечисленные профили и parser gate остаются
блокерами реализации, а не считаются уже одобренными решениями. Ревью документа
не является проверкой работающего Kubernetes planner.

## Конкретные рекомендации по четырём вопросам — 2026-11-05

Это уточнение предложения, а не согласование или реализация. Проверены текущие
go.mod, SDK validators и fixture store. В common YAML v3.0.4 indirect,
в daemon v3.0.5 indirect; Go проекта — 1.27.3. Kubernetes dependencies и
production version pin пока отсутствуют. Следующие значения — рекомендуемый
первый профиль, а не результаты нагрузочных испытаний.

### 1. YAML: контролируемое построение дерева

Рекомендация: проверить `go.yaml.in/yaml/v4 v4.0.0` как кандидата, сохраняя
инженерный gate до его включения. Это проверенный upstream tag от 2026-11-04,
commit `f2f9563503f11b7a60da0c532175dcc693ce8d5a`, ещё очень свежий релиз.

| Вариант | Что установлено | Компромисс |
| --- | --- | --- |
| v3.0.5 без изменений | Decode строит Node tree до пользовательского обхода | Меньше изменений зависимостей, но не обеспечивает раннюю точную глубину 64 |
| goccy/go-yaml v1.19.2 | Tokenization перед AST; нужный ранний depth/node budget в проверенном API не найден | Другой parser не снимает блокер автоматически |
| v4.0.0 + недостающий upstream hook | Есть plugins для loader limits и aliases; нет доказанной общей глубины/node budget | Предпочтительный путь на Go, но требуется проверка/доработка upstream |
| libyaml 0.2.5 event API | Позволяет контролировать построение дерева по событиям | cgo, C toolchain, управление памятью и дополнительный аудит; не первый выбор |

У v4 `loaderlimits.DepthValue(64)` контролирует flow/block счётчики раздельно;
это не доказательство совокупной глубины mapping/sequence, особенно с indentless
sequences. `aliasdata.WithDisable(true)` запрещает aliases; `AliasNone()` у
loaderlimits отключает проверку, а не aliases. `mergekey.WithDisable(true)`
отключает merging и само по себе не означает отказ на merge-синтаксис.

Нужен hook перед созданием очередного узла: общий depth, node count,
document count, отказ на alias/merge до expansion. Предпочтительно принять его
upstream. Если понадобится временный fork, вынести конкретный diff и срок/план
сопровождения на отдельное решение; сейчас fork не предлагается считать разрешённым.
Не заменять это самописным YAML parser или проверкой после полного Decode.

Предлагаемый входной профиль: строковые mapping keys; дубликаты запрещены;
JSON-совместимые scalar values; YAML 1.2 Core resolution с явным отказом для
непредставимых JSON чисел (NaN/Inf), нестандартных tags и non-string keys.
`true/false` — boolean, `yes/no/on/off` — строки; строковые значения для полей
вроде env.value нужно заключать в кавычки, если они выглядят как число/boolean.
Нет автоматического stringification, timestamp/object construction и merge.
Пустые документы учитываются в 128, но не создают ресурс; пакет без Deployment
не проходит уже существующее правило. `<<` как запрещённый merge key отклоняется
независимо от кавычек: это предлагаемое явное ограничение совместимости.

Источники: [v3 Decode](https://github.com/yaml/go-yaml/blob/v3.0.5/yaml.go),
[goccy parser](https://github.com/goccy/go-yaml/blob/v1.19.2/parser/parser.go),
[v4 plugins](https://github.com/yaml/go-yaml/blob/v4.0.0/plugin.go),
[v4 limits](https://github.com/yaml/go-yaml/blob/v4.0.0/plugin/loaderlimits/plugin.go),
[v4 scanner](https://github.com/yaml/go-yaml/blob/v4.0.0/internal/libyaml/scanner.go),
[libyaml events](https://github.com/yaml/libyaml/blob/0.2.5/include/yaml.h),
[YAML 1.2.2](https://yaml.org/spec/1.2.2/).

**Что согласовать:** scalar/duplicate/tag profile как поведение продукта.
Выбор проверенной библиотеки и тестов — инженерная работа; пользователь не должен
выбирать библиотеку вместо доказательства её пригодности. Fork/cgo — отдельное
решение только при необходимости. Ранние 64/128 и запрет aliases/merge уже заданы.

### 2. Kubernetes и защищённые поля

Рекомендованный pin: **k3s v1.37.2+k3s1 / upstream schema v1.37.2 /
Restricted v1.37 / k8s.io/api, apimachinery, pod-security-admission v0.37.2**.
Официальные stable/latest каналы на дату проверки указывают эту k3s версию;
альтернатива из канала v1.36 — v1.36.6+k3s1 с соответствующим полным набором
schema/policy/dependencies. 1.36 даёт больше времени эксплуатации ветки, но
увеличивает отставание; для ещё не установленного продукта рекомендую stable.
Не использовать плавающие latest при сборке или policy evaluation.
Минимум Go для проверенного k8s.io/api — 1.26.0, ниже проектного 1.27.3;
полный dependency graph и совместимость сборки пока не проверялись.

Локальная проверка: закреплённые OpenAPI v3 schemas + PaaS semantic policies +
версионированный Restricted evaluator на PodTemplate. Не тянуть
`k8s.io/kubernetes` ради server defaulting. `k8s.io/api` даёт типы, но не все
server defaults. OpenAPI validator и Kubernetes extensions требуют compatibility
gate. Deployment dry-run не доказывает допуск будущего Pod: PSA enforce относится
к Pods, а workload templates проверяются для audit/warn.

| Поле/правило | Рекомендация | Статус решения |
| --- | --- | --- |
| Namespace, identity labels/selectors | Заполнять отсутствующее по авторитетному профилю; противоречие ERROR | Уже согласовано |
| managed-by | Исправление с WARN только после ownership check | Уже согласовано |
| ServiceAccount, automount, Secret consumers, imagePullSecrets | Ожидаемый SA; automount=false; запреты/проверки из текущих specs | Уже согласовано |
| Ingress/domain/class/TLS/annotations | Только зарегистрированные значения и прежние host/path правила | Уже согласовано; конкретные значения из профиля |
| Restricted | Проверять v1.37; недостающий обязательный securityContext → ERROR, без выдуманного UID | Restricted уже обязателен; версия и поведение заполнения предлагаются |
| Unknown fields | ERROR по структурной схеме; допустимые arbitrary-key maps сохраняются | Новый строгий профиль |
| status и metadata.uid/resourceVersion/generation/managedFields/creationTimestamp/deletionTimestamp/deletionGracePeriodSeconds | ERROR при наличии во входе, включая null/пустое значение; не удалять молча | Новая policy для пользовательского пакета, не snapshot |
| metadata.ownerReferences/finalizers | ERROR при наличии во входе, включая пустой массив | Новая policy; упрощает владение/удаление, ограничивает импорт чужих manifests |
| nodeName, runtimeClassName, schedulerName, priorityClassName | Только явно разрешённые профилем значения; отсутствие не заполнять выдуманным значением | Новая policy; список разрешений задаётся административно |

Не запрещать все annotations, volumes, init/sidecars, affinity или tolerations
ради упрощения. Проверять их текущими PaaS правилами, schema и Restricted.
`readOnlyRootFilesystem=true` не навязывать как новое скрытое требование.
Explicit null не приравнивать к отсутствию: принимать только где schema и
конкретное PaaS правило разрешают null; сохранять происхождение значения.

Defaults задаются перечислением: согласованные PaaS identity/Namespace/SA/token
и Ingress corrections; Service type ClusterIP предлагается заполнять при пропуске.
Replicas/rollout/imagePullPolicy/Service allocation автоматически не вычислять.
Для полного сравнения различать omitted intent, explicit value и наблюдаемый
server value. Пропуск поля не доказывает NO_CHANGE без определённой семантики
обновления этого поля; неизвестный результат не выдаёт применимого плана.
Точный per-field update/defaulting registry остаётся техническим gate перед
готовностью полного planner, а не обещанием полного клона API server.

Источники: [каналы k3s](https://update.k3s.io/v1-release/channels),
[k3s release](https://github.com/k3s-io/k3s/releases/tag/v1.37.2%2Bk3s1),
[k8s api go.mod](https://github.com/kubernetes/api/blob/v0.37.2/go.mod),
[schema artifacts](https://github.com/kubernetes/kubernetes/tree/v1.37.2/api/openapi-spec/v3),
[Restricted evaluator](https://github.com/kubernetes/pod-security-admission/blob/v0.37.2/policy/registry.go),
[PSA workload templates](https://kubernetes.io/docs/concepts/security/pod-security-admission/#workload-resources-and-pod-templates).

**Что согласовать:** pin, новые строки policy, отсутствие неоговорённых
автоисправлений и границу локальной проверки. Альтернатива server dry-run лучше
отражает кластерные admission/defaults, но требует mutation rights и сети;
для pure/read-only этапа не подходит. Restricted и ранее согласованные
identity/secret/route правила заново выбирать не нужно.

### 3. Снимок состояния и сравнение

Рекомендация: версионированный `PlannerStateV1` с отдельными разделами target,
policy, ownership/reservations, resource observations и migration prerequisites.
Каждый раздел имеет область полноты и состояние complete/unknown; отсутствие
конкретного объекта представляется absent только внутри подтверждённо полного
набора либо по авторитетному exact lookup. Пустой slice без coverage не означает
«объектов нет». Снимок не содержит Secret values; только разрешённые ссылки и
версионированные metadata, необходимые для решений.

Resource observations включают собственные и чужие объекты, необходимые для
проверки имени, selectors и маршрутов; retained resources участвуют в итоговой
проверке. Нет скрытого DELETE. В полном приватном плане фиксируются intent,
сравниваемые current/target поля, зависимости, claims, warnings, policy version
и дополнительные действия. Snapshot revision — token проверки свежести, не
подмена содержательного сравнения. Policy revision change требует перечитать
и перепланировать; нерелевантное глобальное изменение не должно само становиться
изменением бизнесового плана.

Canonical profile v1: ключи maps упорядочены; порядок lists сохраняется, включая
schema set/map lists. Это может консервативно потребовать новый preview при
перестановке эквивалентных элементов, но не скрывает изменение порядка.
Отсутствие, null и explicit value различаются; целые числа не проходят float64.
UID существующего объекта участвует в идентичности/сравнении; resourceVersion
сохраняется для execution guard, исключаясь из смыслового сравнения по текущему
spec. Исключения status/observation timestamps задаются точными путями.

Альтернатива schema-aware сортировки list-map/set уменьшит лишние re-preview,
но требует корректной обработки Kubernetes extensions и отдельного профиля;
не рекомендую добавлять эту сложность в первый шаг. Opaque map/JSON snapshot
проще начать, но затрудняет доказательство полноты и случайно смешивает unknown
с absent; не рекомендую.

Для будущего adapter: полный paginated List имеет единый resourceVersion внутри
одной коллекции; разные GVK/Namespace не образуют общей транзакции. Неполная
страница, 403, timeout, 410 без полного повторного чтения — unknown, не absent.
Не перечислять только PaaS-labelled объекты, если это скрывает чужой конфликт.
Нет повторов внутри pure planner и нет сетевых запросов под DB lock.
Источник: [Kubernetes list consistency](https://kubernetes.io/docs/reference/using-api/api-concepts/#retrieving-large-results-sets-in-chunks).

**Что согласовать:** сохранение порядка lists как консервативный первый профиль
и per-field семантику omitted values. Полнота, fail-closed, ownership, retained
resources и повторная проверка перед apply уже следуют из действующих требований.
Конкретные Go-типы и разделение adapters — инженерные решения.

### 4. Предлагаемые бюджеты первого профиля

Все значения ниже новые, кроме явно обозначенных существующих. MiB/KiB бинарные.
Это логические пределы, не гарантия RAM/RSS; benchmark/fuzz до runtime обязателен.

| Объект | Рекомендуемый предел | Обоснование |
| --- | --- | --- |
| Input/files/documents/depth/name | Существующие 8 MiB / 64 / 128 / 64 / 512 UTF-8 bytes | Не ослаблять и не переносить лимиты history |
| Узлы YAML на весь пакет | 262 144 | Отдельно ограничивает много мелких элементов внутри 8 MiB |
| Diagnostics | 128 записей и 256 KiB сериализованного массива | Достаточно для нескольких файлов; не бесконечный error dump |
| Diagnostic code/message/field_path | 64 / 1024 / 1024 UTF-8 bytes | Безопасные коды/шаблоны; file сохраняет разрешённые 512 bytes |
| Public PreviewResponse | 1 MiB фактического JSON response body | План без values существенно меньше входного пакета |
| Полный private plan | 32 MiB canonical JSON, 524 288 JSON nodes | Место для current/target, зависимостей и claims при 8 MiB input |
| PlannerStateV1 | 32 MiB canonical JSON, 262 144 JSON nodes, 4096 resource observations | Явный предел также для большого существующего окружения |
| Claims production profile | 4096 resources / 4096 routes / 2 MiB canonical JSON | Те же численные ceilings, что fixture; отдельная production validation |

YAML node — каждый scalar (включая mapping key), mapping и sequence; document
wrapper не узел, но документы считаются отдельно. Счётчики общие на все файлы.
JSON node — object, array или scalar, плюс отдельный узел каждого object key;
это самостоятельные счётчики state и plan, а не лимит только входного YAML.
Для source и snapshot нельзя сначала построить неограниченную коллекцию, а
потом измерить её: лимиты проверяются при чтении/создании/добавлении элементов.
Snapshot size включает все разделы/metadata, resource count — все GVK вместе,
включая чужие и retained observations; превышение не допускает частичный snapshot.
Private plan size включает current/target, claims, auxiliary data и профили.
JSON budgets включают escaping и структурные bytes; wire response — также
пробелы/newline фактического encoder. Canonical storage без форматирующих пробелов.

При overflow diagnostics заменить результат одним гарантированно небольшим
ERROR `DIAGNOSTIC_LIMIT_EXCEEDED`; plan/preview_id/expiry отсутствуют. Никакого
успешного усечения warnings. Ошибки parser/schema выводятся только безопасными
шаблонами, не через Error() с исходными значениями. Публичные field paths не
раскрывают arbitrary map keys: например, `/data` вместо пользовательского ключа;
точный приватный diff сохраняется. Для UPDATE показывать реально изменённого
родителя, а не выдуманные leaf paths.

Будущий HTTP mapping: input/node/diagnostic complexity errors → обычная
неприменимая PreviewResponse с ERROR (HTTP 200 по текущему content-error контракту);
неполный/слишком большой state или вычисленный plan/response, который нельзя
полностью представить → 503 ErrorResponse с новым кодом `PLANNER_LIMIT_EXCEEDED`
или `STATE_INCOMPLETE`. Body >8 MiB остаётся 413. Сохранение сверх существующей
квоты остаётся 503 PREVIEW_CAPACITY_EXCEEDED. Внутренний этап возвращает typed
результат/ошибку и не регистрирует HTTP.

Квоты unused 64/256 MiB и active 16/256 MiB не увеличивать автоматически.
При input=8 MiB, plan=32 MiB, diagnostics=0.25 MiB квота 256 MiB вмещает только
шесть максимальных payloads: 64 и 16 — потолки количества, а не обещание ёмкости.
Текущий fixture quota не включает отдельный claims JSON (до 2 MiB на запись),
индексы/TOAST/WAL и память; не объявлять его total-storage budget. Production
profile должен учитывать смысловую копию claims внутри private plan и явно
документировать дополнительную физическую копию, если она остаётся.
Не переиспользовать fixture validator имён 96 bytes для Kubernetes names до 253.

Альтернативы: 64 KiB diagnostics/history-style слишком рано обрезает полезные
multi-file ошибки; 4 MiB public response и 64 MiB plan снижают частоту отказов,
но увеличивают память и уменьшают число сохранённых preview. Рекомендованные
1/32 MiB — стартовый компромисс, подлежащий проверке на реалистичных fixtures.

**Что согласовать:** новые численные ограничения и пользовательское поведение
при переполнении. Конкретные счетчики/streaming implementation — инженерные
детали. Неизменность квот, отсутствие скрытого усечения и утечек уже требуются.

### Что действительно требуется от пользователя

Можно одним решением утвердить рекомендуемый **внутренний** профиль: версии,
новые входные policy из таблицы, консервативное сравнение и бюджеты. Это не
разрешает public preview/apply, bootstrap или deployment. Инженерные gates YAML
и schema/defaulting должны быть доказаны до заявления готовности полного planner.
Если необходим fork/cgo или обнаружится несовместимое ограничение, представить
конкретный вариант отдельно. Выбирать новую YAML-библиотеку вслепую, заново
разрешать Restricted либо подтверждать уже заданные input limits не требуется.

Независимое ревью Astra этого уточнения потребовало и учтено: arbitrary-key maps
не являются unknown fields; input metadata policy не применяется к observations;
scope полноты коллекций обязателен; порядок lists не обещает эквивалентность любых
перестановок; budgets проверяются до материализации. Код не менялся и версии
не устанавливались, runtime/conformance/нагрузочные проверки не выполнялись.
