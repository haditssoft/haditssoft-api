# AGENTS.md — api-haditssoft

## Stage After Work (mandatory)

Every time you finish your work, run `git add` on your changes — and ONLY your changes. Exclude any pre-existing modified files that you did not touch. Then suggest a git commit message. Never commit or create the commit yourself.

## Build & Run

```bash
# Build binary
go build -o app

# Build for Linux (deployment)
set GOOS=linux && go build -ldflags "-s -w" -o app

# Run directly
go run main.go

# Run with live reload (requires nodemon or air)
nodemon --watch . --ext go --exec go run main.go

# Tidy dependencies
go mod tidy
```

## Test Commands

```bash
# Run all tests
go test ./...

# Run single package tests
go test ./validations/...

# Run single test function
go test -run TestFunctionName ./path/to/package

# Verbose
go test -v -run TestFunctionName ./path/to/package
```

## Project Structure
```
├── main.go                     # Entry point
├── internal/                   # Domain packages (handler/service/repository)
├── models/                     # GORM models + search helpers
├── validations/                # Input validation structs
└── .env.example                # Environment template
```

## Code Style Guidelines

### Imports
Three groups separated by blank lines, each sorted alphabetically:
1. Standard library
2. Third-party (external modules)
3. Internal/local (`github.com/haditssoft/haditssoft-backend/...`)

Local imports use aliases with descriptive prefixes when package name differs:
```go
import (
    "errors"
    "time"

    "github.com/gofiber/fiber/v2"
    "gorm.io/gorm"

    "github.com/haditssoft/haditssoft-backend/databases/connections"
    "github.com/haditssoft/haditssoft-backend/models"
    noteValidations "github.com/haditssoft/haditssoft-backend/validations/note"
)
```

### Formatting
- Tabs for indentation (standard `go fmt`)
- File naming pattern: `<name>.<category>.go` (e.g. `auth.controller.go`, `user.model.go`, `auths.route.go`, `create.user.validation.go`)
- Run `go fmt ./...` before committing

### Types & Structs
- PascalCase for exported types: `UserCreate`, `AdminMainData`, `UserResponseField`
- JSON tags in snake_case: `json:"created_at"`
- GORM tags with `gorm:"notNull;size:50"`
- Form tags alongside JSON tags on validation structs: `form:"email" json:"email"`
- Response DTOs in `responses/` package, suffix `ResponseField`

### Naming Conventions
- **Exported functions**: PascalCase (`LoadMainData`, `ValidateModel`)
- **Unexported functions**: camelCase (`getUserByEmail`, `dbColumnName`)
- **Variables**: camelCase (`modelValidation`, `allErrors`, `responseModel`)
- **Interfaces**: PascalCase (`CRUDController`, `AuthController`)
- **Package-level vars**: PascalCase if exported (`DB`, `TrCtx`), camelCase if private (`uni`, `validate`)
- **Constants**: PascalCase (`TrCtx`, `ErrorWhenValidate`)
- **File naming**: `<name>.<category>.go` pattern throughout

### Controllers
- Define struct types with empty bodies: `type Auth struct{}`
- Implement interfaces from `controllers/controller.go`:
  - `CRUDController` (GetList, GetOne, GetSome, Create, Update, DeleteOne, DeleteSome)
  - `AuthController` (Login, Logout, Identity, Refresh)
  - `OptionsController` (GetDataForSelect)
- Method receivers: pointer `(ctl *Auth)`

### Routes
- Function signature: `func RouteName(rg fiber.Router)`
- Group endpoints under a path: `app := rg.Group("/auths")`
- Dispatch via interface: `var ctrl controllers.AuthController = new(front.Auth)`
- Or directly: `ctrl := new(front.MainData)`

### Error Handling
- Controllers return `c.Status(code).JSON(fiber.Map{...})` — never panic
- GORM errors: check `errors.Is(err, gorm.ErrRecordNotFound)` and `result.RowsAffected`
- Transactions: `connections.DB.Transaction(func(tx *gorm.DB) error { return err })`
- Body parsing errors: return `fiber.StatusInternalServerError`
- Validation errors: return `fiber.StatusBadRequest` with `fiber.Map{"errors": allErrors}`
- Success responses: `c.JSON(responseModel)` or `c.SendStatus(fiber.StatusNoContent)`

### Models (GORM)
- Package `models`, table name via `func (User) TableName() string { return "User" }`
- Naming strategy: `NoLowerCase: true`, `SingularTable: true` (PascalCase table/column names)
- Hooks: `BeforeSave`, `BeforeCreate`, `BeforeUpdate`, `AfterFind`, `AfterCreate`, `AfterUpdate`, `AfterDelete`
- Activity logging within same transaction for Create/Update/Delete
- DB access via `connections.DB` singleton

### Validation
- Validation structs in `validations/<entity>/` with `validate:"..."` tags
- Central engine in `validations/validator.go` with `ValidateModel(model)` returning `map[string]interface{}`
- Custom validators registered via `validations.RegisterCustomValidations()`
- Error messages via `errorMessage()` switch function

### Middleware
- JWT auth: `middlewares.Protected()` from `github.com/gofiber/jwt/v3`
- Admin guard: `middlewares.IsAdmin` blocks non-admin users
- Context propagation via `SetConexContext(c)` in `conexContext.middleware.go`
- Response shape for auth errors: `{"status": "error", "message": "...", "data": nil}`

### API Patterns
- Framework: Fiber v2 with `Prefork: true`
- CORS: `AllowOrigins: "*"`, `AllowHeaders: "*"`, expose `X-Total-Count`
- Static files: `app.Static("/", "./storage")`
- Pagination responses: `{"data": results, "total": total, "page": page, "limit": limit}`
- Success shape: `{"status": "success", "message": "...", "token": ...}` (varies)
- Login/Refresh response includes `refresh_token` alongside `token`
- Copy models to response DTOs: `copier.Copy(responseModel, &model)`

### Auth / Refresh Token Flow
- **Access token**: short-lived (15 min), signed JWT with `user_id` + `email` claims
- **Refresh token**: long-lived (7 days), opaque random string stored as SHA-256 hash in `RefreshToken` table
- **Rotation**: each refresh marks the old token as `is_used = true`, inserts a new record
- **Reuse detection**: if a refresh token is presented when `is_used = true`, all the user's tokens are revoked (force re-login)
- **Endpoints**:
  - `POST /auths/login` → `{"token": "<access>", "refresh_token": "<plain>"}`
  - `POST /auths/refresh` (body: `{"refresh_token": "..."}`) → `{"token": "<new_access>", "refresh_token": "<new_plain>"}`
  - `POST /auths/logout` (protected) → blacklists access token
- **Env**: `JWT_SECRET` env var (falls back to hardcoded value)
- **Constants**: `AccessTokenExpiry = 15min`, `RefreshTokenExpiry = 7d` in `authentications/authentication.go`

### Search Endpoints
Two search strategies available, frontend chooses which to call:

**Single kitab** (original, unchanged):
- `POST /searchHadits/:kitabName/:column`
- Body: `{"keyword": ["..."]}`
- Returns: `[rows, "SEARCHRESULTCOUNT", kitabName]`

**Multi kitab** (concurrent, new):
- `POST /searchHadits/all/:column`
- Body: `{"keyword": ["..."], "books": ["ShahihBukhari", "ShahihMuslim"]}`
- Searches specified books concurrently via goroutines
- Returns single JSON with results grouped by kitab:
```json
{
  "ShahihBukhari": { "rows": [...], "count": 5 },
  "ShahihMuslim":  { "rows": [...], "count": 3 },
  "total": 8
}
```
- `books` is required (400 if missing/empty)
- Uses `searchOneKitab` helper which dispatches to `singleKeywordSearch`, `multiKeywordLikeSearch`, or `indonesiaFTSearch` based on keyword count
- DB pool: `SetMaxOpenConns(10)` + WAL mode enables concurrent reads

### Cron Endpoint (translate missing English)
- `POST /ai/cron/translate/:kitabName?key=<OPENCODE_CRON_KEY>&limit=10`
- Cron-only: guarded by `OPENCODE_CRON_KEY` env var passed as `?key=` query param (constant-time compare, `401` if missing/wrong) — NOT JWT-protected
- `kitabName` must be in `models.GetIndexOfKitab` whitelist (`400` otherwise)
- Selects rows where `English IS NULL OR English = ''`, ordered by `Nomer`, limited by `?limit=` (default 10, must be ≥ 1)
- For each row sequentially: runs `opencode run --format json --agent translate` with the Arabic+Indonesian prompt, parses NDJSON response for text events, writes the returned English back via `UPDATE`
- Translation instructions are defined in `.opencode/agents/translate.md` (automatically loaded by `--agent translate`)
- Per-record CLI/db failures are collected, the batch continues; empty/whitespace CLI replies are counted as failed (not written back)
- Response: `{"processed": n, "updated": m, "failed": [{"nomer": x, "error": "..."}]}`
- Uses the request context (`c.Context()`) for subprocess cancellation; batch aborts remaining rows if the client disconnects

### Bulk Translate Endpoint (translate many hadiths in one AI call)
- `POST /ai/cron/translate/bulk/:kitabName?key=<OPENCODE_CRON_KEY>&limit=10`
- Same cron-key guard (`?key=`), kitab whitelist, and `?limit=` rules as the single-kitab cron endpoint
- Selects the same untranslated rows (`English IS NULL OR English = ''`), then sends them all in ONE AI call:
  - Input prompt is a JSON object keyed by hadith Nomer: `{"<Nomer>": {"arabic": "...", "indonesia": "..."}}` (Arabic primary, Indonesian reference)
  - Agent: `opencode run --format json --pure --agent translate-bulk` — instructions in `.opencode/agents/translate-bulk.md`
  - Reply must be a JSON object keyed by the SAME Numers with the English translation as each value: `{"<Nomer>": "<english>"}` (NOT a nested `{arabic, indonesia, english}` object)
- Reply parsing is defensive: trims, tolerates markdown fences/pre/trailing text, extracts the `{...}` block, then unmarshals into `map[string]string`; forbidden/nested values or invalid JSON fail the whole batch
- Writes each translation back via `UPDATE ... SET English = ? WHERE Nomer = ?`; missing/empty keys for requested Numers are collected in `failed`; keys in the reply that were NOT requested are ignored (never written)
- Response: `{"processed": n, "updated": m, "failed": [{"nomer": x, "error": "..."}]}`
- Uses the request context (`c.Context()`) for subprocess cancellation
- **Sweep mode** (`?all=1`): keeps draining the table in batches of `?limit=` until no untranslated rows remain. Batches advance by `Nomer ASC` and never resend an already-attempted Nomer, so a failing batch is reported in `failed` and skipped (guaranteed termination, no infinite loop). `?maxBatches=N` (≥ 1) caps the number of batches in one request; absent = unbounded. Without `?all=1`, a single batch is processed (default, backward compatible)

### Title Bulk Translate Endpoint (Kitab/Bab titles in one AI call)
- `POST /ai/cron/translate/title/bulk/:kitabName?key=<OPENCODE_CRON_KEY>&limit=10&type=both`
- Same cron-key guard (`?key=`), kitab whitelist, `?limit=`, `?all=1`, and `?maxBatches=` rules as the hadith bulk endpoint (shared engine in `internal/opencode/translate-bulk.handler.go`)
- `?type=` selects which tables to translate: `kitab`, `bab`, or `both` (default). Invalid `?type=` → `400`
- Translates **book titles** (`Kitab<kitabName>`):
  - Rows: `NKitabEng IS NULL OR NKitabEng = ''`, ordered by `VMember ASC`
  - Input prompt keyed by VMember id: `{"<VMember>": {"arabic": "<NKitabArab>", "indonesia": "<NKitab>"}}`
  - Writes back via `UPDATE ... SET NKitabEng = ? WHERE VMember = ?`
- Translates **chapter titles** (`Bab<kitabName>`):
  - Rows: `NBabEng IS NULL OR NBabEng = ''`, ordered by `VMemberBab ASC`
  - Input prompt keyed by VMemberBab id: `{"<VMemberBab>": {"arabic": "<NBabArab>", "indonesia": "<NBab>"}}`
  - Writes back via `UPDATE ... SET NBabEng = ? WHERE VMemberBab = ?`
- Agent: `opencode run --format json --pure --agent translate-title-bulk` — instructions in `.opencode/agents/translate-title-bulk.md`
- Reply parsing, failure handling, and sweep semantics are identical to the hadith bulk endpoint; each table is swept independently
- Response includes aggregate totals plus a per-table breakdown (so `failed` ids cannot be misattributed):
```json
{
  "processed": n, "updated": m, "failed": [...],
  "kitab": {"processed": a, "updated": b, "failed": [...]},
  "bab":   {"processed": c, "updated": d, "failed": [...]}
}
```
- A skipped table (e.g. `?type=kitab`) reports a zeroed sub-object; `failed` entries use `{"nomer": <id>, "error": "..."}` where `nomer` is the VMember/VMemberBab value

### AI Ask Endpoint (context + timeout)
- `POST /ai/ask` (JWT-protected)
- The subprocess is bound to the HTTP request context via `exec.CommandContext`, so when the client disconnects, the `opencode` subprocess is killed immediately (no zombie processes)
- A hard timeout is enforced with `context.WithTimeout`: default **90s** (`defaultAskTimeoutSec` in `internal/opencode/handler.go`), configurable via `OPENCODE_ASK_TIMEOUT_SEC` env var (must be ≥ 1; invalid/zero values fall back to the default)
- On timeout, the handler returns `502 Bad Gateway` with `{"error": "failed to get AI response"}` (the exec error is logged but not exposed)

## Test Coverage (mandatory)

Every change to application code must ship with tests. Run the full suite before finishing: `go test ./...` — all tests must pass.

Coverage expectations:

- Every non-trivial change to handlers, services, repositories, models, routes, or shared logic ships with tests covering the happy path, validation failures, auth/guard failures, and every error branch (exec/DB/parse/not-found).
- Pure logic (parsers, flag builders, search helpers, validators) uses table-driven tests (`tests := []struct{...}` + `t.Run(name, ...)`), including edge cases: empty input, malformed input, duplicates, whitespace/trimming, boundary indices.
- HTTP endpoints are tested at the route level via `app.Test(...)`: success, 4xx/5xx failures, wrong HTTP method, missing/invalid body, no/invalid/expired token, deleted/inactive resource. Assert the status code, response body shape (e.g. `status`/`message`/`errors`/`error` keys), and DB side effects (e.g. refresh-token rotation + `is_used`, blacklist count, activity log rows).

Rules:

- Test files live beside the code in the **same package** (white-box, `package <pkg>`), named `<subject>_test.go`; group shared helpers in one `_test.go` per package (e.g. `handler_test.go`).
- Per package, define: `setup<X>TestDB(t)` (in-memory SQLite via `github.com/glebarez/sqlite`, `NamingStrategy{SingularTable: true, NoLowerCase: true}`, unique DB name per test via a package-level counter, assign to `database.DB`, restore/close in `t.Cleanup`), `setup<X>TestApp(t)` (`fiber.New()` + `RegisterRoutes(...)`), a `make<X>Request(t, app, method, path, body, token)` helper returning `*http.Response`, and a `decode<X>JSON(t, resp, dest)` helper.
- Set prerequisite env (e.g. `JWT_SECRET`) and call `validator.RegisterCustomValidations()` inside `TestMain(m *testing.M)` when the package needs them; other env vars are set per test and unset via `t.Cleanup`.
- Mock external/OS boundaries by overriding a package-level var (e.g. `execCommandFunc`) in the test and restoring the original in `t.Cleanup`; never call the real `opencode` CLI, network, or production DB from tests.
- Use only the standard `testing` package for assertions (no testify/gomega). Use `t.Helper()` on all helpers, `t.Run` for subtests, and error messages in the form `status = %d, want %d` / `got %q, want %q`.
- Test names follow `Test<Subject>_<Outcome>` (e.g. `TestLogin_Success`, `TestRefresh_ReuseDetection`, `TestRoute_AskNoAuth`, `TestParseOpenCodeNDJSON`); group endpoint tests with `// ===...===` banner comments.
- Keep tests deterministic, independent, and runnable in parallel-by-default mode: the only shared mutable global allowed is `database.DB`, and it is swapped/restored per test via `t.Cleanup`.
