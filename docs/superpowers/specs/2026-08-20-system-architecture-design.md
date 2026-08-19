# NeedToBuy — Overall System Architecture Design

Дата: 2026-08-20
Статус: одобрено (design), готовится план реализации.
Контекст: продолжение [[docs/mvp-decisions.md]] — та же MVP-сессия зафиксировала
продукт/сущности/стек, этот документ фиксирует техническую архитектуру
реализации на их основе.

## Цель

Определить структуру репозитория, слои бэкенда, модель данных и поверхность
API для MVP, чтобы можно было перейти к плану реализации.

## 1. Высокоуровневая архитектура

```
NeedToBuy/ (monorepo)
├── backend/            Go API (net/http + chi)
│   ├── cmd/server/      entrypoint
│   ├── internal/        доменные пакеты (см. §2)
│   ├── migrations/      golang-migrate .sql файлы
│   └── web/             embedded build фронтенда (prod)
├── frontend/            React + TypeScript SPA (Vite)
├── docker-compose.yml   Postgres (+ mailcatcher для OTP-писем в dev)
└── docs/
```

Решения:
- **Monorepo**: бэкенд и фронтенд в одном репозитории — проще для соло-MVP,
  один PR может менять API и UI вместе, один CI-пайплайн.
- **Local dev**: `docker-compose up` поднимает Postgres; Go-сервер — `go run`
  (опционально hot-reload через `air`); Vite dev server проксирует `/api/*`
  на бэкенд.
- **Production**: Go-бинарник отдаёт собранный SPA (`frontend/dist`) как
  статику и JSON API с одного домена/origin — не нужен CORS, cookie сессии
  работает без дополнительной настройки.
- **Auth**: email+OTP → verify создаёт запись в `sessions`, ставит cookie
  (`HttpOnly`, `Secure`, `SameSite=Lax`) с ID сессии. Защищённые роуты идут
  через middleware, которая по cookie подгружает `user_id` из БД.
- **DB access**: sqlx (тонкая обёртка над `database/sql`, ручной SQL + скан
  в структуры) — без ORM, соответствует принципу "без тяжёлого фреймворка".
- **Миграции**: golang-migrate, plain up/down `.sql` файлы.

## 2. Структура пакетов бэкенда

Плоская структура намеренно — соло-разработка, скоуп MVP, YAGNI по
слоям. `service.go` добавляется в конкретный пакет только когда там
появляется бизнес-логика сложнее CRUD+валидации.

```
backend/internal/
├── httpapi/       роутинг (chi), middleware (session, admin-gate, recover,
│                  logging), JSON response/error helpers
├── auth/          OTP request/verify хендлеры, sessions repository + middleware
├── child/         CRUD профиля ребёнка (handler + repository)
├── wishlist/      CRUD вишлист-айтемов, публичный read-only view для дарителя
├── catalog/       браузинг каталога (публичный/для родителя) + admin-модерация
│                  + триггер ИИ-генерации предложений
├── db/            sqlx.DB обёртка, embed.FS для миграций
└── config/        загрузка env (DB DSN, mail sender, session TTL, admin email)
```

Решения:
- **Admin-гейтинг**: без таблицы ролей — middleware в `httpapi` на
  `/api/admin/*` сравнивает `session.user.email == config.AdminEmail`.
  Соответствует MVP-решению "единственный модератор — Карина, без ролей".
- **Возрастная сетка**: Go `const`/enum пакет (`internal/agerange`), не
  таблица в БД — соответствует MVP-решению захардкодить сетку.
- **ИИ-провайдер для генерации каталога**: в архитектуре это небольшой
  интерфейс (`catalog.Suggester`), который вызывает admin-generate эндпоинт.
  Выбор конкретного вендора — отдельное решение вне скоупа этого документа.

## 3. Модель данных

```
users                  -- родители
  id, email (unique), created_at

otp_codes
  id, email, code_hash, expires_at, consumed_at, created_at

sessions
  id, user_id (FK users), expires_at, created_at

children
  id, parent_id (FK users), name, birth_date,
  public_share_token (unique, random)  -- ссылка для дарителя
  consent_child_data_at                -- согласие на обработку ПД ребёнка (152-ФЗ)
  created_at

wishlist_items
  id, child_id (FK children), title, marketplace_search_url,
  catalog_item_id (FK catalog_items, nullable)  -- заполнено, если добавлено из каталога
  created_at, updated_at

catalog_items
  id, age_range_code (enum const, не FK), category,
  title, marketplace_search_url,
  status (draft | approved | rejected),
  source (ai | admin),
  created_at, approved_at (nullable)
```

Решения:
- **Публичная ссылка дарителя**: `GET /api/public/wishlist/{public_share_token}`
  — без авторизации, read-only, отдаёт имя/возраст ребёнка + вишлист.
  Без статуса "забронировано" (registry-фича осознанно отложена).
- **Каталог → вишлист**: `wishlist_items.catalog_item_id` заполняется при
  one-click добавлении из каталога; связь "копия при добавлении" —
  title/url копируются в момент добавления, дальнейшие правки/удаление
  каталожной записи не влияют задним числом на уже добавленные вишлисты.
- **152-ФЗ**: `consent_child_data_at` на `children`, проставляется в момент
  создания профиля (чекбокс → `now()`).

## 4. Поверхность API

**Auth**
- `POST /api/auth/otp/request` `{email}` — генерирует код, отправляет письмом
  (dev: mailcatcher), rate-limit по email/IP
- `POST /api/auth/otp/verify` `{email, code}` — создаёт `users`, если новый,
  создаёт `sessions`, ставит cookie
- `POST /api/auth/logout` — удаляет сессию, чистит cookie

**Для родителя** (требует сессию)
- `GET/POST /api/children`, `PATCH/DELETE /api/children/{id}`
- `GET/POST /api/children/{id}/wishlist`, `PATCH/DELETE /api/wishlist/{id}`
- `GET /api/catalog?age_range=&category=` — браузинг для вдохновения
- `POST /api/wishlist/from-catalog/{catalog_item_id}` `{child_id}` — one-click добавление

**Публичное** (без авторизации)
- `GET /api/public/wishlist/{token}` — вид дарителя

**Admin** (сессия + admin-email гейт)
- `GET /api/admin/catalog?status=draft`
- `POST /api/admin/catalog/generate` `{age_range, category}` — вызывает
  `catalog.Suggester`, создаёт draft-записи
- `PATCH /api/admin/catalog/{id}` — редактирование / approve / reject

## 5. Обработка ошибок

Единый JSON-конверт: `{"error": {"code": "not_found", "message": "..."}}`,
маппится из внутреннего типа ошибки (`httpapi.Error{Code, HTTPStatus,
Message}`), так что хендлеры пишут `return httpapi.NotFound("child")` вместо
ручной работы со статус-кодами.

`chi` recoverer middleware ловит паники → 500 с тем же конвертом, стектрейс
логируется только на сервере.

## 6. Тестирование

- **Repository-слой**: реальный Postgres через docker-compose (без моков) —
  табличные тесты, transaction-per-test rollback для изоляции.
- **Хендлеры**: `httptest` + реальный роутер, поверх тестовой БД.
- **Фронтенд**: Vitest + React Testing Library (стандартная пара для Vite);
  не расписано подробно в этом документе — фокус на архитектуре бэкенда.

## Вне скоупа этого документа

- Выбор конкретного ИИ-провайдера для генерации каталожных предложений.
- Детальный дизайн UI/фронтенда (компоненты, роутинг SPA).
- CI/CD и деплой-инфраструктура.
- Юр.форма для выплат Ozon-партнёрки, маркировка рекламы (erid/ОРД),
  соц.вход — все осознанно отложены согласно [[docs/mvp-decisions.md]].
