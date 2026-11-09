# Приёмка чтения истории операций, 7 ноября 2026

Закрыты 3.3.6–3.3.10. Тестовый клиент читает сохранённую операцию из
PostgreSQL через файловый Unix-сокет до и после перезапуска процесса daemon.
Записи создаёт DB fixture. Приёмка исполнения деплоя и восстановления после
прерванного apply относится к общему пункту 3.3.

| Проверка | Что подтвердили |
| --- | --- |
| Переход схемы | Тест на v2 сначала воспроизвёл отсутствие схемы v3. Миграция сохранила видимый и логически удалённый каталог. Повторные и конкурентные миграции прошли. Runtime читает историю, попытка INSERT отклонена. Отзыв SELECT делает control plane неготовым. |
| Контракт снимка | Проверены длины полей в UTF-8 байтах, управляющие символы, 32 диагностики, временные отметки, состояния, JSON escaping. Для снимка и DTO проверена граница 64 KiB, для полного JSON страницы — 8 MiB. |
| Пагинация | На 105 записях проверены все пять состояний, порядок ID, фильтр, удаление позиции, изменение limit, курсор другого списка и дубль в lookahead. Отдельный обход через generated client включает кириллицу и продолжение после tombstone. |
| Целостность и ошибки | NULL, несовпадающие колонки и snapshot, повреждённый JSON, дубли ключей и неверный Unicode отклонены. Через UDS проверены 200, 400, 404, 503 и 500 с безопасным телом. Контрольная строка из повреждённой записи в ответ не попала. |
| Перезапуск | Отдельный процесс повторно открыл БД. После чтения и перезапуска снимок таблиц каталога, истории операций и Goose совпал побайтно с исходным. |
| Модули | Go 1.27.3. Для common, ctl и daemon прошли build, test, race, vet, tidy-diff, fmt и golangci-lint v2.15.0. Бинарники ctl/daemon собраны для linux/amd64. Оба прогона daemon использовали PostgreSQL 18.6 и выполнили DB-тесты. |
| SDK | Все 58 прежних файлов в docs/api, docs/oapi, docs/proto, pkg/grpc и pkg/http побайтно совпали с HEAD paas-common `c69ceee25606687745fc937f13b34e5ef1d21244`. Новый validator подключается явно. |
| ub-mode | Для трёх новых production-файлов history.go максимальный CRAP равен 7,0. Mutator обнаружил пропуски проверки отдельных timestamps, версии курсора и последней полной страницы. После исправлений тесты убили 27/27 мутаций валидатора, 46/46 хранилища и 5/5 HTTP-сервиса. Dryer при пороге 0,8 нашёл три пары только среди прежних SDK validators. |

## Повтор проверки

В `paas-daemon` запустить обе команды с доступным Docker и Unix-сокетами.

```sh
sh scripts/test-postgres.sh -count=1 -v
sh scripts/test-postgres.sh -count=1 -race -v
```

Скрипт выводит версию PostgreSQL и digest образа, создаёт временный контейнер
и удаляет его после тестов. В `paas-common` проверить change командой
`openspec validate define-k3s-paas-foundation --strict`.
Остальные команды для каждого модуля приведены в его README.

## SHA-256

Digest образа PostgreSQL — `sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873`.
SHA-256 списка 58 прежних SDK-файлов — `b788c7a3d4b0201fb7bf2c9b223b12a61c5d1ec692650e646aecc1bc007d3410`.
Список содержит по строке `<sha256>  <путь от корня paas-common>` в порядке
`git ls-tree -r --name-only`, с LF после каждой строки.

Контрольные суммы проверенных исходников, тестов и миграции приведены ниже.
Пути отсчитываются от корня рабочего пространства.

```text
c3981fc3b93488451c3abd99db1f483f8e57d15f1017d76aeee6d293e0896fee  paas-common/pkg/http/paas/validation/history.go
05add269e63962062a26c59a0d8c5857ec5ac691f25c689dfd98c720e94e1150  paas-common/pkg/http/paas/validation/history_test.go
484f571b347e34ab3638224861e0c7b1a291f87a374623e64a26bb9f71157433  paas-daemon/internal/daemon/history.go
118c5c9e7de83c49dff8ea230637bd05d5f42eb4d05431d6cfd707f5e19f4631  paas-daemon/internal/daemon/postgres_test.go
5093ad00657b83b2254bf40ad64ece5f58b5eecfbcaa84cf392a904970dad490  paas-daemon/internal/daemon/server.go
b7bee52f9a3644e68e2bf74fa9a06e70b368888ca44aac08f83cffcdeeafcac5  paas-daemon/internal/store/history.go
ee402754205b22bef0fcf47f0d90974b8ce826761cff600b2e4b38b5d983a7c5  paas-daemon/internal/store/history_test.go
52b45052291a0da147a47719f73a1f766d99c73de52e3cd4c9dc5dfe949a5b21  paas-daemon/internal/store/integration_test.go
ce3dbdd974c49f8ce70be8b736265de49fc4af4e600b11d36e7e318fb9d6b385  paas-daemon/internal/store/migrate.go
e2927f697de07b495085a497a0f5a72127509bf3303e869347837e24ecb5a3df  paas-daemon/internal/store/migrations/00003_operation_history.sql
34de65899eb2ee56a20b22ff083e40d3be8756e8ba673c2a4d5436a6d892abf1  paas-daemon/internal/store/store.go
```
