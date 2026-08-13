# AGENTS.md — api-haditssoft

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
- For each row sequentially: sends Arabic + Indonesian to opencode with the `translationSystemMessage` system prompt (Arabic = source of truth, Indonesian = reference only), then writes the returned English back via `UPDATE`
- Per-record LLM/db failures are collected, the batch continues
- Response: `{"processed": n, "updated": m, "failed": [{"nomer": x, "error": "..."}]}`
