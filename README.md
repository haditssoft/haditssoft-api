# HaditsSoft API

Backend REST API for **HaditsSoft**, a digital hadith reader and search platform. It serves 14 major kitabs with Arabic text and Indonesian translations, provides full-text search with Indonesian stemming, and powers the frontend at [haditssoft.github.io](https://github.com/haditssoft/haditssoft.github.io).

- [Features](#features)
- [Tech Stack](#tech-stack)
- [Getting Started](#getting-started)
- [Environment Variables](#environment-variables)
- [Project Structure](#project-structure)
- [API Overview](#api-overview)
- [Search Endpoints](#search-endpoints)
- [Authentication & Tokens](#authentication--tokens)
- [Admin Panel](#admin-panel)
- [Email & Verification](#email--verification)
- [Deployment](#deployment)
- [Testing](#testing)
- [Known Kitab Names](#known-kitab-names)
- [Related Documentation](#related-documentation)
- [License](#license)

---

## Features

- **Hadith data API** — load hadith, chapters, books, sanad (chain), narrator biographies, scholar comments, classifications, similar hadith, and total counts.
- **Smart search** — search within a single kitab or across multiple kitabs concurrently; uses `LIKE`, multi-keyword, and FTS5 with KBBI-based stemming for Indonesian queries.
- **Authentication** — JWT access tokens (15 min) with rotating refresh tokens (7 days) including reuse detection.
- **Email verification** — 6-digit code sign-up verification with 15-minute expiry and a 2-minute resend cooldown.
- **Forgot password** — reset password via a 6-digit email code.
- **User data** — bookmarks, notes, last-read position, theme, font, and search-mode preferences.
- **Admin API** — admin authentication, user management, and full CRUD for hadith records per kitab.
- **Utility integrations** — reCAPTCHA verification with Telegram report forwarding, and an AI assistant proxy to a local [opencode](https://opencode.ai) server.
- **Operations** — SQLite with WAL mode and a connection pool for concurrent reads, activity logging, and per-request query logs.

---

## Tech Stack

| Layer      | Technology |
|------------|------------|
| Language   | Go 1.19 |
| Web framework | [Fiber v2](https://gofiber.io) (Prefork enabled) |
| ORM        | [GORM](https://gorm.io) |
| Database   | SQLite via [glebarez/sqlite](https://github.com/glebarez/sqlite) (WAL mode) |
| Auth       | [golang-jwt](https://github.com/golang-jwt/jwt) + [gofiber/jwt](https://github.com/gofiber/jwt), bcrypt |
| Validation | [go-playground/validator](https://github.com/go-playground/validator) |
| Email      | [Hermes](https://github.com/matcornic/hermes) + [gomail](https://gopkg.in/gomail.v2) (SMTP) |
| Env        | [godotenv](https://github.com/joho/godotenv) |

---

## Getting Started

### Prerequisites

- Go 1.19 or newer
- A SQLite database file containing the hadith dataset (see [Database note](#database-note))

### Setup

```bash
# 1. Clone the repository
git clone <your-repo-url>
cd <your-repo-dir>

# 2. Create your environment file from the example
cp .env.example .env

# 3. Edit .env and fill in at least:
#    ADMIN_EMAIL, ADMIN_PASSWORD, JWT_SECRET, DB_CREDENTIAL

# 4. Run the server
go run main.go
```

The server listens on the port set by `APP_PORT` (default `8081`).

### Build

```bash
# Build binary (Windows)
go build -o app

# Build for Linux (deployment)
set GOOS=linux && go build -ldflags "-s -w" -o app
```

### Run tests

```bash
go test ./...
```

### Database note

On startup the app only auto-migrates the `User`, `Activity`, `BlacklistToken`, and `RefreshToken` tables. The hadith data lives in the SQLite file referenced by `DB_CREDENTIAL` and must already exist with:

- One table per kitab (e.g. `ShahihBukhari`, `ShahihMuslim` — see [Known Kitab Names](#known-kitab-names)).
- Optional FTS5 shadow tables named `FTS<KitabName>` (e.g. `FTSSunanIbnuMajah`) used for the Indonesian full-text search.
- A `KBBI` table (`katakunci` / `artikata` columns) used for Indonesian word stemming.

On first boot the server also creates the initial admin user from the `ADMIN_EMAIL` and `ADMIN_PASSWORD` environment variables if no user with that email exists.

---

## Environment Variables

All configuration is read from the environment (loaded from `.env` via godotenv).

| Variable               | Required | Default | Description |
|------------------------|----------|---------|-------------|
| `APP_NAME`             | no       | `HaditsSoft` | Application name, used as email sender name |
| `APP_PORT`             | yes      | `8081` | HTTP listen port |
| `APP_URL`              | no       | `http://127.0.0.1:${APP_PORT}` | Public application URL |
| `ADMIN_EMAIL`          | yes      | — | Email of the initial admin user (created on first boot) |
| `ADMIN_PASSWORD`       | yes      | — | Password of the initial admin user (bcrypt-hashed) |
| `DB_CREDENTIAL`        | yes      | — | SQLite connection string (e.g. `haditssoft.db?_busy_timeout=5000`) |
| `JWT_SECRET`           | yes      | — | Secret used to sign JWTs; the server **panics** if unset |
| `MAIL_MAILER`          | no       | `smtp` | Mail driver |
| `MAIL_HOST`            | no       | `smtp.gmail.com` | SMTP host |
| `MAIL_PORT`            | no       | `465` | SMTP port |
| `MAIL_USERNAME`        | no       | — | SMTP username |
| `MAIL_PASSWORD`        | no       | — | SMTP password / app password |
| `MAIL_ENCRYPTION`      | no       | `ssl` | Encryption type |
| `MAIL_FROM_ADDRESS`    | no       | — | Sender address |
| `MAIL_FROM_NAME`       | no       | `${APP_NAME}` | Sender display name |
| `TELEGRAM_BOT_TOKEN`   | no       | — | Bot token for Telegram report forwarding |
| `TELEGRAM_CHAT_ID`     | no       | — | Chat/group ID to receive reports |
| `RECAPTCHA_KEY`        | no       | — | reCAPTCHA v2 secret key |
| `OPENCODE_URL`         | no       | `http://127.0.0.1:4096` | Local opencode server base URL for the AI endpoint |
| `OPENCODE_PROVIDER_ID` | no       | `opencode` | AI model provider ID |
| `OPENCODE_MODEL_ID`    | no       | `deepseek-v4-flash-free` | AI model ID |
| `OPENCODE_AGENT`       | no       | `plan` | opencode agent used for `/ai/ask` |

> **Note:** `JWT_SECRET`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, and `DB_CREDENTIAL` are mandatory — the server fails to start without them.

---

## Project Structure

```
├── main.go                     # Entry point: wiring, middleware, route registration
├── internal/                   # Domain-driven application code
│   ├── hadithdata/             # Public hadith data endpoints
│   ├── search/                 # Single- and multi-kitab search
│   ├── auth/                   # Login, logout, identity, refresh (user + admin)
│   ├── user/                   # Registration, verification, forgot password, profile
│   ├── bookmark/               # Bookmarks
│   ├── note/                   # Notes
│   ├── font/                   # Font preference
│   ├── theme/                  # Theme preference
│   ├── searchmode/             # Search mode preference
│   ├── lastread/               # Last-read position
│   ├── hadithadmin/            # Admin hadith CRUD
│   ├── captcha/                # reCAPTCHA verification + Telegram reports
│   ├── opencode/               # AI assistant proxy
│   └── shared/
│       ├── auth/               # JWT helpers, bcrypt, token generation
│       ├── database/           # Connection, migrations, scopes, entities
│       ├── email/              # SMTP sender + Hermes templates
│       ├── env/                # godotenv loader
│       ├── middleware/         # JWT Protected/TokenOnly, IsAdmin, context
│       ├── response/           # Response DTOs
│       ├── utils/              # Query logging, FTS helpers, KBBI, caches
│       └── validator/          # Validation engine + custom validators
├── models/                     # GORM models + kitab index + FTS helpers
├── validations/                # Per-domain input validation structs
├── .env.example                # Environment template
└── AGENTS.md                   # Contributor / coding guidelines
```

Each domain under `internal/` follows a **handler → service → repository** pattern:

```
internal/<domain>/
├── handler.go      # HTTP layer: parse, validate, respond
├── service.go      # Business logic
├── repository.go   # Data access (GORM)
├── routes.go       # Route registration
├── dto.go          # Request/response types
└── handler_test.go # Handler tests
```

---

## API Overview

Base URL: `http://<host>:<port>`

Public endpoints need no authentication. Protected endpoints require:

```
Authorization: Bearer <access_token>
```

The standard error envelope is:

```json
{
  "status": "error",
  "message": "<description>",
  "data": null
}
```

Validation errors return `{"errors": { "<field>": "<message>" }}` with HTTP 400.

### Hadith data (public)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/loadMainData/:kitabName/:number` | Load a hadith record |
| GET | `/classificationData/:kitabName/:number/:classify` | Classification data |
| GET | `/loadCustomData/:kitabName/:number/:position/:actionId` | Custom data |
| GET | `/loadSanadHadits/:kitabName/:number` | Sanad (chain of narration) |
| GET | `/loadScholarComment/:narratorId` | Scholar comments for a narrator |
| GET | `/loadTotalHadith/:kitabName/:narratorId` | Hadith totals for a narrator |
| GET | `/loadSimilarHadith/:kitabName/:number` | Similar hadith |
| GET | `/loadCompleteProfile/:narratorId` | Complete narrator profile |
| GET | `/searchNoLain/:kitabName/:number` | Other reference numbers |
| GET | `/loadBiographyData/:kitabName` | Narrator biographies |
| GET | `/loadAllBooks/:kitabName` | List of books in a kitab |
| GET | `/loadAllChapters/endfirst/:kitabName/:start/:vSelectedK` | Chapter list (end-first) |
| GET | `/loadAllChapters/:kitabName/:start/:end` | Chapter list |
| POST | `/loadListOfRawiName/` | List of narrators |

### Search (public)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/searchHadits/:kitabName/:column` | Search a single kitab |
| POST | `/searchHadits/all/:column` | Search multiple kitabs concurrently |

See [Search Endpoints](#search-endpoints) for request/response shapes.

### Auth

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auths/login` | — | Login, returns access + refresh tokens |
| POST | `/auths/logout` | 🔒 | Blacklist current access token |
| GET | `/auths/identity` | 🔒 | Current user ID + email |
| POST | `/auths/refresh` | — | Rotate refresh token for a new pair |

### Users

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/users` | — | Sign up (sends 6-digit verification email) |
| POST | `/users/verify` | 🔑 token | Verify email with code |
| POST | `/users/verify/resend` | 🔑 token | Resend verification code |
| POST | `/users/forgot-password` | — | Request password reset code |
| POST | `/users/forgot-password/confirm` | — | Confirm reset with code + new password |
| GET | `/users/:id` | 🔒 | Get profile |
| PUT | `/users/:id` | 🔒 | Update profile |
| DELETE | `/users/:id` | 🔒 | Delete account |

`🔒` = `Protected()` (JWT + active + blacklist checks). `🔑 token` = `TokenOnly()` (JWT validity only, so inactive users can verify their email).

### User preferences (protected)

| Method | Path | Description |
|--------|------|-------------|
| GET / PUT | `/fonts` | Get / update font preference |
| GET / PUT | `/theme` | Get / update theme preference |
| GET / PUT | `/search-mode` | Get / update search mode preference |
| GET | `/lastRead/:book_name` | Get last-read position |
| PUT | `/lastRead` | Update last-read position |

### Bookmarks (protected)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/bookmarks` | Create a bookmark |
| GET | `/bookmarks` | List bookmark titles |
| GET | `/bookmarks/:title` | Get one bookmark by title |
| GET | `/bookmarks/:title/:book_name` | Get bookmark items for a book |
| PUT | `/bookmarks/:title/:book_name` | Update all bookmark items |
| DELETE | `/bookmarks/:title/:book_name` | Delete a bookmark group |

### Notes (protected)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/notes/:book_name/:hadith_id` | Create a note |
| GET | `/notes/:book_name/:hadith_id` | Get a note |
| GET | `/notes/validate-delete/:book_name/:hadith_id` | Check whether a note can be deleted |
| GET | `/notes/:book_name` | List notes for a book |
| PUT | `/notes/:book_name/:hadith_id` | Update a note |
| DELETE | `/notes/:book_name/:hadith_id` | Delete a note |

### Utility

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/verifyreCaptcha/` | — | Verify a reCAPTCHA token, then forward a report to Telegram |
| POST | `/ai/ask` | 🔒 | Ask the opencode-backed AI assistant |
| GET | `/test` | — | Health check, returns `Hello, World!` |

### Admin (JWT + IsAdmin)

All admin routes are prefixed with `/admin`.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/auths/login` | Admin login (Active + Admin users only) |
| POST | `/admin/auths/logout` | Admin logout |
| GET | `/admin/auths/identity` | Admin identity |
| POST | `/admin/auths/refresh` | Rotate admin refresh token |
| GET | `/admin/users` | List users (pagination + search) |
| GET | `/admin/users/some?filter=...` | Get users by IDs |
| GET | `/admin/users/:id` | Get a user |
| POST | `/admin/users` | Create a user |
| PUT | `/admin/users/:id` | Update a user |
| DELETE | `/admin/users/:id` | Soft-delete a user |
| DELETE | `/admin/users?ids=[...]` | Delete multiple users |
| GET | `/admin/:kitabName` | List hadith records (pagination + search) |
| GET | `/admin/:kitabName/:number` | Get one hadith record |
| POST | `/admin/:kitabName` | Create a hadith record |
| PUT | `/admin/:kitabName/:number` | Update a hadith record |
| DELETE | `/admin/:kitabName/:number` | Delete a hadith record |

---

## Search Endpoints

Two search strategies are available — the frontend picks which to call.

### Single kitab

```
POST /searchHadits/:kitabName/:column
```

Body:

```json
{
  "keyword": ["niat", "amal"]
}
```

Response — a JSON array:

```json
[rows, "SEARCHRESULTCOUNT", kitabName]
```

where `rows` is an array of `{ "<kitabIndex>": <hadithNumber> }` objects.

### Multi kitab (concurrent)

```
POST /searchHadits/all/:column
```

Body:

```json
{
  "keyword": ["niat", "amal"],
  "books": ["ShahihBukhari", "ShahihMuslim"]
}
```

`books` is required (HTTP 400 if missing or empty). Searches are run concurrently via goroutines against the SQLite WAL-enabled pool.

Response:

```json
{
  "results": {
    "ShahihBukhari": { "rows": [...], "count": 5 },
    "ShahihMuslim":  { "rows": [...], "count": 3 }
  },
  "total": 8
}
```

### Search modes

Based on keyword count, the backend dispatches to one of:

- **Single keyword** — `LIKE '%keyword%'` on the target column.
- **Multi keyword** — multiple `LIKE` conditions joined with `AND`.
- **Indonesian full-text (FTS5)** — used when the column is `Indonesia` and more than one keyword remains after filtering; combines the kitab's `FTS<KitabName>` table, KBBI stemming, phrase-variant combinations, and a `LIKE` fallback when FTS tables are missing or results are sparse.

Common conjunction words (`atau`, `dan`, `di`, `yang`, `tentang`, `hadits`, `hadis`, `hadist`, `takhrij`) are filtered out before searching. Each search writes `SEARCH_START` / `SEARCH_RESULT` entries to the daily query log.

---

## Authentication & Tokens

### Access token

| Property | Value |
|----------|-------|
| Algorithm | HS256 |
| Expiry | 15 minutes |
| Secret | `JWT_SECRET` env var |
| Claims | `user_id`, `email`, `exp` |
| Header | `Authorization: Bearer <token>` |

### Refresh token

| Property | Value |
|----------|-------|
| Format | 64 random bytes → hex string |
| Storage | SHA-256 hash in the `RefreshToken` table (plain text never persisted) |
| Expiry | 7 days |
| Rotation | Each refresh marks the old token `is_used = true` and issues a new pair |
| Reuse detection | Presenting an already-used token revokes **all** of the user's refresh tokens |

### Flow

```
1. POST /auths/login            → access_token + refresh_token
2. Use access_token in headers  → Authorization: Bearer <access_token>
3. When access_token expires    → POST /auths/refresh with refresh_token
4. POST /auths/logout           → blacklists the current access token
```

### Middleware

- `Protected()` — validates the JWT, then checks the blacklist, the user's `active` flag, and soft-deleted status.
- `TokenOnly()` — validates the JWT only (used so inactive users can verify their email).
- `IsAdmin()` — requires `active = true` and `admin = true` (used on all `/admin` routes).

---

## Admin Panel

- The initial admin is created automatically on first boot from `ADMIN_EMAIL` / `ADMIN_PASSWORD`.
- Admin identity is defined by `active = true` **and** `admin = true` on the `User` model.
- All `/admin/*` routes are guarded by `Protected()` + `IsAdmin()` except `login` and `refresh`.
- Admin mutations (login, logout, user CRUD) are recorded in the `Activity` table.

---

## Email & Verification

- Registration sends a 6-digit verification code with a **15-minute** expiry.
- `POST /users/verify` activates the account (`active = true`).
- `POST /users/verify/resend` generates a new code (old one invalidated) with a **2-minute cooldown**.
- Forgot password uses the same code mechanism, then `POST /users/forgot-password/confirm` sets the new password.
- Emails are sent over SMTP (`MAIL_*` env vars) using Hermes templates; if SMTP is not configured, verification features still function but no email is delivered.

---

## Deployment

```bash
# Cross-compile for Linux
set GOOS=linux && go build -ldflags "-s -w" -o app
```

Production hints:

- **Static files** — `app.Static("/", "./storage")` serves uploaded content from the `storage/` directory. Create it on the target machine (`mkdir storage`).
- **Database** — SQLite runs in **WAL mode** with a 10-connection pool (set on startup), which supports the concurrent multi-kitab searches. Copy your prepared `.db` file alongside the binary and point `DB_CREDENTIAL` at it. Set `_busy_timeout` (e.g. `?_busy_timeout=5000`) to avoid lock errors.
- **Secrets** — `JWT_SECRET` is mandatory; the server refuses to start without it. Never commit `.env`.
- **Prefork** — Fiber runs with `Prefork: true`, spawning one process per CPU core; ensure your deployment honors that (systemd `KillMode=process` or similar) if applicable.
- **Logs** — a daily query log `query_YYYYMMDD.log` is written to the working directory for search/query diagnostics.
- **Health** — `GET /test` returns `Hello, World!` and can be used as a basic health check.

---

## Testing

```bash
# Run the full test suite
go test ./...

# Single package
go test ./internal/auth/...

# Single test (verbose)
go test -v -run TestFunctionName ./internal/user/...
```

The suite includes 180+ tests covering domain handlers, the migration pipeline, schema drift checks, and FTS utilities.

---

## Known Kitab Names

Valid values for the `kitabName` path parameter (each is its own database table):

| Value                  | Kitab |
|------------------------|-------|
| `ShahihBukhari`        | Sahih al-Bukhari |
| `ShahihMuslim`         | Sahih Muslim |
| `SunanTirmidzi`        | Sunan al-Tirmidhi |
| `SunanAbuDaud`         | Sunan Abu Dawud |
| `SunanNasai`           | Sunan al-Nasa'i |
| `SunanIbnuMajah`       | Sunan Ibn Majah |
| `SunanDarimi`          | Sunan al-Darimi |
| `MusnadAhmad`          | Musnad Ahmad |
| `MuwathaMalik`         | Muwatta Malik |
| `SunanDaruquthni`      | Sunan al-Daraqutni |
| `ShahihIbnuKhuzaimah`  | Sahih Ibn Khuzaymah |
| `ShahihIbnuHibban`     | Sahih Ibn Hibban |
| `AlMustadrak`          | Al-Mustadrak ala al-Sahihayn |
| `MusnadSyafii`         | Musnad al-Shafi'i |

---

## Related Documentation

- [AGENTS.md](AGENTS.md) — build/test commands and coding conventions for contributors.
- [haditssoft.github.io](https://github.com/haditssoft/haditssoft.github.io) — the frontend this API powers.

---

## License

[MIT](LICENSE)
