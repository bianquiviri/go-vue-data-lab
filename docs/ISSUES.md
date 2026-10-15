# Project Issues & Tasks - Go Concurrent Data Processing Lab

## Phase 1: Infrastructure Setup (Priority: High)

### Issue #1: Docker Compose Environment
- [x] Create deploy/docker-compose.yml with PostgreSQL and Caddy
- [x] Create deploy/Caddyfile for reverse proxy
- [x] Create deploy/init-db.sh for database initialization
- [x] Configure mkcert for local TLS (godata.josebianco.local)
- [x] Document setup in README

### Issue #2: Database Migrations (Source & Target)
- [ ] Create migrations/source/001_initial.sql (UP/DOWN)
- [ ] Create migrations/source/002_seeders.sql (UP/DOWN) - idempotent
- [ ] Create migrations/target/001_initial.sql (UP/DOWN)
- [ ] Create migrations/target/002_control_tables.sql (UP/DOWN) for cron_schedules, job_executions
- [ ] Create migrations/target/003_admin_bootstrap.sql (UP/DOWN) for root user

### Issue #3: Environment Configuration
- [ ] Create .env.example with all required variables
- [ ] Implement config package (backend/internal/config/config.go)
- [ ] Load config from .env with validation
- [ ] Support DB connections for lab_source and lab_target
- [ ] Support JWT, admin bootstrap, logging config

## Phase 2: Backend Core (Priority: High)

### Issue #4: Go Module & Project Structure
- [ ] Initialize go.mod with required dependencies (gin, gorm, zap, cron, jwt, argon2, etc.)
- [ ] Setup cmd/api/main.go with graceful shutdown
- [ ] Setup database connections for both schemas
- [ ] Setup zap logging with lumberjack rotation
- [ ] Setup GORM with PostgreSQL (no AutoMigrate)
- [ ] Implement clean architecture layers (handler, service, repository, middleware)

### Issue #5: Domain Models
- [ ] Customer model (UUID, timestamps, validation)
- [ ] Product model (UUID, SKU, pricing, stock, category)
- [ ] Order model (UUID, order_number, customer_id, status, total_amount, shipping_address JSONB)
- [ ] OrderItem model (BigSerial, order_id, product_id, quantity, unit_price)
- [ ] CronSchedule model (for scheduler persistence)
- [ ] JobExecution model (for job tracking and history)
- [ ] User model with roles (admin/operator/viewer), password hash, status

### Issue #6: Authentication & Authorization
- [x] Argon2id password hashing implementation
- [x] Bootstrap admin (root@gmail.com) from env vars on startup
- [x] JWT token generation/validation (access + refresh)
- [x] Auth middleware for protected routes
- [x] RBAC middleware for role-based access control
- [x] Auth endpoints: POST /api/v1/auth/login, POST /api/v1/auth/refresh, GET /api/v1/auth/me
- [x] Password change and logout endpoints

## Phase 3: Synthetic Data Generation (Priority: High)

### Issue #7: E-commerce Synthetic Data Generator
- [ ] Configurable record counts (millions supported)
- [ ] Reproducible generation with seed
- [ ] Generate customers with realistic names/emails
- [ ] Generate products with SKUs, categories, prices, stock
- [ ] Generate orders linked to random customers
- [ ] Generate order_items linked to orders/products with quantities/pricing
- [ ] Idempotent seeding (avoid duplicates)
- [ ] CLI command or API endpoint to trigger generation
- [ ] Progress reporting during generation

## Phase 4: Concurrent Processing (Priority: High)

### Issue #8: Worker Pool Implementation
- [ ] Generic worker pool with configurable number of workers
- [ ] Job queue using channels
- [ ] Context with cancellation support
- [ ] Panic recovery in workers
- [ ] Batch processing (3000-4000 records per batch)
- [ ] Error handling with fmt.Errorf wrapping
- [ ] Progress tracking and metrics
- [ ] Graceful shutdown

### Issue #9: Data Migration/ETL Worker
- [ ] Extract from lab_source (customers, products, orders, order_items)
- [ ] Transform/validate data (normalize, compute totals, deduplicate)
- [ ] Load to lab_target using SQL Raw for bulk operations
- [ ] Upsert operations (idempotent) with conflict handling
- [ ] Transaction management per batch
- [ ] Metrics collection (processed, inserted, updated, errors)
- [ ] Resume capability on failure
- [ ] Detailed logging with zap

## Phase 5: API Layer (Priority: High)

### Issue #10: REST API Core
- [ ] Health check endpoint: GET /api/v1/health
- [ ] API versioning (/api/v1)
- [ ] Request validation with binding
- [ ] Consistent error response format
- [ ] CORS configuration
- [ ] Rate limiting (if needed)
- [ ] Swagger/OpenAPI 3.x integration
- [ ] Swagger UI at /swagger/index.html

### Issue #11: Source Data Endpoints
- [ ] GET /api/v1/source/customers (pagination, filtering, search)
- [ ] GET /api/v1/source/customers/:id
- [ ] GET /api/v1/source/products (pagination, filtering by category)
- [ ] GET /api/v1/source/products/:id
- [ ] GET /api/v1/source/orders (pagination, filtering by customer/status)
- [ ] GET /api/v1/source/orders/:id with items
- [ ] GET /api/v1/source/order-items (pagination)

### Issue #12: Target Data Endpoints
- [ ] GET /api/v1/target/customers (pagination, filtering)
- [ ] GET /api/v1/target/products (pagination, filtering)
- [ ] GET /api/v1/target/orders (pagination, filtering)
- [ ] GET /api/v1/target/orders/:id with items
- [ ] GET /api/v1/target/analytics/summary
- [ ] GET /api/v1/target/analytics/top-products
- [ ] GET /api/v1/target/analytics/top-customers

### Issue #13: Worker & Job Control Endpoints
- [ ] POST /api/v1/jobs/generate/start (start synthetic data generation)
- [ ] POST /api/v1/jobs/migrate/start (start ETL migration)
- [ ] POST /api/v1/jobs/stop (stop running job)
- [ ] GET /api/v1/jobs/status (current job status)
- [ ] GET /api/v1/jobs/history (job executions history with pagination)
- [ ] GET /api/v1/jobs/progress (real-time progress)

### Issue #14: Scheduler (Cron) Endpoints
- [ ] GET /api/v1/scheduler/jobs (list cron schedules)
- [ ] POST /api/v1/scheduler/jobs (create cron schedule)
- [ ] PUT /api/v1/scheduler/jobs/:id (update cron schedule)
- [ ] DELETE /api/v1/scheduler/jobs/:id (delete cron schedule)
- [ ] POST /api/v1/scheduler/jobs/:id/enable
- [ ] POST /api/v1/scheduler/jobs/:id/disable
- [ ] POST /api/v1/scheduler/jobs/:id/run (trigger immediately)
- [ ] Integrate robfig/cron/v3 with PostgreSQL persistence

### Issue #15: Admin & Monitoring Endpoints
- [ ] GET /api/v1/admin/logs/errors (from zap+lumberjack)
- [ ] GET /api/v1/admin/users (list users - admin only)
- [ ] POST /api/v1/admin/users (create user - admin only)
- [ ] PUT /api/v1/admin/users/:id (update user - admin only)
- [ ] DELETE /api/v1/admin/users/:id (delete user - admin only)
- [ ] GET /api/v1/admin/system/stats

### Issue #16: Server-Sent Events (SSE)
- [ ] SSE endpoint for real-time job progress: GET /api/v1/jobs/stream
- [ ] Broadcast job progress updates to connected clients
- [ ] Handle client disconnects gracefully
- [ ] Heartbeat mechanism to keep connections alive
- [ ] Authentication for SSE endpoint

## Phase 6: Frontend (Vue 3) (Priority: High)

### Issue #17: Frontend Setup
- [ ] Initialize Vue 3 + TypeScript + Vite in web/
- [ ] Configure Tailwind CSS
- [ ] Configure shadcn-vue components
- [ ] Setup vue-router with routes
- [ ] Setup pinia for state management
- [ ] Setup axios for API calls
- [ ] Configure environment variables (VITE_API_BASE_URL)
- [ ] Theme system (Light/Dark/System) with persistence

### Issue #18: Authentication UI
- [ ] Login page with form validation
- [ ] JWT token storage (httpOnly considerations or localStorage with refresh)
- [ ] Auth store (pinia)
- [ ] Route guards for protected routes
- [ ] Logout functionality
- [ ] User profile display

### Issue #19: Dashboard & Layout
- [ ] Main layout with sidebar/navigation
- [ ] Dashboard with system overview stats
- [ ] Navigation based on user role (RBAC)
- [ ] Theme toggle component
- [ ] Responsive design

### Issue #20: Data Tables & Views
- [ ] Reusable data table component with pagination
- [ ] Source data views (Customers, Products, Orders)
- [ ] Target data views (Customers, Products, Orders)
- [ ] Order detail view with order items
- [ ] Filtering and search functionality
- [ ] Sorting capabilities

### Issue #21: Job Monitoring & Control
- [ ] Job control panel (start/stop generate/migrate jobs)
- [ ] Real-time job progress via SSE
- [ ] Job history table
- [ ] Job status indicators
- [ ] Error display for failed jobs

### Issue #22: Scheduler Management UI
- [ ] Cron jobs list view
- [ ] Create/Edit cron job form
- [ ] Enable/Disable cron jobs
- [ ] Trigger cron job manually
- [ ] Cron expression validation

### Issue #23: Admin Panel
- [ ] User management (CRUD) - admin only
- [ ] Error logs viewer from /api/v1/admin/logs/errors
- [ ] System statistics display
- [ ] Role assignment

### Issue #24: Analytics Dashboard
- [ ] Summary cards (total customers/products/orders)
- [ ] Top products chart/table
- [ ] Top customers chart/table
- [ ] Revenue/order statistics
- [ ] Data visualization (charts)

## Phase 7: Testing & Quality (Priority: High)

### Issue #25: Backend Testing
- [ ] Unit tests for config package
- [ ] Unit tests for models/validation
- [ ] Unit tests for auth utilities (hashing, JWT)
- [ ] Unit tests for worker pool
- [ ] Unit tests for ETL logic
- [ ] Integration tests for API endpoints
- [ ] Repository tests with test DB
- [ ] Service layer tests with mocks
- [ ] Achieve >= 80% code coverage
- [ ] Run go test -race
- [ ] Run go vet
- [ ] Run golangci-lint

### Issue #26: Frontend Testing
- [ ] Unit tests for components (Vitest)
- [ ] Unit tests for stores (Pinia)
- [ ] Unit tests for composables
- [ ] Component tests with Vue Test Utils
- [ ] API service tests with mocks
- [ ] Achieve >= 80% code coverage
- [ ] E2E tests for critical flows (optional but recommended)

### Issue #27: End-to-End & Integration
- [ ] Test full flow: generate synthetic data → migrate via workers → view in UI
- [ ] Test auth flow (login, RBAC, logout)
- [ ] Test SSE real-time updates
- [ ] Test scheduler functionality
- [ ] Test error handling and recovery

## Phase 8: DevOps & Deployment (Priority: Medium)

### Issue #28: Docker & Local Environment
- [ ] Complete docker-compose.yml with all services
- [ ] PostgreSQL with init scripts
- [ ] Caddy with TLS via mkcert
- [ ] Volume persistence for DB and logs
- [ ] Health checks for services
- [ ] .env integration

### Issue #29: Makefile & Scripts
- [ ] make build (backend build)
- [ ] make run (backend run)
- [ ] make test (go test -race -v ./...)
- [ ] make lint (golangci-lint)
- [ ] make migrate-up / migrate-down (migrations)
- [ ] make docker-up / docker-down
- [ ] make swagger (generate swagger docs)
- [ ] make seed / make generate (if applicable)

### Issue #30: Documentation & Polish
- [ ] Complete README.md with setup instructions
- [ ] API documentation via Swagger
- [ ] Architecture diagram (optional)
- [ ] Contributing guidelines
- [ ] Troubleshooting section
- [ ] Verify all commands from Makefile work
- [ ] Update docs/MEMORY.md with final state
