# Предложение v14: журнал допуска Traefik к backend

Владелец одобрил Goose v14 и реализацию 8 ноября 2026 года. Схема v13 уже применена на стенде;
изменять её задним числом нельзя. Это дополнение к [согласованной архитектуре
Ingress → backend](traefik-backend-network-proposal-2026-11-08.md).

## Почему новая версия

Таблица v13 `component_network_intents` технически допускает отдельный префикс
ключа, и установщик распознаёт её intent по namespace/name/UID. Но старый
daemon v13 не знает такую связь с Ingress и не будет повторно отзывать policy
после изменения backend или потери источника. Штатный `schemaVersion` старого
daemon отклоняет версию выше 13. Отдельная v14 даёт явную границу совместимости,
а отдельная таблица — читаемую связь без перегрузки service grant.

**Schema gate не удаляет уже действующие NetworkPolicy.** После невозможности
запустить новый daemon последнее фактическое правило может остаться на узле.
Контролируемый откат требует сначала отозвать все эти правила и подтвердить
пустой spec по их UID либо запретить downgrade. Нельзя считать отказ readiness
доказательством отзыва. При недоступности API/БД откат и смена сетевого профиля
ждут восстановления управления.

## DDL

Новая миграция `00014_ingress_network.sql`, владелец схемы — существующий
мигратор. `spec` — JSON Kubernetes `NetworkPolicySpec`, без секретов; поля
проверяет приложение. Одна запись соответствует Ingress и конкретному instance;
разные Ingress с одним backend имеют независимые записи.

```sql
CREATE TABLE paas_execution.ingress_network_intents (
    app_id text COLLATE "C" NOT NULL,
    component_id text COLLATE "C" NOT NULL,
    ingress_name text NOT NULL,
    instance_id text COLLATE "C" NOT NULL,
    namespace_name text NOT NULL,
    namespace_uid text NOT NULL,
    controller_uid text NOT NULL,
    ingress_uid text,
    service_name text NOT NULL,
    service_uid text,
    deployment_uid text,
    target_port integer NOT NULL,
    operation_id text COLLATE "C" NOT NULL,
    desired_revision text NOT NULL,
    desired_allow boolean NOT NULL,
    intent_id text NOT NULL,
    policy_name text NOT NULL,
    spec text NOT NULL,
    object_uid text,
    resource_version text,
    confirmed_intent_id text,
    PRIMARY KEY (app_id, component_id, ingress_name, instance_id),
    UNIQUE (namespace_name, policy_name)
);
CREATE INDEX ingress_network_intents_namespace
    ON paas_execution.ingress_network_intents (namespace_name);
```

`controller_uid` — UID Namespace `kube-system`, привязанный установкой к
проверенной метке Pod Traefik; это не UID Pod. `service_uid`, `deployment_uid`
и `ingress_uid` могут отсутствовать только до подтверждения соответствующего
создания; до выдачи allow обязательны UID Service и Deployment, после записи
маршрута — также UID Ingress. `desired_revision` связывает принятый план,
Service/Deployment instance, targetPort и controller UID; `intent_id` меняется
при новом действии. Подтверждение возможно только при точном совпадении
`confirmed_intent_id`, UID, resourceVersion и прочитанного spec/fence. На
каждом этапе функции store проверяют reservation, актуальную revision и
ownership в одной транзакции под существующей общей сетевой блокировкой.

## Минимальные права

При установке v14 использовать существующие роли; секреты и kube-system RBAC
daemon не расширять:

```sql
REVOKE ALL ON paas_execution.ingress_network_intents
    FROM paas_runtime, paas_settings_writer, paas_operation_writer, paas_namespace_preparer;
GRANT SELECT, INSERT ON paas_execution.ingress_network_intents TO paas_operation_writer;
GRANT UPDATE (ingress_uid, service_name, service_uid, deployment_uid, target_port, operation_id,
              controller_uid, desired_revision, desired_allow, intent_id, spec,
              object_uid, resource_version, confirmed_intent_id)
    ON paas_execution.ingress_network_intents TO paas_operation_writer;
GRANT SELECT (namespace_name, namespace_uid, intent_id, policy_name, object_uid)
    ON paas_execution.ingress_network_intents TO paas_namespace_preparer;
```

Preparer при проверке управляемых NetworkPolicy учитывает эту таблицу как
`ingress`, с теми же проверками namespace UID и intent, что и текущий v13.
Обновление readiness проверяет наличие таблицы и каждого нужного права;
обычный runtime reader не получает к ней доступ. Отдельное право `DELETE` не
предлагается: запись и пустую policy сохраняем для восстановления и аудита.
Для обновлений одной и той же пары Ingress/instance используется одна строка.
Как и для существующего журнала принадлежности ресурсов, у записей нет общего
срока хранения. Ограничение на число файлов одного запроса не ограничивает
суммарное число записей. Имя и UID нового Service разрешено сохранять
только после подтверждённого отзыва прежнего допуска.

## Переход и исполнение

- Мигратор выполняет v13 → v14 один раз, не пересоздаёт Namespace, приложения,
  существующие Deployment, Service и Ingress. Существующие ресурсы не получают
  сетевое разрешение автоматически. Новый исполнитель добавляет его после
  проверки сохранённого Ingress и backend.
- До миграции проверяется отсутствие незавершённых deployment-операций, чьи
  планы не содержат v14 intent: их нельзя молча продолжить по старому порядку.
  Для ранее принятых и неподтверждённых действий требуется завершение или
  явное восстановление. Установщик проверяет роли и v14 до запуска daemon.
- Preview фиксирует проверяемые доступные параметры; apply перепроверяет их,
  создаёт Deployment и Service, ждёт готовых EndpointSlice и Pod, записывает
  intent, создаёт пустую policy и привязывает UID до allow. Затем применяет
  Ingress, фиксирует его UID и повторно проверяет обе стороны.
- Изменение Service или Ingress сначала подтверждает deny старого intent;
  затем проверяет новый backend и только после нового allow меняет маршрут.
  Сохранённые Ingress вне пакета также учитываются. Если ответ потерян между
  созданием пустой policy и записью UID, восстановление принимает только
  объект с точным именем, namespace UID, intent annotation и пустым spec.
  Любой чужой UID/fence остаётся `RECOVERY_REQUIRED`.
- На повторе после успешного `BeginAction` исполнитель читает записанный UID и
  journal, а не создаёт заново. `ownership` остаётся источником принадлежности
  Ingress, Service и Deployment после истечения сохранённого preview; новый
  journal хранит собственный spec и identity независимо от его срока.
- Сохранённые v13 Ingress до первого v14 reconcile остаются без новой policy;
  они не должны считаться проверенно доступными, пока её UID и spec не прочитаны.
  Если API недоступен, состояние не считается подтверждённым и новый allow не
  создаётся.

Нужны интеграционные тесты v13→v14 и прав всех ролей; запрета запуска v13 на
v14; потери ответа после каждого шага; гонки изменения targetPort/Service UID;
двух Ingress с общим backend; отдельного отзыва и отсутствия допуска к другому
порту или instance. Затем — фактический HTTPS и негативные пробы на согласованной
k3s VM. Unit/fake тест не доказывает dataplane.
