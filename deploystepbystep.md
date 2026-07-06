# Scout: пошаговое руководство по развёртыванию на VPS (Caddy + GHCR)

Это подробная инструкция для человека, который **никогда раньше не
разворачивал приложение на VPS**. Она рассчитана на то, что вы будете
копировать команды из блоков кода по порядку. Команды и имена переменных
оставлены на английском (как они есть в репозитории); пояснения — на
русском.

Краткая версия для тех, кто уже знаком с темой, находится в разделе
[«Deploy to a VPS with Caddy and GHCR»](./README.md#deploy-to-a-vps-with-caddy-and-ghcr)
файла `README.md`. Здесь — полная процедура: что арендовать/купить, какие
значения откуда брать и куда вставлять, как настроить SSH/Docker/файрвол/
DNS/объектное хранилище/GitHub/Caddy, как засеять базу и проверить всё, и как
не потерять доступ к серверу и не «утечь» секреты.

**Важно.** Эта инструкция не подключается к реальному серверу и не выполняет
реальных изменений DNS/сертификатов/секретов — весь код и конфигурация в этом
репозитории проверены локально (см. финальный отчёт по задаче 021). Каждый
шаг ниже вы выполняете сами, на своей инфраструктуре.

---

## 7.1 Таблица подготовки (заполните перед началом)

Прежде чем начинать, заведите себе (в текстовом файле или менеджере паролей)
такую таблицу и заполняйте её по ходу дела. Каждое значение из колонки
`Placeholder` используется в командах ниже вместо угловых скобок — например,
`<SERVER_IP>` замените на реальный IP-адрес вашего сервера.

| Placeholder | Пример формата | Где взять | Где используется |
| --- | --- | --- | --- |
| `<SERVER_IP>` | `203.0.113.10` | Панель управления VPS | DNS, SSH, секрет GitHub |
| `<SSH_PORT>` | `22` | Настройки VPS / `sshd_config` | SSH, файрвол, секрет GitHub |
| `<DEPLOY_USER>` | `deploy` | Создаётся вами на сервере | SSH-подключение из workflow |
| `<DOMAIN>` | `scout.example.com` | Регистратор доменов / DNS | DNS, Caddy, CORS |
| `<ACME_EMAIL>` | `you@example.com` | Ваш email | Аккаунт сертификатов Caddy |
| `<GITHUB_OWNER>` | `kartmos` (в нижнем регистре) | URL репозитория | Имена образов GHCR |
| `<GITHUB_REPOSITORY>` | `scout-takehome` | URL репозитория | Имена образов GHCR |
| `<S3_ENDPOINT>` | `s3.eu-central-1.amazonaws.com` | Панель объектного хранилища | env сервера |
| `<S3_PUBLIC_ENDPOINT>` | `cdn.example.com` | Объектное хранилище / DNS | Presigned-ссылки |
| `<S3_BUCKET>` | `scout-photos-prod` | Панель объектного хранилища | env сервера |
| `<S3_REGION>` | `eu-central-1` | Панель объектного хранилища | env сервера |

**Важное предупреждение.** IP-адрес `203.0.113.10` в этой таблице — это
документационный TEST-NET адрес (RFC 5737), он используется только как
пример формата и никогда не резолвится в интернете. Не используйте его как
реальный адрес — впишите туда IP, который выдаст ваш провайдер VPS.

---

## 7.2 Аренда сервера приложения

1. Выберите поддерживаемый Linux LTS-дистрибутив — эта инструкция использует
   **Ubuntu 24.04 LTS** (Debian 12 подходит так же, команды `apt`
   идентичны).
2. Архитектура: **x86-64 (amd64)** — самый распространённый и дешёвый
   вариант; **ARM64 (aarch64)** тоже полностью поддерживается (образы
   собираются под обе платформы), но проверьте, что провайдер продаёт именно
   ARM-инстансы, если хотите сэкономить на ARM-тарифе.
3. Размер сервера:
   - **Минимум**: 1 vCPU, 1 GB RAM, 20 GB SSD — этого достаточно, потому что
     сам Scout (api + web + Caddy) документированно укладывается примерно в
     0.5–1 vCPU и ~576 MiB RAM (см. таблицу «Resource budget» в README), а
     остальное занимает ОС и системные процессы.
   - **Комфортный вариант**: 1–2 vCPU, 2 GB RAM — оставляет запас для ОС,
     логов, обновлений безопасности и разового процесса `seed`.
   - Эти цифры считают **только приложение Scout**; они не включают
     отдельный сервер под MinIO (см. пункт 6 ниже).
4. Серверу нужен **публичный IPv4-адрес** — без него Let's Encrypt не сможет
   провалидировать домен через HTTP-01. IPv6/`AAAA`-запись добавляйте, только
   если провайдер подтвердил, что IPv6 подключён и файрвол настроен для него
   (см. 7.5); если сомневаетесь — не добавляйте `AAAA` вообще.
5. Сохраните доступ к **консоли восстановления провайдера** (web-консоль /
   "recovery console" / "VNC console") — это ваш запасной вход, если
   что-то пойдёт не так с SSH или файрволом.
6. Объектное хранилище (MinIO или S3-совместимый сервис) **не размещается на
   этом маленьком сервере** — оно намеренно вынесено за пределы бюджета
   `~1 vCPU / 512 MB–1 GB RAM`, отведённого под сам Scout. Если вы
   разворачиваете собственный MinIO, это отдельный сервер/инстанс со своим
   IP, своим DNS-именем и собственным (более щедрым) бюджетом ресурсов —
   не считайте его частью бюджета этого VPS.

---

## 7.3 Создание и установка SSH-ключа

На вашем локальном компьютере (не на сервере):

**macOS / Linux:**

```bash
ssh-keygen -t ed25519 -a 100 -f ~/.ssh/scout_deploy -C "scout-deploy"
cat ~/.ssh/scout_deploy.pub
```

**Windows (PowerShell):**

```powershell
ssh-keygen -t ed25519 -a 100 -f "$env:USERPROFILE\.ssh\scout_deploy" -C "scout-deploy"
Get-Content "$env:USERPROFILE\.ssh\scout_deploy.pub"
```

Это создаёт **пару ключей**:

- `scout_deploy` — **приватный ключ**. Он остаётся только на вашем
  компьютере (и позже — в виде секрета GitHub, см. 7.7). **Никогда** не
  отправляйте его по почте/мессенджеру и не коммитьте в git.
- `scout_deploy.pub` — **публичный ключ**. Его можно спокойно показывать —
  он вставляется в панель VPS (поле "SSH key" при создании сервера) или
  позже добавляется в `~/.ssh/authorized_keys` пользователя на сервере.

Права доступа обязательны:

```bash
chmod 700 ~/.ssh
chmod 600 ~/.ssh/scout_deploy
chmod 644 ~/.ssh/scout_deploy.pub
```

После создания сервера подключитесь первый раз и сверьте fingerprint,
который покажет провайдер в своей панели, с тем, что видите в терминале:

```bash
ssh -i ~/.ssh/scout_deploy root@<SERVER_IP>
# При первом подключении SSH покажет что-то вроде:
#   ED25519 key fingerprint is SHA256:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.
# Сверьте этот fingerprint с тем, что показывает панель провайдера
# (обычно в разделе информации о сервере/консоли), и только потом
# отвечайте "yes".
```

Не меняйте настройки доступа (файрвол, отключение пароля и т.д.), пока не
откроете **второй** SSH-сеанс и не убедитесь, что вход по ключу работает —
это защищает вас от случайной блокировки (см. 7.4).

Опционально — алиас в `~/.ssh/config` для удобства:

```
Host scout-server
    HostName <SERVER_IP>
    Port <SSH_PORT>
    User <DEPLOY_USER>
    IdentityFile ~/.ssh/scout_deploy
```

После этого можно подключаться просто командой `ssh scout-server`.

---

## 7.4 Безопасная подготовка и укрепление сервера

Выполняется на сервере (сначала под `root`, которого даёт провайдер).

**1. Обновление пакетов:**

```bash
apt update && apt -y upgrade
```

**2. Создание пользователя `<DEPLOY_USER>` и его ключа:**

```bash
adduser --disabled-password --gecos "" <DEPLOY_USER>
usermod -aG sudo <DEPLOY_USER>

mkdir -p /home/<DEPLOY_USER>/.ssh
chmod 700 /home/<DEPLOY_USER>/.ssh
# Вставьте содержимое scout_deploy.pub (см. 7.3) в файл ниже:
nano /home/<DEPLOY_USER>/.ssh/authorized_keys
chmod 600 /home/<DEPLOY_USER>/.ssh/authorized_keys
chown -R <DEPLOY_USER>:<DEPLOY_USER> /home/<DEPLOY_USER>/.ssh
```

**Проверьте вход новым пользователем в отдельном терминале, не закрывая
текущую root-сессию:**

```bash
ssh -i ~/.ssh/scout_deploy <DEPLOY_USER>@<SERVER_IP>
sudo -v   # подтверждает, что sudo работает
```

**3. Установка Docker Engine (официальный репозиторий) и Compose v2:**

```bash
apt -y install ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  | tee /etc/apt/sources.list.d/docker.list > /dev/null

apt update
apt -y install docker-ce docker-ce-cli containerd.io docker-compose-plugin
```

**4. Добавление `<DEPLOY_USER>` в группу `docker`:**

```bash
usermod -aG docker <DEPLOY_USER>
```

> **Предупреждение.** Членство в группе `docker` фактически равносильно
> root-доступу: любой, кто может запускать контейнеры, может смонтировать
> корень файловой системы хоста внутрь контейнера. Добавляйте в эту группу
> только пользователя, которым управляет workflow деплоя.

Разлогиньтесь и зайдите заново (группа применяется при новом входе), затем
проверьте:

```bash
ssh -i ~/.ssh/scout_deploy <DEPLOY_USER>@<SERVER_IP>
docker version
docker compose version
```

**5. Синхронизация времени** (нужна для корректной проверки TLS-сертификатов
и подписанных S3-запросов):

```bash
timedatectl set-ntp true
timedatectl status   # убедитесь, что "System clock synchronized: yes"
```

**6. Настройка файрвола (UFW) — сначала разрешите нужные порты, потом
включайте:**

```bash
apt -y install ufw
ufw allow <SSH_PORT>/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable
ufw status verbose
```

**Не включайте `ufw enable`, пока не добавили правило для вашего реального
SSH-порта** — иначе вы потеряете доступ к серверу.

**7. Создание рабочих каталогов:**

```bash
mkdir -p /opt/scout/data
chown -R <DEPLOY_USER>:<DEPLOY_USER> /opt/scout
chmod 750 /opt/scout
```

**8. (Опционально) автоматические обновления безопасности:**

```bash
apt -y install unattended-upgrades
dpkg-reconfigure -plow unattended-upgrades
```

Компромисс: это снижает риск непропатченных уязвимостей, но обновления
могут перезапускать системные службы в неожиданное время — на единственном
production-сервере планируйте окно обслуживания (см. 7.12), если это для вас
важно.

**О «жёсткой» настройке SSH.** Эта инструкция **намеренно не автоматизирует**
отключение входа по паролю/root, потому что ошибка в конфигурации `sshd`
может заблокировать вас снаружи без физического доступа к серверу. Если вы
всё же хотите это сделать, сначала убедитесь во всех четырёх пунктах:

1. вход по ключу для `<DEPLOY_USER>` работает во **втором** отдельном
   терминале (первый держите открытым);
2. `sudo` работает;
3. вы знаете, как попасть в консоль восстановления провайдера;
4. конфигурация проверена командой `sshd -t` **до** перезапуска службы.

```bash
cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak-$(date +%Y%m%d)
nano /etc/ssh/sshd_config
# PasswordAuthentication no
# PermitRootLogin no
sshd -t                      # обязательно перед перезапуском
systemctl restart ssh
```

Если что-то пошло не так — восстановите бэкап и перезапустите:

```bash
cp /etc/ssh/sshd_config.bak-YYYYMMDD /etc/ssh/sshd_config
sshd -t && systemctl restart ssh
```

Смена SSH-порта (`<SSH_PORT>` ≠ `22`) — это необязательный шаг и **не
замена** ключей и файрвола, а лишь небольшое снижение шума от автоматических
сканеров.

---

## 7.5 Покупка и настройка домена

Покупка домена (у регистратора) и настройка DNS — это две разные операции;
DNS обычно настраивается либо у самого регистратора, либо у отдельного
DNS-провайдера (например, Cloudflare), на который указывают NS-записи
домена.

1. Создайте `A`-запись для `<DOMAIN>` (или поддомена, например
   `scout.example.com`), указывающую на `<SERVER_IP>`.
2. TTL можно оставить низким на время первичной настройки (например, 300 c);
   после стабилизации — увеличить.
3. Проверьте распространение записи:

   ```bash
   dig +short <DOMAIN>
   # или
   nslookup <DOMAIN>
   ```

   Результат должен совпадать с `<SERVER_IP>`. Изменения DNS могут занимать
   от нескольких минут до нескольких часов.
4. Добавляйте `AAAA`-запись, только если вы подтвердили, что у VPS есть
   рабочий IPv6-адрес **и** файрвол настроен для IPv6 — иначе оставьте
   только `A`-запись.
5. **Cloudflare (опционально).** Если домен управляется через Cloudflare:
   - на этапе первого получения сертификата держите режим **"DNS only"**
     (серое облако) — Caddy должен видеть реальный IP сервера для ACME
     HTTP-01 challenge;
   - только после того как сертификат Caddy выпущен и сайт работает, можно
     включить проксирование ("Proxied", оранжевое облако) с режимом
     TLS **"Full (strict)"** — это не обязательно, а опциональная надстройка.
6. Выпуск сертификата Caddy требует одновременно: (a) DNS-запись указывает
   на сервер и (b) порты 80/443 реально доступны снаружи в момент первого
   запуска Caddy.

---

## 7.6 Подготовка внешнего объектного хранилища

Проект **всегда** использует внешнее, S3-совместимое хранилище — оно не
разворачивается на этом маленьком сервере. Ниже — шаги в общем виде (для
AWS S3, Cloudflare R2 или отдельно развёрнутого MinIO терминология в панели
может немного отличаться).

1. Создайте **приватный** bucket (без публичного анонимного доступа).
2. Создайте **отдельный** access key с правами только на этот bucket
   (принцип наименьших привилегий — не переиспользуйте ключи от других
   проектов).
3. Запишите в таблицу из 7.1:
   - `<S3_ENDPOINT>` — внутренний адрес `host:port` (без схемы `https://`);
   - `<S3_PUBLIC_ENDPOINT>` — публичный адрес, встраиваемый в presigned-
     ссылки, которые видит браузер (может совпадать с `<S3_ENDPOINT>`, если
     оба доступны из интернета);
   - `<S3_BUCKET>`, `<S3_REGION>`;
   - TLS: используется ли `https` (`SCOUT_S3_SECURE=true`) на каждом из
     эндпоинтов.
4. Если presigned-ссылки читаются напрямую из браузера, а bucket настроен за
   отдельным доменом/CDN, может понадобиться CORS-политика на **самом
   bucket** (не в приложении), разрешающая `GET` с `https://<DOMAIN>` — при
   этом сам bucket **остаётся приватным**, доступ идёт только по подписанным
   URL с ограниченным сроком жизни.
5. Сопоставление с переменными сервера (раздел 7.8):

   | Значение из панели хранилища | Переменная `.env.production` |
   | --- | --- |
   | Внутренний endpoint | `SCOUT_S3_ENDPOINT` |
   | Публичный endpoint (если отличается) | `SCOUT_S3_PUBLIC_ENDPOINT` |
   | Access key | `SCOUT_S3_ACCESS_KEY` |
   | Secret key | `SCOUT_S3_SECRET_KEY` |
   | Bucket | `SCOUT_S3_BUCKET` |
   | Region | `SCOUT_S3_REGION` |
   | TLS у внутреннего endpoint | `SCOUT_S3_SECURE` |
   | TLS у публичного endpoint (если отличается) | `SCOUT_S3_PUBLIC_SECURE` |

6. Эти значения **пишутся только** в `/opt/scout/.env.production` на
   сервере (см. 7.8) — не в GitHub (см. обоснование в 7.7).
7. Быстрая диагностика после настройки:

   ```bash
   # Проверка сетевой доступности и TLS
   curl -Iv https://<S3_ENDPOINT>/

   # Проверка резолва DNS публичного endpoint
   dig +short <S3_PUBLIC_ENDPOINT>
   ```

   Частые причины ошибок подписи (`SignatureDoesNotMatch` / 403):
   неправильная схема (`http` вместо `https` или наоборот), хостнейм в
   presigned-ссылке не совпадает с реальным endpoint, неверный регион, прокси
   переписывает заголовки на пути к хранилищу, либо рассинхронизировано
   время сервера (см. пункт 5 в 7.4).

---

## 7.7 Подготовка GitHub и GHCR

UI GitHub может немного отличаться по формулировкам, но путь такой:
**Repository → Settings → Environments** и **Repository → Settings →
Secrets and variables → Actions**.

**1. Создайте Environment `production`:**

- Settings → Environments → New environment → имя `production`.
- Deployment branches: ограничьте только `main`.
- **Не добавляйте** "Required reviewers" — это условие нужно для полностью
  автоматического (zero-click) деплоя, описанного в этой инструкции. Если вы
  всё же хотите ручное подтверждение перед каждым деплоем — добавьте
  Required reviewers, но тогда процесс станет approval-gated (кто-то должен
  вручную одобрять каждый деплой в интерфейсе GitHub) — это осознанный
  отдельный режим, не то, что описано ниже.

**2. Создайте Actions variable `PRODUCTION_DEPLOY_ENABLED`:**

- Settings → Secrets and variables → Actions → вкладка **Variables** → New
  repository variable.
- Имя: `PRODUCTION_DEPLOY_ENABLED`, значение: `false` (пока инфраструктура не
  готова — см. 7.10, когда включать `true`).

**3. Настройте защиту ветки `main`** (Settings → Branches → Branch protection
rules, или новый Rulesets):

- Require a pull request before merging.
- Require status checks to pass: выберите `CI` и `Integration`.
- Block force pushes и запретите удаление ветки.
- Учтите: у администраторов репозитория обычно есть опция обхода правил
  ("Do not allow bypassing the above settings" — включите её, если хотите,
  чтобы правило действовало без исключений даже для админов).

**4. Добавьте секреты Environment `production`** (Settings → Environments →
production → Environment secrets → Add secret), точные имена:

| Имя секрета | Значение |
| --- | --- |
| `DEPLOY_HOST` | `<SERVER_IP>` или `<DOMAIN>`, если он уже резолвится |
| `DEPLOY_PORT` | `<SSH_PORT>` |
| `DEPLOY_USER` | `<DEPLOY_USER>` |
| `DEPLOY_SSH_PRIVATE_KEY` | содержимое `~/.ssh/scout_deploy` (приватный ключ) |
| `DEPLOY_SSH_KNOWN_HOSTS` | см. ниже — это **не** тот же ключ, что для входа |
| `VITE_SCOUT_API_KEY` | сгенерированный API-ключ, тот же, что попадёт в `SCOUT_API_KEY` на сервере |

Скопировать приватный ключ, не печатая его лишний раз в терминал:

```bash
pbcopy < ~/.ssh/scout_deploy      # macOS — сразу в буфер обмена
# или откройте файл в редакторе и скопируйте вручную, затем вставьте
# в текстовое поле секрета на GitHub
```

**Получение и проверка host key сервера** (это открытый **ключ самого
сервера**, которым SSH-клиент проверяет, что подключается к нужной машине —
не путайте его с вашим личным ключом входа из 7.3):

```bash
ssh-keyscan -p <SSH_PORT> <SERVER_IP> > /tmp/scout_known_hosts
cat /tmp/scout_known_hosts
```

Обязательно **сверьте fingerprint** с тем, что показывает консоль
провайдера, или с тем, что вы видели при самом первом ручном подключении в
7.3 (та строка `SHA256:...`), прежде чем доверять этому файлу:

```bash
ssh-keygen -lf /tmp/scout_known_hosts
```

Только после сверки вставьте содержимое `/tmp/scout_known_hosts` в секрет
`DEPLOY_SSH_KNOWN_HOSTS`. Именно так рабочий процесс деплоя проверяет
подлинность сервера — он **не** выполняет `ssh-keyscan` во время самого
деплоя (это было бы небезопасно, потому что доверяло бы первому встречному
ответу без проверки).

Установите `DEPLOY_HOST`, `DEPLOY_PORT`, `DEPLOY_USER` и `VITE_SCOUT_API_KEY`
как обычные секреты (см. таблицу выше).

**5. Права workflow.** `.github/workflows/deploy.yml` уже объявляет
минимально необходимые права (`contents: read`, `actions: read` для сверки
CI, `packages: write` только в задаче публикации образов) — в настройках
репозитория (Settings → Actions → General → Workflow permissions) достаточно
оставить `Read repository contents permission` по умолчанию; ничего
расширять вручную не требуется.

**6. Видимость пакетов GHCR.** По умолчанию только что опубликованные пакеты
GHCR обычно приватные.

- **Публичный пакет** — сервер тянет образы без входа в реестр (`docker
  compose pull` работает сразу).
- **Приватный пакет** — на сервере нужно один раз выполнить вход с токеном,
  имеющим **только** право `read:packages` (fine-grained personal access
  token, минимально необходимое):

  ```bash
  echo "<READ_PACKAGES_TOKEN>" | docker login ghcr.io --username <GITHUB_OWNER> --password-stdin
  ```

  Введите токен **интерактивно** (через `--password-stdin`, как показано
  выше) — никогда не подставляйте его как аргумент команды (он попадёт в
  историю shell и в списки процессов) и не сохраняйте в файлах репозитория.
  Также убедитесь, что видимость пакета и права доступа репозитория
  разрешают именно вашей учётной записи/токену на сервере читать оба образа
  (`<repo>-api` и `<repo>-web`).

**7. Почему секреты объектного хранилища не нужны в GitHub.** Продакшн-
креды S3/MinIO используются только внутри контейнера `api` на сервере — они
читаются исключительно из `/opt/scout/.env.production`. Workflow деплоя их
никогда не запрашивает и не передаёт, поэтому GitHub Actions вообще не
видит эти секреты.

---

## 7.8 Подготовка `/opt/scout/.env.production`

На сервере, от имени `<DEPLOY_USER>`:

```bash
cd /opt/scout
# Скопируйте deploy/.env.production.example из вашего локального клона репозитория,
# например через scp (см. пример команды в 7.9), либо создайте файл вручную:
nano /opt/scout/.env.production
chmod 600 /opt/scout/.env.production
```

Полный аннотированный пример (замените все `<...>` реальными значениями из
таблицы 7.1 и хранилища 7.6):

```bash
SCOUT_DOMAIN=<DOMAIN>
SCOUT_ACME_EMAIL=<ACME_EMAIL>

SCOUT_API_IMAGE=ghcr.io/<GITHUB_OWNER>/<GITHUB_REPOSITORY>-api
SCOUT_WEB_IMAGE=ghcr.io/<GITHUB_OWNER>/<GITHUB_REPOSITORY>-web
# deploy.sh перезаписывает копию этого значения только через переменную
# окружения SCOUT_IMAGE_TAG при каждом запуске; значение ниже — стартовое.
SCOUT_IMAGE_TAG=sha-0000000

SCOUT_DATABASE_PATH=/opt/scout/data/predictions.db

SCOUT_CORS_ALLOWED_ORIGINS=https://<DOMAIN>

SCOUT_API_KEY=<сгенерированное-значение-см-ниже>

SCOUT_S3_ENDPOINT=<S3_ENDPOINT>
SCOUT_S3_PUBLIC_ENDPOINT=<S3_PUBLIC_ENDPOINT>
SCOUT_S3_ACCESS_KEY=<ваш-access-key>
SCOUT_S3_SECRET_KEY=<ваш-secret-key>
SCOUT_S3_BUCKET=<S3_BUCKET>
SCOUT_S3_SECURE=true
SCOUT_S3_PUBLIC_SECURE=true
SCOUT_S3_REGION=<S3_REGION>

SCOUT_THUMBNAIL_CACHE_MAX_BYTES=268435456
```

Важные соответствия, о которых легко забыть:

- `SCOUT_CORS_ALLOWED_ORIGINS` должен буквально совпадать со схемой и
  доменом сайта (`https://<DOMAIN>`, без завершающего слэша).
- `SCOUT_API_IMAGE` / `SCOUT_WEB_IMAGE` должны **точно** совпадать с именами,
  которые публикует `.github/workflows/deploy.yml` (они выводятся в summary
  запуска workflow — см. 7.10).
- `SCOUT_API_KEY` на сервере обязан **дословно** совпадать со значением
  секрета `VITE_SCOUT_API_KEY` в GitHub (7.7) — иначе браузер будет получать
  `401/403` (см. 7.13).
- `SCOUT_DATABASE_PATH` указывает на файл, который вы ещё загрузите (7.9); он
  монтируется в контейнер **только для чтения**.
- Внутренний (`SCOUT_S3_ENDPOINT`) и публичный (`SCOUT_S3_PUBLIC_ENDPOINT`)
  адреса, а также флаги `SCOUT_S3_SECURE`/`SCOUT_S3_PUBLIC_SECURE`, должны
  соответствовать реальной схеме (`http`/`https`) каждого из них.

Генерация криптографически случайных секретов (например, для
`SCOUT_API_KEY`, а также `SCOUT_S3_ACCESS_KEY`/`SCOUT_S3_SECRET_KEY`, если вы
создаёте их сами, а не берёте из панели хранилища):

```bash
openssl rand -hex 32
```

Вставляйте результат прямо в `nano`/`vim`, не выводя его повторно в терминал
и не сохраняя в истории команд отдельной командой `echo`.

**Про `VITE_SCOUT_API_KEY`.** Это значение попадает в собранный JavaScript
браузера — то есть **любой посетитель сайта может его увидеть** через
инструменты разработчика. Это не настоящая аутентификация пользователей, а
демонстрационная схема с единым ключом (см. «Security limitations» в
`README.md`). Не описывайте и не считайте этот ключ конфиденциальным по
отношению к посетителям сайта — он служит только для того, чтобы браузер мог
обращаться к вашему собственному API.

Проверка без вывода секретов наружу:

```bash
grep -c '=' /opt/scout/.env.production   # должно совпасть с количеством строк-переменных
stat -c '%a %U' /opt/scout/.env.production   # ожидается: 600 <DEPLOY_USER>
```

---

## 7.9 Загрузка базы данных и исходных изображений для seed

С вашего локального компьютера (пути соответствуют клону репозитория):

```bash
scp -i ~/.ssh/scout_deploy -P <SSH_PORT> \
  dataset/predictions.db \
  <DEPLOY_USER>@<SERVER_IP>:/opt/scout/data/predictions.db

rsync -avz -e "ssh -i ~/.ssh/scout_deploy -p <SSH_PORT>" \
  dataset/images/ \
  <DEPLOY_USER>@<SERVER_IP>:/opt/scout/data/images/

# Файл deploy/.env.production.example тоже удобно скопировать так,
# а затем отредактировать на сервере (см. 7.8):
scp -i ~/.ssh/scout_deploy -P <SSH_PORT> \
  deploy/.env.production.example \
  <DEPLOY_USER>@<SERVER_IP>:/opt/scout/.env.production
```

Проверка на сервере:

```bash
ls -la /opt/scout/data/predictions.db
ls /opt/scout/data/images | wc -l          # ожидается 50
sha256sum /opt/scout/data/predictions.db
# Сравните вывод с зафиксированным хешем в .claude/prompts/ROADMAP.md:
# b84f73a33e99496d1152ef366d914c64a6e60cb72c494c9f2c42bc7b7dcaeb39
```

`predictions.db` **никогда не изменяется** приложением — API монтирует его
`read_only: true`. Каталог `dataset/images` (загруженный в
`/opt/scout/data/images`) — это **временный источник** для однократного
`seed`; в production originals отдаются исключительно из объектного
хранилища, а не из этого каталога на диске сервера. Удалять временные JPEG
можно только после того, как вы подтвердили, что все originals успешно
загружены в хранилище (см. конец 7.10).

---

## 7.10 Первый деплой и активация автоматизации

Пока переменная `PRODUCTION_DEPLOY_ENABLED` отсутствует или равна `false`,
слияния в `main` **безопасны** — автоматический деплой пропускается
(workflow сам выведет `::notice::` об этом и завершится без ошибок), а
образы продолжают проходить обычные `CI`/`Integration` проверки. Это
позволяет замёржить весь этот инфраструктурный код до того, как сервер, DNS
и секреты будут готовы.

**Когда всё из разделов 7.4–7.9 готово** (сервер укреплён, DNS указывает на
сервер, `.env.production` заполнен, GHCR доступен серверу, объектное
хранилище отвечает):

1. Установите `PRODUCTION_DEPLOY_ENABLED=true` (Settings → Secrets and
   variables → Actions → Variables → отредактировать значение).
2. Запустите первый деплой вручную: **Actions → Deploy → Run workflow**.
   Введите **точный** SHA коммита `main` в поле `commit_sha` (полные 40 hex-
   символов; посмотреть его: `git rev-parse main` в вашем локальном клоне).
   Оставьте поле `image_tag` пустым — это первый билд, готового тега ещё
   нет.
3. Workflow выполнит по порядку:
   - `build_and_push` — соберёт и опубликует оба образа (`linux/amd64` +
     `linux/arm64`) под тегом `sha-<короткий-коммит>` в GHCR;
   - `deploy` — передаст на сервер `deploy/compose.server.yaml` и
     `deploy/Caddyfile` (во временные `.new`-файлы), затем выполнит
     `/opt/scout/deploy.sh sha-<короткий-коммит>`.
4. В summary запуска (вкладка запуска в Actions) будут видны точные имена
   образов (`ghcr.io/<owner>/<repo>-api`, `ghcr.io/<owner>/<repo>-web`) и
   развёрнутый тег — сверьте их со значениями `SCOUT_API_IMAGE`/
   `SCOUT_WEB_IMAGE`/`SCOUT_IMAGE_TAG` в `/opt/scout/.env.production`
   (7.8), при необходимости поправьте файл и повторите `workflow_dispatch`.

   Если это самый первый запуск и файлы `compose.server.yaml`/`Caddyfile`
   ещё ни разу не передавались на сервер — workflow сам загрузит их (шаг
   "Transfer deployment files" выполняется при каждом запуске деплоя, не
   только при первом); дополнительный ручной bootstrap-перенос не нужен.

**Проверка на сервере:**

```bash
cd /opt/scout
docker compose -f compose.server.yaml --env-file .env.production ps
docker compose -f compose.server.yaml --env-file .env.production logs --tail 100 caddy
cat release/current_tag
curl -Iv https://<DOMAIN>/
curl -fsS https://<DOMAIN>/api/healthz
```

Ожидаемый результат: три контейнера (`api`, `web`, `caddy`) в статусе
`healthy`/`running`, `curl` к домену возвращает `200 OK`, а `release/
current_tag` содержит только что развёрнутый `sha-<...>` тег.

**Seed (первичное наполнение):**

```bash
cd /opt/scout
docker compose -f compose.server.yaml --env-file .env.production \
  --profile seed run --rm seed

# Повторный запуск должен пройти без ошибок и не задвоить фотографии —
# так проверяется идемпотентность:
docker compose -f compose.server.yaml --env-file .env.production \
  --profile seed run --rm seed
```

Откройте `https://<DOMAIN>/` в браузере и убедитесь, что галерея показывает
50 фотографий с bounding box. Только после этого — и только если вы
уверены, что все originals успешно загружены в объектное хранилище, —
можно удалить временные JPEG с сервера:

```bash
rm -rf /opt/scout/data/images
```

(`predictions.db` при этом не трогаем — он остаётся частью production
топологии.)

---

## 7.11 Обычный (routine) деплой и откат

**Обычный zero-click цикл** после активации:

1. Вы мёржите проверенный (через pull request) код в защищённую ветку
   `main`.
2. GitHub автоматически запускает `CI` и `Integration` для этого коммита;
   дождитесь, что оба завершились успешно (зелёная галочка) для одного и
   того же SHA.
3. После успешного завершения `Integration` на `main` автоматически
   запускается `Deploy`: он сверяет, что `CI` для того же SHA тоже успешен,
   собирает и публикует оба образа под неизменяемым тегом `sha-<commit>` и
   деплоит именно этот тег.
4. Никакого ручного `workflow_dispatch` или подтверждения в Environment не
   требуется — это и есть «zero-click» после первичной активации.

**Временная остановка автоматики:**

```text
Settings → Secrets and variables → Actions → Variables →
  PRODUCTION_DEPLOY_ENABLED = false
```

Это останавливает **будущие** автоматические деплои, но не останавливает уже
работающее приложение. Если какой-то запуск уже прошёл гейт CI/Integration и
начал деплоиться до того, как вы поставили `false` — он **не отменяется на
полпути** (деплои намеренно сериализованы и не отменяются в процессе), он
доработает до конца.

**Откат (rollback) к предыдущему релизу:**

```bash
ssh -i ~/.ssh/scout_deploy -p <SSH_PORT> <DEPLOY_USER>@<SERVER_IP> \
  '/opt/scout/rollback.sh'
```

Скрипт берёт **точный** предыдущий immutable-тег из
`/opt/scout/release/previous_tag` (никогда не `latest`), восстанавливает
вместе с ним сохранённую топологию (`compose.server.yaml`/`Caddyfile`,
записанные при последнем успешном деплое), выполняет ту же ограниченную по
времени проверку здоровья контейнеров и публичных `https://<DOMAIN>/` +
`https://<DOMAIN>/api/healthz`, и только при успехе меняет местами
`current_tag`/`previous_tag` (так что повторный запуск `rollback.sh`
вернёт вас обратно вперёд).

**Откат затрагивает только образы и конфигурацию** (compose/Caddyfile);
он **не трогает** `predictions.db`, объектное хранилище или данные Caddy
(сертификаты) — они не версионируются вместе с релизом образов.

**Ротация `SCOUT_API_KEY`/`VITE_SCOUT_API_KEY`** — это координированная
операция:

1. Сгенерируйте новое значение (`openssl rand -hex 32`).
2. Обновите секрет `VITE_SCOUT_API_KEY` в GitHub Environment `production`.
3. Обновите `SCOUT_API_KEY` в `/opt/scout/.env.production` **тем же
   значением**.
4. Запустите новый деплой (обычный merge в `main`, либо `workflow_dispatch`)
   — новый `web`-образ соберётся с новым встроенным ключом; старые и новые
   значения не должны существовать одновременно дольше, чем требуется на
   один цикл деплоя, иначе браузер получит `401`.

---

## 7.12 Эксплуатация, резервные копии и обновления

**Статус и логи** (безопасно смотреть из-под `<DEPLOY_USER>`, без
привилегированного доступа к внутренней сети):

```bash
cd /opt/scout
docker compose -f compose.server.yaml --env-file .env.production ps
docker compose -f compose.server.yaml --env-file .env.production logs --tail 100 api
docker compose -f compose.server.yaml --env-file .env.production logs --tail 100 web
docker compose -f compose.server.yaml --env-file .env.production logs --tail 100 caddy
curl -fsS http://127.0.0.1:2019 2>/dev/null || true   # admin API Caddy — доступен только изнутри контейнера, не с хоста
```

**Диск и память:**

```bash
df -h /opt/scout
docker system df
docker volume ls | grep scout-server   # thumb-cache, caddy-data, caddy-config
```

Помните, что кэш миниатюр (`thumb-cache`) — это **дисковый**, а не
оперативный ресурс, с ограничением по объёму (`SCOUT_THUMBNAIL_CACHE_MAX_BYTES`,
см. 7.8); он не должен расти неограниченно — если диск заканчивается,
проверьте, не был ли этот лимит увеличен без необходимости.

**Резервное копирование:**

- `.env.production` — скопируйте в защищённое хранилище секретов
  (менеджер паролей, зашифрованный бэкап), **не** в обычный файловый бэкап
  без шифрования и не в git.
- `predictions.db` — совпадает с версией в репозитории; регулярный бэкап не
  обязателен, если он не изменяется, но полезно хранить копию отдельно.
- Данные Caddy (`caddy-data` volume — сертификаты и учётная запись ACME) —
  бэкапьте, если не хотите заново проходить rate-limit Let's Encrypt при
  восстановлении; том можно скопировать командой `docker run --rm -v
  scout-server_caddy-data:/data -v $(pwd):/backup alpine tar czf
  /backup/caddy-data-backup.tar.gz -C /data .`.
- Ответственность за бэкап/версионирование самих оригиналов изображений
  лежит на вашем объектном хранилище (включите версионирование бакета там,
  где это поддерживается — это не настраивается в этом репозитории).

**Обновления Docker/ОС:** планируйте окно обслуживания; после обновления
Docker Engine перепроверьте `docker compose ps` и здоровье контейнеров.

**Обновление сертификата:** происходит автоматически внутри Caddy (том
`caddy-data` персистентен между перезапусками контейнера) — никаких ручных
действий не требуется, пока DNS продолжает указывать на сервер и порты
80/443 открыты.

**Безопасная очистка образов** (только явно неиспользуемые старые версии,
никогда не массовая очистка):

```bash
docker image ls | grep scout   # определите вручную, какие теги реально не используются
docker image rm ghcr.io/<owner>/<repo>-api:sha-<старый-неиспользуемый-тег>
```

**Никогда не выполняйте** `docker system prune -a`, `docker volume prune`,
`docker compose down -v` на production-сервере — это может удалить тома
(включая `thumb-cache`/`caddy-data`) или образы, необходимые для отката.

**Восстановление после перезагрузки сервера:** все сервисы объявлены с
`restart: unless-stopped`, поэтому Docker поднимет их автоматически при
старте демона. Проверьте после любой перезагрузки:

```bash
docker compose -f compose.server.yaml --env-file .env.production ps
curl -fsS https://<DOMAIN>/api/healthz
```

---

## 7.13 Таблица диагностики проблем

Во всех приведённых ниже командах **не выводите** содержимое
`.env.production`, заголовки `Authorization`, подписанные URL или cookies.

| Симптом | Вероятная причина | Точная проверка | Решение |
| --- | --- | --- | --- |
| SSH timeout | Файрвол блокирует порт, неверный `<SERVER_IP>`/`<SSH_PORT>` | `nc -zv <SERVER_IP> <SSH_PORT>` | Проверьте UFW-правила (7.4), IP и порт в секретах GitHub |
| SSH permission denied | Неверный/не тот приватный ключ, не тот пользователь | `ssh -v -i ~/.ssh/scout_deploy <DEPLOY_USER>@<SERVER_IP>` (смотрите verbose-вывод) | Сверьте, что публичный ключ есть в `authorized_keys`, а `DEPLOY_SSH_PRIVATE_KEY` — это именно приватная половина той же пары |
| SSH host key mismatch | Сервер переустановлен/IP переиспользован, либо `DEPLOY_SSH_KNOWN_HOSTS` устарел | `ssh-keygen -lf <(ssh-keyscan -p <SSH_PORT> <SERVER_IP>)` и сравнить fingerprint | Обновите секрет `DEPLOY_SSH_KNOWN_HOSTS` только после независимой проверки нового fingerprint (7.7) |
| Workflow не может push в GHCR | Недостаточно прав `packages: write`, либо репозиторий/организация запрещает публикацию пакетов | Посмотрите шаг "Log in to GHCR" / "Build and push" в логах запуска Actions | Проверьте Settings → Actions → General → Workflow permissions; убедитесь, что `secrets.GITHUB_TOKEN` используется, а не личный токен |
| Сервер не может стянуть приватный образ | Не выполнен `docker login ghcr.io`, либо токену не хватает `read:packages`, либо пакет не расшарен на нужный аккаунт | `docker pull ghcr.io/<owner>/<repo>-api:sha-<tag>` вручную на сервере | Выполните вход токеном с правом только `read:packages` (7.7) |
| Compose ругается на отсутствующую переменную | Значение отсутствует в `.env.production` | `docker compose -f compose.server.yaml --env-file .env.production config` (сообщение укажет точное имя переменной) | Добавьте пропущенное значение в `.env.production`, сверяясь с `deploy/.env.production.example` |
| Caddy не может выпустить сертификат | DNS ещё не указывает на сервер, порт 80/443 закрыт, либо Cloudflare проксирует до выпуска сертификата | `docker compose ... logs caddy` — ищите строки про ACME/`tls`; `dig +short <DOMAIN>` | Дождитесь распространения DNS, откройте 80/443 в UFW, временно отключите проксирование Cloudflare (см. 7.5) |
| Редирект-петля за Cloudflare | Режим SSL в Cloudflare выставлен как "Flexible", а не "Full (strict)" | Откройте сайт в браузере — цикл `https → http → https` виден в адресной строке/DevTools | Переключите Cloudflare SSL/TLS на "Full (strict)" (см. 7.5) |
| Caddy отвечает 502/503 | Контейнер `web` не поднялся/не прошёл healthcheck | `docker compose ... ps`, `docker compose ... logs web` | Проверьте здоровье `web`/`api` по отдельности; убедитесь, что `api` прошёл healthcheck прежде `web` (зависимость `depends_on: condition: service_healthy`) |
| `/api` отвечает 401/403 | `SCOUT_API_KEY` на сервере не совпадает с `VITE_SCOUT_API_KEY`, использованным при сборке web-образа | Сравните значения (не печатая их в лог) через `diff <(echo -n "$A" \| sha256sum) <(echo -n "$B" \| sha256sum)` | Синхронизируйте оба значения и передеплойте (см. ротацию ключа в 7.11) |
| API не проходит healthcheck | Неверный путь/права `SCOUT_DATABASE_PATH`, файл отсутствует | `docker compose ... logs api`; `ls -la /opt/scout/data/predictions.db` | Загрузите/поправьте права файла (7.9); путь в `.env.production` должен быть абсолютным |
| Bucket недоступен | Неверный endpoint/регион, bucket не существует, сетевые ограничения у хранилища | `curl -Iv https://<S3_ENDPOINT>/` | Сверьте значения `SCOUT_S3_*` (7.6/7.8); проверьте правила доступа в панели хранилища |
| Ошибка presigned-ссылки на original (DNS/TLS/подпись/CORS) | `SCOUT_S3_PUBLIC_ENDPOINT` не резолвится или не совпадает со схемой подписи, либо CORS bucket не разрешает браузеру | `dig +short <S3_PUBLIC_ENDPOINT>`; откройте presigned-ссылку из ответа `/api/photos` в новой вкладке (не логируя саму ссылку) | Сверьте публичный/внутренний endpoint и флаги `SECURE` (7.6); настройте CORS на bucket |
| Не та архитектура образа | Сервер ARM64, а собран только amd64 (или наоборот) | `docker image inspect <образ> --format '{{.Architecture}}'` на сервере | Убедитесь, что workflow действительно собирает `linux/amd64,linux/arm64` (уже включено в `deploy.yml`); пересоберите тег |
| Диск заполнен образами/логами/кэшем | Накопились старые неиспользуемые образы или логи контейнеров | `df -h`, `docker system df`, `du -sh /var/lib/docker/containers/*/*.log` | Удалите точечно неиспользуемые старые теги (7.12); настройте ротацию логов Docker (`log-opts` в `/etc/docker/daemon.json`), если ещё не настроена |
| Откат не находит предыдущий релиз | Это первый деплой — предыдущего релиза действительно ещё не существует | `cat /opt/scout/release/previous_tag` (ожидаемо: файла нет) | Это ожидаемое поведение для первого релиза — откатывать пока не на что; дождитесь второго успешного деплоя |

---

## 7.14 Финальный чек-лист безопасности

Отметьте каждый пункт перед тем, как считать сервер полностью готовым к
боевой эксплуатации:

- [ ] Вход по SSH-ключу для `<DEPLOY_USER>` проверен во втором сеансе;
      приватный ключ хранится только у вас, никогда не в git и не в чатах.
- [ ] Доступ к консоли восстановления провайдера сохранён и проверен.
- [ ] Файрвол (UFW) открывает только `<SSH_PORT>`, `80` и `443` — ничего
      больше.
- [ ] Compose публикует наружу только сервис `caddy`; `api`, `web` и admin
      API Caddy недоступны снаружи Docker-сети.
- [ ] DNS (`dig +short <DOMAIN>`) указывает на сервер, и HTTPS открывается
      в браузере без предупреждений сертификата.
- [ ] В GitHub настроен Environment `production` с шестью секретами (7.7),
      ограничение по ветке `main`, и репозиторная переменная
      `PRODUCTION_DEPLOY_ENABLED` существует.
- [ ] Ветка `main` защищена: обязателен pull request и успешные проверки
      `CI` + `Integration` перед слиянием.
- [ ] Реальный `/opt/scout/.env.production` имеет права `600` и никогда не
      коммитился в git.
- [ ] Права доступа к GHCR минимальны (`read:packages` на сервере,
      `packages: write` только в workflow через `GITHUB_TOKEN`).
- [ ] Bucket объектного хранилища приватный; originals отдаются только по
      подписанным URL с ограниченным сроком действия.
- [ ] `predictions.db` смонтирован только для чтения и не изменяется
      приложением.
- [ ] Резервные копии `.env.production`, данных Caddy и понимание процедуры
      восстановления существуют (7.12).
- [ ] Ни один секрет не встречается в git-истории, логах workflow, истории
      команд shell или на скриншотах, которыми вы делитесь.
