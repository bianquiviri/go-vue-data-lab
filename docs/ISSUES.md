# Project Issues & Tasks - Go Concurrent Data Processing Lab

## Phase 1: Infrastructure Setup (Priority: High)

### Issue #1: Docker Compose Environment
- [ ] Create deploy/docker-compose.yml with PostgreSQL and Caddy
- [ ] Create deploy/Caddyfile for reverse proxy
- [ ] Create deploy/init-db.sh for database initialization
- [ ] Configure mkcert for local TLS (godata.josebianco.local)
- [ ] Document setup in README

### Issue #2: Database Migrations (Source & Target)
- [ ] Create migrations/source/001_initial.sql (UP/DOWN)
- [ ] Create migrations/source/002_seeders.sql (UP/DOWN) - idempotent
- [ ] Create migrations/target/001_initial.sql (UP/DOWN)
- [ ] Create migrations/target/002_control_tables.sql (UP/DOWN) for cron_schedules, job_executions

## Phase 2: Backend Core (Priority: High)

### Issue #3: Go Module & Project Structure
- [ ] Initialize go.mod with required dependencies
- [ ] Setup cmd/api/main.go
- [ ] Implement config management
- [ ] Setup database connections for both DBs
- [ ] Setup zap logging with lumberjack
- [ ] Setup GORM with PostgreSQL

### Issue #4: Domain Models
- [ ] Customer model (UUID, timestamps)
- [ ] Product model (UUID, SKU, pricing, stock)
- [ ] Order model (UUID, order_number, JSONB shipping_address)
- [ ] OrderItem model (BigSerial)
- [ ] CronSchedule model
- [ ] JobExecution model
- [ ] User model with roles (admin/operator/viewer)

### Issue #5: Authentication & Authorization
- [ ] Argon2id/bcrypt password hashing
- [ ] Bootstrap admin (root@gmail.com) from env vars
- [ ] JWT middleware
- [ ] RBAC middleware for role-based access
- [ ] Auth endpoints (login, refresh, me)

## Phase 3: Synthetic Data Generation (Priority: High)

### Issue #6: E-commerce Synthetic Generator
- [ ] Configurable record counts (millions)
- [ ] Reproducible with seed
- [ ] Generate customers with realistic data
- [ ] Generate products with SKUs, categories, prices
- [ ] Generate orders with random customers/products
- [ ] Generate order_items with quantities/pricing
- [ ] Idempotent seeding

## Phase 4: Concurrent Processing (Priority: High)

### Issue #7: Worker Pool Implementation
- [ ] Worker pool with goroutines & channels
- [ ] Context with cancellation
- [ ] Panic recovery
- [ ] Batch processing (3000-4000 records)
- [ ] Error handling with fmt.Errorf wrapping
- [ ] Progress tracking

### Issue #8: Data Migration/ETL Worker
- [ ] Extract from lab_source
- [ ] Transform/validate data
- [ ] Load to lab_target using SQL Raw for bulk ops
- [ ] Upsert operations (idempotent)
- [ ] Transaction management
- [ ] Metrics collection

## Phase 5: API Layer (Priority: High)

### Issue #9: REST Endpoints
- [ ] Health check
- [ ] Source data CRUD/views (customers, products, orders)
- [ ] Target data CRUD/views with pagination
- [ ] Worker/job control endpoints
- [ ] Scheduler endpoints (cron)
- [ ] Admin logs endpoint (protected)
- [ ] SSE endpoint for real-time metrics

### Issue #10: OpenAPI/Swagger
- [ ] Swagger annotations
- [ ] Swagger UI at /swagger/index.html
- [ ] Security definitions (JWT)

## Phase 6: Scheduler (Priority: Medium)

### Issue #11: Cron Scheduler with Persistence
- [ ] Integrate robfig/cron/v3
- [ ] Store schedules in PostgreSQL (cron_schedules)
- [ ] Track job executions (job_executions)
- [ ] REST API for managing schedules

## Phase 7: Frontend (Priority: High)

### Issue #12: Vue 3 + Vite + shadcn-vue Setup
- [ ] Initialize Vite + Vue 3 + TypeScript
- [ ] Configure Tailwind CSS
- [ ] Install shadcn-vue components
- [ ] Setup routing (vue-router)
- [ ] Setup state management (pinia)
- [ ] API client (axios)

### Issue #13: Theme System
- [ ] Light/Dark/System themes
- [ ] Theme selector component
- [ ] Persist theme preference in localStorage
- [ ] Apply theme on app load

### Issue #14: Dashboard & Data Views
- [ ] Dashboard layout with sidebar/nav
- [ ] Parallel tables for source and target data
- [ ] Pagination for both tables
- [ ] Filtering/searching capabilities
- [ ] Clean, responsive UI

### Issue #15: Job Monitoring & Management
- [ ] Real-time job progress via SSE
- [ ] Start/stop/cancel jobs
- [ ] Job history/executions
- [ ] Progress bars/charts
- [ ] Metrics display

### Issue #16: Target Data Editing
- [ ] Edit forms for target records
- [ ] Validation
- [ ] Save/cancel operations
- [ ] Optimistic UI updates

## Phase 8: Testing & Quality (Priority: High)

### Issue #17: Backend Testing (TDD)
- [ ] Unit tests with ≥80% coverage
- [ ] Integration tests for DB operations
- [ ] Worker pool tests with race detection
- [ ] API endpoint tests
- [ ] Run go test -race ./...
- [ ] Run go vet

### Issue #18: Frontend Testing
- [ ] Unit tests with Vitest
- [ ] Component tests
- [ ] ≥80% statements coverage
- [ ] Linting with ESLint

### Issue #19: End-to-End & Integration
- [ ] Full pipeline test (generate -> process -> view)
- [ ] Error handling scenarios
- [ ] Panic recovery tests
- [ ] Performance/load testing basics

## Phase 9: Documentation & Deployment (Priority: Medium)

### Issue #20: Documentation
- [ ] Update README with full setup
- [ ] API documentation (Swagger)
- [ ] Architecture diagrams
- [ ] Troubleshooting guide
- [ ] Contributing guidelines

### Issue #21: Final Polish
- [ ] Error logging endpoint (GET /api/v1/admin/logs/errors)
- [ ] Log rotation with lumberjack
- [ ] Security review (no secrets in code)
- [ ] All linters passing
- [ ] Production readiness check
