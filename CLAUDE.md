# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Global Standards & Resources

**⚠️ IMPORTANT: Read global coding standards first**

📋 **Global Rules:** `~/.claude/global-rules.md`
- Code quality standards (Go best practices, security, testing)
- Git commit format and rules
- Deployment and performance guidelines

📚 **UI Components:** `~/.claude/templui-blocks.md` (222 blocks, 33 categories)
- Use templUI components for all web UI development
- Check library before implementing any UI feature
- Available: forms, cards, buttons, modals, tables, tabs, etc.

---

## Project Overview

**Hypermetrics** - A SaaS application for Hyperliquid protocol analytics, providing real-time on-chain data tracking, whale monitoring, and trading insights.

**Tech Stack:**
- **Backend:** Go 1.25 with standard library HTTP server
- **Frontend:** Templ (type-safe Go templates) + Tailwind CSS + HTMX
- **Database:** SQLite (default) or PostgreSQL
- **Payment:** Polar (default) or Stripe (via factory pattern)
- **Storage:** S3-compatible (MinIO dev, AWS/R2/DO production) or Google Cloud Storage
- **Auth:** OAuth 2.0 (Google/GitHub) + Magic Links + Password
- **Email:** Resend API

## Common Development Commands

```bash
# Start development server with hot reload (Tailwind + Templ)
task dev

# Run only Templ with hot reload and proxy
task templ

# Watch Tailwind CSS changes
task tailwind-watch

# Clean Tailwind output
task tailwind-clean

# Stop Docker services (MinIO, PostgreSQL)
task down

# Manual commands
go run cmd/server/main.go              # Run server directly
templ generate                         # Generate Go from .templ files
tailwindcss -i ./assets/css/input.css -o ./assets/css/output.css  # Build CSS
```

## Architecture & Code Structure

### Clean Architecture Pattern

**Layers (Dependency Flow: Handler → Service → Repository → Database):**

1. **Handlers** (`internal/handler/`) - HTTP request/response handling
2. **Services** (`internal/service/`) - Business logic and orchestration
3. **Repositories** (`internal/repository/`) - Data access layer
4. **Models** (`internal/model/`) - Domain entities

### Application Bootstrap Flow

1. **`cmd/server/main.go`** - Entry point, loads config
2. **`internal/config/config.go`** - Reads environment variables from `.env`
3. **`internal/app/app.go`** - Initializes services via dependency injection
4. **`internal/routes/routes.go`** - Sets up HTTP routes and middleware
5. **`internal/db/init.go`** + **`internal/db/migrations/`** - Database setup and migrations

### Key Architectural Patterns

**1. Service Container Pattern** (`internal/app/app.go`):
```go
type App struct {
    Cfg                 *config.Config
    DB                  *sqlx.DB
    AuthService         *service.AuthService
    UserService         *service.UserService
    PaymentService      payment.Provider  // Interface for pluggable providers
    // ... other services
}
```

**2. Payment Provider Factory** (`internal/service/payment/factory.go`):
- Interface-based design allowing Polar/Stripe swapping
- Provider selected via `PAYMENT_PROVIDER` env var
- Each provider implements `Provider` interface with webhooks and checkout

**3. Middleware Chain** (Applied in `internal/routes/routes.go`):
- Applied globally via `middleware.Chain()` in main.go
- Auth: `middleware.RequireAuth()` and `middleware.RequireGuest()`
- Rate limiting: `middleware.RateLimitAuth()` on auth endpoints
- CSRF: `middleware.CSRFProtect()` on state-changing requests
- Security headers, logging, recovery

**4. Repository Pattern**:
- All database queries isolated in repositories
- Uses `sqlx` for named queries and struct scanning
- Each repository focuses on one domain entity

## Frontend (Templ + HTMX)

### Templ Component Structure

```
internal/ui/
├── components/     # Reusable components (buttons, forms, cards)
├── pages/         # Full page templates
├── blocks/        # Layout sections (navbar, footer, sidebar)
├── layouts/       # Base HTML layouts
└── icons/         # SVG icon components
```

**Templ Best Practices:**
- Components are type-safe Go functions returning `templ.Component`
- Props passed as function parameters (compile-time safety)
- HTMX attributes for dynamic interactions (`hx-post`, `hx-swap`, etc.)
- Generate code: `templ generate` or use `task templ` for hot reload

### UI Component Library (TemplUI)

Configuration: `.templui.json`
- Pre-built components for common patterns
- Tailwind-based styling
- Icons, forms, buttons, modals, etc.

## Database & Migrations

**Migration System:** Goose (pressly/goose)
- Migrations in `internal/db/migrations/`
- Auto-run on server start via `db.RunMigrations()`
- SQLite and PostgreSQL dialects supported

**Key Tables:**
- `users` - Authentication (email, password_hash, OAuth IDs)
- `profiles` - User metadata (name, bio, avatar)
- `tokens` - Auth tokens (magic links, password reset, email verification)
- `subscriptions` - Billing and plan information
- `goals` + `goal_entries` - Core feature (goal tracking with steps)
- `files` - S3 file metadata (avatars, uploads)

**Switching Databases:**
```bash
# SQLite (default)
DB_DRIVER=sqlite3
DB_CONNECTION=./data/hypermetrics.db

# PostgreSQL
DB_DRIVER=postgres
DB_CONNECTION=postgresql://user:pass@localhost:5432/dbname
```

## Authentication System

**Multi-method Auth:**
1. **OAuth 2.0** - Google/GitHub (handlers: `auth.GoogleAuth`, `auth.GitHubAuth`)
2. **Magic Links** - Passwordless email login (handler: `auth.SendMagicLink`)
3. **Password** - Traditional email/password (handler: `auth.PasswordAuth`)

**Auth Flow:**
- User logs in → JWT token generated → stored in `auth` cookie (httpOnly, secure in prod)
- `middleware.RequireAuth()` validates JWT and loads user into context
- User accessible via `middleware.GetUser(r.Context())`

**Password-Optional Design:**
- Users can sign up via OAuth/magic link without setting a password
- `users.password_hash` is nullable
- Password can be set later via account settings

## Payment Integration

**Provider Interface** (`internal/service/payment/provider.go`):
```go
type Provider interface {
    CreateCheckoutSession(...)
    HandleWebhook(...)
    CancelSubscription(...)
}
```

**Supported Providers:**
- **Polar** (`internal/service/payment/polar.go`) - Default, crypto-friendly
- **Stripe** - Placeholder for future implementation

**Webhook Handling:**
- Polar webhooks → `/webhooks/polar` → `billing.PolarWebhook`
- Validates signatures, updates `subscriptions` table
- Events: `subscription.created`, `subscription.updated`, `subscription.canceled`

## File Storage

**Storage Interface** (`internal/storage/`):
- Pluggable storage backend via `Storage` interface
- Two implementations available:
  - **S3Storage** (`s3.go`) - S3-compatible (MinIO, AWS S3, R2, DO Spaces)
  - **GCSStorage** (`google.go`) - Google Cloud Storage

**S3-Compatible Storage:**
- Development: MinIO (`http://localhost:9000`)
- Production: AWS S3, Cloudflare R2, DigitalOcean Spaces
- Files stored with metadata in `files` table
- Avatar uploads: max 5MB, PNG/JPG only

**Google Cloud Storage:**
- Uses service account credentials or Application Default Credentials (ADC)
- Supports signed URLs for temporary access
- Configuration via `GCS_*` environment variables (if implemented)

**Usage:**
```go
fileService.Upload(userID, file, "avatars")  // Returns File model
fileService.GetSignedURL(fileID)             // Temporary download URL
```

## Content Management

**Markdown-Based Documentation:**
- `content/docs/` - Product documentation (features, guides, FAQ)
- `content/legal/` - Privacy policy, Terms of Service, Cookie policy
- Frontmatter parsed for metadata (title, description, order)
- Rendered via `internal/markdown/` with syntax highlighting

**Services:**
- `DocsService` - Handles `/docs/*` pages
- `LegalService` - Handles `/legal/{privacy,terms,cookies}`

## Environment Configuration

**Critical Variables (`.env`):**
```bash
# App
APP_NAME=Hypermetrics
APP_URL=https://hypermetrics.xyz
APP_ENV=production  # or development

# Database
DB_DRIVER=sqlite3  # or postgres
DB_CONNECTION=./data/hypermetrics.db

# Security
JWT_SECRET=<random-secret>
CSRF_SECRET=<random-secret>

# OAuth
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GITHUB_CLIENT_ID=...
GITHUB_CLIENT_SECRET=...

# Email
RESEND_API_KEY=...
EMAIL_FROM=noreply@hypermetrics.xyz

# Payment (Polar)
PAYMENT_PROVIDER=polar
POLAR_API_KEY=...
POLAR_WEBHOOK_SECRET=...
POLAR_PRODUCT_ID_BASIC_MONTHLY=...
POLAR_PRODUCT_ID_BASIC_YEARLY=...
POLAR_PRODUCT_ID_PRO_MONTHLY=...
POLAR_PRODUCT_ID_PRO_YEARLY=...

# S3 Storage
S3_ENDPOINT=http://localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=hypermetrics
S3_REGION=us-east-1
```

## Important Implementation Notes

### Adding New Features

1. **Define Model** in `internal/model/`
2. **Create Migration** in `internal/db/migrations/`
3. **Add Repository** in `internal/repository/` (database queries)
4. **Add Service** in `internal/service/` (business logic)
5. **Create Handler** in `internal/handler/` (HTTP endpoints)
6. **Register Routes** in `internal/routes/routes.go`
7. **Build UI** with Templ in `internal/ui/`

### Middleware Usage

**Protected Routes:**
```go
mux.HandleFunc("GET /app/dashboard", middleware.RequireAuth(handler.Dashboard))
```

**Guest-Only Routes:**
```go
mux.HandleFunc("GET /auth", middleware.RequireGuest(handler.AuthPage))
```

**Rate Limiting:**
```go
rateLimiter := middleware.RateLimitAuth()
mux.HandleFunc("POST /auth/login", rateLimiter(handler.Login))
```

### HTMX Patterns

**Partial Page Updates:**
```html
<button hx-post="/api/goal/complete" hx-target="#goal-status" hx-swap="outerHTML">
  Complete Goal
</button>
```

**Form Submissions:**
```html
<form hx-post="/app/profile" hx-swap="outerHTML">
  <!-- Form triggers HTMX request, replaces form with response -->
</form>
```

## Testing & Development

**Local Development Setup:**
1. Copy `.env.example` to `.env` (if exists) or configure `.env`
2. Start MinIO: `docker compose up -d` (optional, for file uploads)
3. Run dev server: `task dev`
4. Access: `http://localhost:7331` (proxied) or `http://localhost:8090` (direct)

**Hot Reload:**
- Templ changes → auto-regenerate and restart server
- Tailwind CSS → auto-rebuild on file changes
- Go code changes → auto-restart (via templ's `--cmd` flag)

## Deployment

**Production Build (Docker):**
```bash
docker build -t hypermetrics .
docker run -p 8090:8090 --env-file .env hypermetrics
```

**Static Binary:**
- Multi-stage Dockerfile compiles Go to static binary
- Alpine-based final image (~20MB)
- All assets embedded via `//go:embed`

## Subscription Plans

**Current Pricing Tiers:**
- **Free**: 3 goals, basic features
- **Basic**: $9/month ($86/year) - 25 goals, export, email support
- **Pro**: $19/month ($182/year) - Unlimited goals, priority support, API access

**Plan Configuration:**
- Plan limits defined in `internal/model/subscription.go`
- Polar product IDs mapped in `internal/service/payment/polar.go`
- Update pricing in:
  - `internal/ui/pages/app_billing.templ`
  - `internal/ui/pages/marketing_home.templ`
  - `content/docs/` (FAQ, getting-started, features)
  - `content/legal/terms.md`

## Legal Documents

**All payments are NON-REFUNDABLE** - explicitly stated in Terms of Service.
Contact email: `support@hypermetrics.xyz`

Update legal docs when:
- Changing data collection practices
- Adding new features that affect privacy
- Modifying payment/refund policies
- Integrating new third-party services

## Deployment Notes

**Google Cloud Run:**
- Container must listen on `0.0.0.0:$PORT` (not localhost)
- Use `PORT` env var from Cloud Run (defaults to 8080)
- Server binding in code: `http.ListenAndServe(":"+port, handler)`
- Startup timeout: default 10s, increase if needed for migrations/DB setup
