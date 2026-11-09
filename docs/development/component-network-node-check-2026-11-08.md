# Эксперимент сетевой изоляции на network-check, 8 ноября 2026

На отдельной одноузловой Lima VM `network-check` (2 GiB) запущен k3s `v1.37.2+k3s1`, flannel и kube-router NetworkPolicy, iptables `v1.8.11 (nf_tables)`. Узел Ready, IPv4 Node IP `192.168.5.15`, Pod CIDR `10.42.0.0/24`, API ClusterIP `10.43.0.1`. Отдельного IPv6 default route не было. Служебные и тестовые образы импортированы из локальных архивов: DNS registry внутри VM был недоступен.

В отдельном Namespace созданы listener Pod с NodePort `32334`, неограниченный контрольный Pod и restricted Pod. На restricted действовала экспериментальная egress NetworkPolicy `0.0.0.0/0` с исключениями `10/8`, `127/8`, `169.254/16`, `172.16/12`, `192.168/16`, `224/4`. На узле временно работал HTTP listener `:18337`. Дополнительно создан отдельный network namespace с veth-парой `203.0.113.1/30` на узле и `203.0.113.2/30` у peer; peer слушал TCP `:18338`.

| Новое TCP-соединение | Контрольный Pod | Restricted Pod |
| --- | --- | --- |
| Node IP `192.168.5.15:32334`, NodePort | подключился | отказ |
| Node IP `192.168.5.15:18337`, host listener | подключился | отказ |
| Node IP `192.168.5.15:6443`, API | подключился | отказ |
| Node IP `192.168.5.15:10250`, kubelet | подключился | отказ |
| API ClusterIP `10.43.0.1:443` | подключился | отказ |
| Управляемый peer `203.0.113.2:18338` | подключился | подключился |

Адрес тестового интерфейса узла `203.0.113.1:18337` до добавления отдельного исключения был достижим обоим Pod. После `kubectl -n paas-network-preflight patch networkpolicy public-only --type=json -p '[{"op":"add","path":"/spec/egress/0/to/0/ipBlock/except/-","value":"203.0.113.1/32"}]'` контрольный Pod по-прежнему подключался, restricted получил отказ, а peer `203.0.113.2:18338` остался доступен обоим. Проверки проводились командами `kubectl -n paas-network-preflight exec <pod> -- nc -z -w 3 <ip> <port>`; оба Pod проверялись для каждого listener.

`203.0.113.0/24` — TEST-NET. Эксперимент специально допускал эти адреса, чтобы проверить механизм dataplane в изолированном стенде; реальный профиль `all` должен исключать весь этот диапазон, как и остальные специальные и внутренние диапазоны. Маршрут через veth менялся только на временной VM; host firewall не изменялся.

После начального эксперимента временный Namespace и veth удалили. Для проверки перезапуска на той же VM установили временный systemd unit k3s, заново создали Pod и NetworkPolicy и выключили/включили VM. Boot ID изменился с `cdb220c6-cd9d-41d6-bc94-cc0d744ffb52` на `68b99b81-40d4-48b3-b6fb-c4b287b9b8dc`; существующие policy и NodePort `31450` сохранились. После запуска тестового host listener контрольный Pod подключился к host listener, NodePort, API и kubelet, restricted получил отказ на каждом из четырёх адресов. Listener на хосте и veth-peer после загрузки создавались заново; их автоматическое восстановление не проверялось.

Затем к тестовому интерфейсу узла добавили `203.0.113.5/32`, оставив в экспериментальной NetworkPolicy исключение только для прежнего адреса узла `203.0.113.1/32`. Control подключился к обоим адресам host listener и к peer. Restricted получил отказ на `.1`, но **подключился к новому адресу узла `.5`** и к peer. После добавления `.5/32` в исключения restricted перестал подключаться к `.5`, сохранив доступ к peer; control подключался к обоим. Измерение показывает окно между изменением адресов узла и обновлением IP-исключений. Все адреса этой пробы остаются TEST-NET и в реальном профиле исключаются целиком.

Это эксперимент с NetworkPolicy, а не проверка installer и не подтверждение независимой устойчивой границы Pod/host. Он не проверял реальные внешние маршруты, UDP, IPv6 или все возможные адреса и NodePort. Наблюдаемое после смены адреса окно требует независимого host-ограничения или другого согласованного механизма: периодическое обновление NetworkPolicy само по себе не закрывает окно. До подтверждения устойчивого ограничения и его повторной проверки при изменении профиля `all` остаётся BLOCKED.

После этой проверки временный Namespace удалён с ожиданием завершения, listener и veth-peer остановлены и удалены, экспериментальный systemd unit k3s удалён. До следующего опыта host firewall не менялся; другие VM не изменялись.

## Эксперимент с одобренной nftables-границей

Владелец отдельно разрешил испытать nftables только на `network-check`. На ноде временно установили пакет `nftables` и создали собственную таблицу `inet paas_network_probe`. В `prerouting` с priority `-101`, до обычного dstnat, правило для входа `cni0` и IPv4 source `10.42.0.0/24` сначала допускает `ct state established,related`, затем отклоняет `fib daddr type local`. Это экспериментальная конфигурация, не штатный installer. До применения оба Pod подключались к не исключённому тестовому адресу узла `203.0.113.5:18337` и к peer `203.0.113.2:18338`.

После применения оба Pod получили отказ к host listener, NodePort и Node IP API, а host положительно подключился к тем же listener/NodePort/API. Оба Pod продолжили подключаться к управляемому peer. После добавления на узел ещё одного тестового адреса `203.0.113.9/32` без изменения nft и NetworkPolicy оба Pod получили отказ к новому адресу host; host подключился, peer по-прежнему был доступен обоим. Счётчик правила drop вырос с 0 до 19. Это наблюдение IPv4 на данном dataplane, включая исключённые из реального `all` TEST-NET адреса.

Путь Pod→Node IP API и Pod→API Service различается: после nft control Pod получил отказ к `192.168.5.15:6443`, но подключился к `10.43.0.1:443`; restricted Pod получил отказ к обоим, поскольку его NetworkPolicy исключает Service CIDR. Для restricted Pod добавили разрешение DNS на CoreDNS, после чего оба Pod разрешали имена. CoreDNS и Traefik пересоздали при действующей nft-границе; затем создали новый Service и Ingress. Оба Pod разрешили DNS нового Service, а HTTP через Traefik NodePort с новым Host вернул `v1`. Host→Traefik NodePort и control Pod→Traefik ClusterIP подключались; Pod→Node IP Traefik NodePort отклонялся.

Это не подтверждает безопасность других CNI, IPv6, spoofed source, всех вариантов NodePort, сохранения текущих соединений или реакцию на потерю/изменение правил. Проверка systemd и reboot выполняется отдельно; статус `all` по этим наблюдениям не публикуется.

У исходного варианта с `ip saddr 10.42.0.0/24` обнаружен обход: тестовый Pod с `uid=0` и `CAP_NET_RAW` отправил raw UDP с подставленным source `192.168.5.2` на host `203.0.113.9:18339`; собственный listener получил payload. Это не доказывает возможность такой отправки обычным workload: его production-профиль требует PSA Restricted и `drop ALL`. Тем не менее штатная граница не должна доверять source, заданному Pod. В экспериментальной таблице оба правила заменили на `iifname "cni0" meta nfproto ipv4`: сначала `ct state established,related accept`, затем `fib daddr type local counter drop`. Повторный spoofed UDP listener не получил, счётчик drop вырос; отдельный UDP от host listener получил. Этого недостаточно для вывода о защите от всех вариантов spoofing или уже установленных соединений.

Для отдельной проверки порядка загрузки временный unit nft сделал зависимостью `Requires` и `After` тестового k3s unit. При намеренно неверном nft-файле guard unit перешёл в `failed`, k3s остался `inactive` с dependency failure. После восстановления файла оба unit снова стали active. Эти units существуют только в тестовой VM и не являются реализацией installer.

После восстановления выполнен ещё один cold start: boot ID изменился с `4e76fa57-3321-447a-a8bf-dbea0544cf76` на `733c7064-0bdf-4c67-87df-0da747aa54cc`, оба unit стали active, граф `Requires`/`After` сохранился, а действующая nft-таблица уже не содержала source-предиката. API ClusterIP, DNS и HTTP через существующий Traefik Ingress работали; Node IP API был закрыт Pod. После повторного создания host listener и veth-peer Pod не подключился к новому TEST-NET адресу узла, но подключился к peer; host подключился к собственному listener. Spoofed UDP из тестового root Pod снова не дошёл, положительный UDP от host дошёл. Старт Traefik асинхронный: первая HTTP-попытка сразу после готовности тестовых Pod получила connection refused; после `kubectl rollout status deployment/traefik` повтор вернул `v1`.

Выявлен функциональный отказ общего правила для всех Pod. При активной тестовой nft-таблице `metrics-server` оставался `0/1 Ready`, в журнале повторялся timeout к kubelet `192.168.5.15:10250/metrics/resource`; APIService `v1beta1.metrics.k8s.io` показывал `MissingEndpoints`, запрос metrics API возвращал 503, `kubectl top nodes` — `Metrics API not available`. После удаления **только** таблицы `paas_network_probe` на той же VM, без замены Pod, metrics-server перешёл в Ready, APIService стал Available, metrics API и `kubectl top nodes` вернули данные. Namespace в момент проверки ещё оставался Terminating и позднее был удалён контроллером. При этом systemd guard по-прежнему показывал `active`, хотя таблица уже отсутствовала: состояние unit не доказывает наличие правил. Не следует добавлять доверие к source IP как исправление: выше зафиксирован его spoofing. Штатный guard с таким глобальным правилом пока не пригоден; ограниченный способ поддержки системных workload требует отдельного решения и проверки.

## Эксперимент ограниченного допуска metrics-server

Для привязки сетевого исключения к штатному metrics-server проверены оба
официальных airgap-архива k3s `v1.37.2+k3s1`. SHA-256 архивов совпали с
опубликованными контрольными суммами. Для образа
`docker.io/rancher/mirrored-metrics-server:v0.9.0` проверены размер и SHA-256
каждого дескриптора от списка платформ до конфигурации соответствующей
архитектуры. [Суммы релиза arm64](https://github.com/k3s-io/k3s/releases/download/v1.37.2%2Bk3s1/sha256sum-arm64.txt)
и [суммы релиза amd64](https://github.com/k3s-io/k3s/releases/download/v1.37.2%2Bk3s1/sha256sum-amd64.txt).

| Архитектура | Digest манифеста платформы | Digest конфигурации |
| --- | --- | --- |
| linux/arm64 | `sha256:d0341e07a1cf2b751e684cba68e2a0c62d3c5d2d23501fd67dc384d517ec65de` | `sha256:37ceda8805d2e3bca331bcdd659d9cf8d3b4075a68b288054c88e62232d52883` |
| linux/amd64 | `sha256:25d291fde59974547bac6f07fa9d6cf6f5bedd1f19d60c893311c5e741e0a42f` | `sha256:d7b01abacd67fcd2cd2f61b87cb46c850c4f090e77d3d5ae0878b9894e0fcd0f` |

На network-app-check файл Deployment побайтно совпал с шаблоном релиза после
двух штатных подстановок. Использованы пустой префикс реестра и порядок адресов
`InternalIP,ExternalIP,Hostname`. SHA-256 полученного файла —
`528152585ac18dd47c180b31bf667288cab779caa23516d094a2417b527c5ef1`.
Для Deployment с отдельным именем выполнен server-side dry-run. Полный
`spec.template` после заполнения значений по умолчанию совпал с шаблоном
работающего Deployment. SHA-256 компактного JSON с отсортированными ключами —
`31d144b3a4c8c3c5ba4c07d445aa116d20eb4ac65b3f11dc35a9f25a052fe060`.
Живые сетевые испытания проводились на arm64. [Исходный manifest k3s](https://github.com/k3s-io/k3s/blob/v1.37.2%2Bk3s1/manifests/metrics-server/metrics-server-deployment.yaml).

Владелец отдельно одобрил тест identity через host veth. Для Pod metrics-server сверены UID и containerd sandbox; `eth0.iflink=6` в его netns совпал с host `vethde094f2e.ifindex=6`, master `cni0`. В тестовой `bridge prerouting` таблице для трафика через `cni0` очищался зарезервированный бит packet mark; только для этого veth при назначении Node IP `192.168.5.15`, TCP `10250` бит выставлялся. В `inet prerouting` перед общим drop разрешался только пакет с этим битом и тем же назначением/портом. Второй тестовый Pod не подключился к kubelet. Изначально metrics API отвечал, но счётчик исключения был 0, поэтому существующее established-соединение не считали доказательством нового допуска.

После `rollout restart` старый Pod исчез, новый UID `796dc436-3dd5-4ab6-b195-0761430a518d` получил IP `10.42.0.83`; его `eth0.iflink=12` совпал с `vethc6c40be4.ifindex=12`, master `cni0`. До обновления привязки новый Pod оставался `0/1`, а scrape kubelet завершался timeout. После атомарной замены имени veth в тестовом bridge-правиле metrics-server стал Ready, счётчик ограниченного accept показал новый пакет, `kubectl top nodes` вернул данные.

Другой тестовый root Pod с `CAP_NET_RAW` отправил raw TCP SYN на Node IP:10250, подставив source IP metrics Pod и `SO_MARK=0x40000000`. На inet hook счётчик «подделанная метка сохранилась» остался 0, счётчик «метка очищена» стал 1, общий drop вырос; metrics API продолжил отвечать. Это проверка выбранного kernel/dataplane, а не доказательство всех вариантов spoofing. Тестовый Pod имел больше прав, чем обычный workload с PSA Restricted и `drop ALL`.

Первое восстановление выполнялось вручную. Затем на этой же VM запущен временный root observer: он выбирал ReplicaSet по UID владельца и текущей revision Deployment, затем единственный Running Pod текущего ReplicaSet с service account `metrics-server`, единственный Ready containerd sandbox с тем же Pod UID и сверял Pod IP, netns `eth0.iflink`, host veth ifindex и master `cni0`. При изменении identity он сначала атомарно удалял старый veth из bridge-цепи, повторно проверял identity и только затем добавлял новый. Ещё один `rollout restart` завершился без ручного разрешения: observer перешёл от Pod `796dc436-3dd5-4ab6-b195-0761430a518d`/`vethc6c40be4` к Pod `224b7b32-2e60-4d90-a114-7e3844f98b9e`/`vethce5b340f`, metrics-server стал Ready, счётчик accept вырос и `kubectl top nodes` работал. Общая host-граница при этом не отключалась.

Это прототип, а не штатный observer: он не реализует весь контракт проверки trusted Deployment/Namespace и образа, безопасный повтор после отказа, проверку системных таблиц на дрейф, постоянный publisher snapshot или полный набор неоднозначных состояний. Эксплуатационная готовность исключения и режима `all` пока не подтверждена.

Отдельно создали временную неоднозначность: два Pod metrics-server были Running в текущем ReplicaSet. Прототип сообщил `ambiguous current metrics Pod` и убрал veth-исключение, оставив очистку mark и общий drop; после возврата к одной реплике сам восстановил привязку. Существующее established-соединение могло оставаться разрешённым по conntrack, поэтому проверено закрытие допуска для новых соединений, а не немедленный разрыв старых. На раннем варианте прототипа ошибка синтаксиса nft при старте оставляла прежнее исключение; в эксперименте это исправили очисткой при старте, но корректное восстановление после неизвестного результата записи ещё требует штатного журнала и отдельных испытаний.

## Ordinary grant и revoke через API

Для независимой приёмки использована чистая одноузловая VM `network-app-check`: штатные `ctl node prepare` и `node install` установили k3s `v1.37.2+k3s1`, затем PostgreSQL `18.6` и schema v13. Первый WIP daemon имел digest `sha256:da266d7e05f9c1ca9a0ba6476aff6aaeb7b3d87f37c7406a7194f3369c2803d7`; затем штатно установлен исправленный daemon `sha256:d09748519096eaef7eda78b654b306f878bc1decf9e080c988a1d35e38f4e946` без повторной миграции. Job `control-migrate-v13` с UID `30f7f4f0-b842-40f1-933e-7117f89be833` завершился успешно; до испытания `/ready` вернул `ok`, daemon не перезапускался.

Через prepare-app, settings API и deployment preview/apply созданы два приложения по два компонента. Все четыре Deployment дошли до SUCCEEDED. До grant A1 подключился к A2 по Pod IP и получил `v1`; A1→B1 ClusterIP был закрыт. После гранта A1→B1 ревизия стала APPLIED; A1 получил `v1` и через ClusterIP, и через разрешённый Pod IP/порт. Новые A2→B1, B1→A1 и A1→B2 соединения были отклонены.

На первом WIP отзыв сначала дважды вернул 503: PostgreSQL зафиксировал отмену ожидания общего advisory lock по таймауту settings writer. Повтор сохранил ревизию и новые A1→B1 соединения были отклонены, но статус колебался между APPLIED и BLOCKED/UNPROVEN_POLICY при одинаковой applied revision. После исправления блокировки и штатного обновления только daemon выполнены три цикла grant/revoke без 503: времена set-policy для grant составили 1,907 / 1,485 / 1,324 с, для revoke — 0,951 / 0,099 / 1,129 с. Все шесть ревизий достигли APPLIED, по пять последующих GET для каждой остались APPLIED. При каждом grant новый запрос возвращал `v1`, после каждого revoke новое соединение отклонялось. Наибольшее измеренное время 1,907 с относится к этому стенду и не является верхней оценкой задержки под нагрузкой.

После отзыва у egress policy с UID `b9768723-95c5-4eb5-ab63-ab9ee0be6e6b` и ingress policy с UID `bd21506d-89d7-401f-af9a-c859798eeb40` списки разрешений были пусты. Независимый запрос из B1 к своему ClusterIP `10.43.182.119:8080` вернул `v1`, а свежий запрос из A1 к тому же адресу получил `connection refused`.

Во время установки обнаружен отдельный дефект wrapper: на профиле только v13 `prepare-app` искал отсутствующий v4 settings profile. На стенде для продолжения штатно добавили v4 profile; исходный wrapper исправлен и отдельно проверен в изолированном Debian-контейнере на v13-only, отсутствующий и symlink profile. Повтор на VM именно с одним профилем v13 ещё не выполнен. Эти проверки ordinary grant/revoke не подтверждают режим `all`, UDP, named targetPort, failover/restart или все варианты дрейфа.

При отсутствии publisher настройка `all` сохранилась с новой desired revision и состоянием BLOCKED/`HOST_ISOLATION_UNPROVEN`; затем режим вернули в `none`, новая ревизия достигла APPLIED. Это проверка отказа, не проверка внешнего доступа `all`.

Дополнительно проверен grant **до первого запуска** источника. Первая операция на образе `d097…` ушла в RECOVERY_REQUIRED за 223 мс до создания action или network intent. Независимо от точной причины той операции обнаружен дефект: `pg_try_advisory_lock=false` до callback трактовался как неизвестный исход. После исправления и обновления daemon на `sha256:2c63781a5a362a47047cd6a6749e6ef64505774738d2e64bd52bc12358f3752f` новый источник с заранее сохранённым grant сначала показал PENDING, первый deploy завершился SUCCEEDED, ревизия достигла APPLIED, запрос к B1 вернул `v1`. Target ingress UID `e130837c-37cd-473d-ae13-398090e73acf` создан в `19:12:36Z`, source egress UID `3fb4255a-08ca-42c0-bd65-134a72530c6b` — в `19:12:37Z`, Deployment UID `72cac211-2b97-4ab7-a1ad-a79fa189eb7f` — в `19:12:39Z`. Отдельный PostgreSQL и unit regression проверяет, что только подтверждённый busy до callback остаётся для повтора, а неизвестный исход после action требует recovery. Причину конкретной первой операции без исходной внутренней ошибки утверждать нельзя.

### Направление conntrack

На отдельной `network-check` в экспериментальной `inet paas_network_probe`
заменено общее `ct state established,related accept` на
`ct state established,related ct direction reply accept`. Временное узкое
разрешение TCP с control Pod 10.42.0.82 к NodeIP:18400 позволило установить
сокет и получить `x`. После удаления только этого временного разрешения тот же
сокет перестал доставлять данные (timeout). Независимо соединение с узла к
Pod listener 10.42.0.80:8080 получило `network-probe`. Это эксперимент
conntrack на тестовой VM, не приёмка штатного установщика.

На той же VM в отдельных mount и network namespaces проверен WIP boot helper:
при root-owned intent `ipv4-flannel-v1` созданы обе deny-only таблицы одним
nft batch и проверено точное readback; повторный запуск принят, а после добавления
постороннего правила повторный запуск отказал. Эта изолированная проверка не
означает установку unit или включение профиля на узле.
