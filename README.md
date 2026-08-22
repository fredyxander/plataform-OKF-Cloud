# OKF Document Processing Platform

Multi-user web platform for asynchronous document processing and generation of bundles compatible with the **Open Knowledge Format (OKF)**.

The project is developed as part of the Cloud Architecture course and focuses on applying cloud-native architectural principles such as stateless services, asynchronous processing, persistent external storage, containerization, scalability, and fault isolation.

> **Current status:** Milestones 1–7 are completed and verified. The backend E2E flow, authentication/authorization, OKF bundle generation, idempotent redelivery handling, bounded retries, DLQ handling and partial-object cleanup are operational. Before starting the frontend, the remaining backend/rubric checklist is being completed.

---

## Architecture

The platform follows an asynchronous, service-oriented architecture.

```text
                    ┌─────────────────┐
                    │     React       │
                    │    Frontend     │
                    └────────┬────────┘
                             │ HTTP
                             ▼
                    ┌─────────────────┐
                    │     Go API      │
                    │    Stateless    │
                    └───┬────┬────┬───┘
                        │    │    │
                        │    │    └──────────► PostgreSQL
                        │    │                 Metadata
                        │    │
                        │    └───────────────► MinIO
                        │                      Object Storage
                        │
                        ▼
                    RabbitMQ
                        │
                        ▼
                   ┌─────────┐
                   │ Go      │
                   │ Worker  │
                   └────┬────┘
                        │
                   ┌────┴───────────┐
                   ▼                ▼
                MinIO          PostgreSQL
```

### Processing flow

The current backend processing flow is:

```text
User
  │
  │ Upload document
  ▼
React
  │
  │ HTTP Request
  ▼
Go API
  │
  ├── Store original document in MinIO
  ├── Create processing job in PostgreSQL
  └── Publish job to RabbitMQ
             │
             ▼
          RabbitMQ
             │
             ▼
          Go Worker
             │
             ├── Retrieve document
             ├── Convert document
             ├── Generate OKF bundle
             ├── Validate bundle
             ├── Store bundle in MinIO
             └── Update job status
```

The API does **not** perform document conversion directly. Processing is delegated to independent workers so HTTP requests do not remain blocked while documents are being processed.

---

## Technology Stack

| Component | Technology | Responsibility |
|---|---|---|
| Frontend | React + TypeScript + Vite | User interface |
| Frontend Server | Nginx | Serves the production React build |
| API | Go | HTTP API and application orchestration |
| Worker | Go | Asynchronous document processing |
| Database | PostgreSQL | Persistent application metadata |
| Message Broker | RabbitMQ | Asynchronous job delivery |
| Object Storage | MinIO | Original documents, assets and generated bundles |
| Containers | Docker | Service isolation and packaging |
| Orchestration | Docker Compose | Local multi-container deployment |

---

## Architectural Principles

The project is designed around the following principles:

### Stateless API

The API does not depend on local persistent files or in-memory application state.

Persistent information is stored externally in:

- PostgreSQL for metadata.
- MinIO for files and generated bundles.
- RabbitMQ for asynchronous job delivery.

This allows API containers to be restarted without interrupting jobs already being processed by workers.

### Asynchronous processing

Document conversion is performed outside the HTTP request lifecycle.

```text
API → RabbitMQ → Worker
```

The API creates and publishes a job and returns immediately to the client.

### Independent workers

Workers operate independently from the API.

Once a job has been published to RabbitMQ, a worker can continue processing it even if the API becomes temporarily unavailable.

### Persistent storage

Containers are considered disposable.

Docker volumes currently provide persistence for:

```text
postgres_data
rabbitmq_data
minio_data
```

Application containers can therefore be recreated without using their local filesystem as persistent storage.

### Separation of responsibilities

Each service has a specific responsibility:

```text
React       → User interface
API         → HTTP interface and orchestration
RabbitMQ    → Job delivery
Worker      → Document processing
PostgreSQL  → Metadata
MinIO       → Files
```

---

## Project Structure

```text
okf-project/
│
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   │   └── main.go
│   │   └── worker/
│   │       └── main.go
│   │
│   ├── internal/
│   │   ├── application/
│   │   ├── config/
│   │   ├── database/
│   │   ├── domain/
│   │   ├── http/
│   │   ├── queue/
│   │   └── storage/
│   │
│   ├── migrations/
│   ├── Dockerfile
│   ├── .dockerignore
│   ├── go.mod
│   └── go.sum
│
├── frontend/
│   ├── src/
│   ├── public/
│   ├── Dockerfile
│   ├── .dockerignore
│   ├── package.json
│   └── package-lock.json
│
├── docker-compose.yml
├── .env.example
├── .gitignore
└── README.md
```

The backend uses a single Go module with two independent entry points:

```text
cmd/api     → API executable
cmd/worker  → Worker executable
```

Shared application code is located under `internal/`.

---

## Requirements

To run the complete platform locally, only the following tools are required:

- Docker
- Docker Compose

For development outside containers:

- Go
- Node.js
- npm

---

## Environment Configuration

Create a `.env` file in the project root.

Example:

```env
POSTGRES_DB=okf
POSTGRES_USER=okf
POSTGRES_PASSWORD=change-me

RABBITMQ_USER=okf
RABBITMQ_PASSWORD=change-me

MINIO_ROOT_USER=okfadmin
MINIO_ROOT_PASSWORD=change-me
```

Do not commit `.env` files containing credentials.

An `.env.example` file is provided to document the required configuration.

---

## Running the Application

### Build and start

From the project root:

```bash
docker compose up -d --build
```

Check the status of all containers:

```bash
docker compose ps
```

The following services should be running:

```text
frontend
api
worker
postgres
rabbitmq
minio
```

---

## Service Access

### Frontend

```text
http://localhost:5173
```

### API health check

```text
http://localhost:8080/health
```

Expected response:

```json
{
  "status": "ok"
}
```

### RabbitMQ Management

```text
http://localhost:15672
```

Use the RabbitMQ credentials configured in `.env`.

### MinIO Console

```text
http://localhost:9001
```

Use the MinIO credentials configured in `.env`.

### PostgreSQL

Default exposed port:

```text
5432
```

PostgreSQL health can also be checked with:

```bash
docker compose exec postgres pg_isready
```

---

## Container Management

Stop the containers without removing them:

```bash
docker compose stop
```

Start previously stopped containers:

```bash
docker compose start
```

Remove containers and the Compose network:

```bash
docker compose down
```

Rebuild after source-code changes:

```bash
docker compose up -d --build
```

To completely reset the local environment, including persistent volumes:

```bash
docker compose down -v
```

> **Warning:** `docker compose down -v` deletes PostgreSQL, RabbitMQ and MinIO local volumes. Use it only when a complete local reset is intended.

---

## Logs

View logs from all services:

```bash
docker compose logs -f
```

API only:

```bash
docker compose logs -f api
```

Worker only:

```bash
docker compose logs -f worker
```

---

## OKF Bundle

A successful conversion will generate a self-contained bundle.

Minimum structure:

```text
bundle/
├── index.md
├── log.md
└── document.md
```

For structured documents:

```text
bundle/
├── index.md
├── log.md
├── section-01.md
├── section-02.md
├── section-03.md
└── assets/
```

`index.md` provides ordered navigation through the generated concepts.

`log.md` contains conversion and validation information.

A bundle must pass validation before it can be published for download. Before the frontend milestone, the validation result will be reviewed so it explicitly distinguishes `VALID`, `VALID_WITH_WARNINGS`, and `INVALID` when required by the project rubric.

---

## Current Scope and Remaining Work

### Completed core scope

- User registration, bcrypt password hashing, login and JWT authentication.
- Multi-user resource isolation by `owner_id`.
- Document upload to MinIO with persistent metadata in PostgreSQL.
- Automatic Job creation from `POST /documents` and immediate `202 Accepted` response with `jobId`.
- Asynchronous RabbitMQ processing with independent Go workers.
- Job status tracking and authorized bundle download.
- OKF conversion, segmentation, bundle generation and minimum bundle validation.
- Idempotent handling of duplicate/redelivered completed Jobs.
- Atomic Job claiming to avoid concurrent processing of the same queued Job.
- Recovery of stale `processing` Jobs.
- Deferred retry queue, bounded processing retries, DLQ and `FAILED` state with `error_message`.
- Compensation/cleanup of partially uploaded bundle objects in MinIO.

### Required backend/rubric checklist before frontend

1. Review bundle validation classification and explicitly support `VALID / VALID_WITH_WARNINGS / INVALID` if the current representation does not distinguish them.
2. Review bundle conversion with representative configurations: short document, structured multi-section document, ordered concepts/index links and reasonable edge cases.
3. Run and document the six verifiable conditions required by the project specification: effective asynchrony, short document, structured document, incomplete bundle rejection, multi-user isolation and duplicate-delivery idempotency.
4. Verify that a clean environment can be configured and started using only this README, `.env.example`, and `docker compose up -d --build`.
5. Demonstrate horizontal worker scaling with at least two workers while preserving `prefetch = 1`, atomic claiming and no duplicate final bundle.
6. Define and verify the completion-notification/status contract around `jobId` so the frontend can detect completion/failure and redirect to the bundle when appropriate. A normal Jobs list/view must remain available independently of notifications.

### Bonus / optional if time remains

- Additional input format.
- `assets/` extraction and references.
- Job cancellation.
- Metrics and additional observability.
- Separate OKF conformity score/reporting.
- Streaming downloads for large bundles.
- Additional real-time notification UX beyond the required status-following flow.

---

## Development Roadmap

### Milestone 1 — Infrastructure ✅

- React container.
- Go API container.
- Go worker container.
- PostgreSQL.
- RabbitMQ.
- MinIO.
- Docker Compose.
- API health endpoint.

### Milestone 2 — Asynchronous communication ✅

```text
POST /documents
       ↓
     Go API
       ↓
Create Document + Job
       ↓
    RabbitMQ
       ↓
    Go Worker
```

The HTTP request does not execute the conversion. The API returns the created `jobId` while the worker processes the Job independently.

### Milestone 3 — Persistence ✅

- Database migrations.
- Domain entities.
- Repository layer.
- Persistent users, documents, Jobs and bundles metadata.

### Milestone 4 — Authentication and authorization ✅

- User registration.
- Password hashing.
- Login.
- JWT authentication.
- Resource authorization by owner.

### Milestone 5 — Documents + MinIO ✅

- Upload.
- Original-document object storage.
- Persistent document metadata.
- Automatic Job creation and enqueue from `POST /documents`.

### Milestone 6 — OKF pipeline ✅

- Worker processing.
- Document segmentation.
- `index.md`, `log.md` and concept generation.
- Bundle validation before publication.
- ZIP packaging and bundle download.

### Milestone 7 — Processing reliability ✅

- Duplicate redelivery idempotency.
- Atomic Job claim.
- Stale-processing recovery.
- Deferred retries for contention.
- Bounded processing retries.
- `FAILED` + `error_message` + DLQ.
- Partial MinIO object cleanup.

### Pre-M8 backend and rubric closure ⏳

- `VALID / VALID_WITH_WARNINGS / INVALID` review.
- Advanced/representative conversion tests.
- Six specification verification scenarios.
- README reproducibility from a clean environment.
- Two-worker scalability demonstration.
- `jobId` completion/status notification contract while preserving a general Jobs view.

### Milestone 8 — Functional frontend ⏳

- Authentication and JWT handling.
- Document upload.
- Immediate `jobId` feedback.
- Jobs list/dashboard.
- Status tracking (`queued`, `processing`, `completed`, `failed`).
- Completion notification.
- Redirect/action from a completed Job to bundle download while preserving normal Jobs navigation.
- Authorized bundle download.

### Milestone 9 — Delivery and presentation ⏳

- Final clean-environment run.
- Rubric evidence and required failure/edge-case demonstrations.
- Architecture and Go-code walkthrough preparation.
- Maximum-20-minute presentation video.
- Known limitations and design decisions.

---

## Current Status

**Milestones 1–7 are completed and verified.**

The complete platform infrastructure can be built and started with:

```bash
docker compose up -d --build
```

The current backend flow is:

```text
POST /documents
      ↓
Document stored in MinIO
      ↓
Document + Job metadata in PostgreSQL
      ↓
RabbitMQ
      ↓
Go Worker
      ↓
OKF conversion + validation
      ↓
Bundle in MinIO + metadata in PostgreSQL
      ↓
COMPLETED / FAILED
```

The next task is **backend/rubric closure**, starting with the bundle validation classification (`VALID / VALID_WITH_WARNINGS / INVALID`). The frontend starts only after the remaining backend checklist has been verified.

---

## License

This project is developed for academic purposes.