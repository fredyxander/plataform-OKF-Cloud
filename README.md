# OKF Document Processing Platform

Multi-user web platform for asynchronous document processing and generation of bundles compatible with the **Open Knowledge Format (OKF)**.

The project is developed as part of the Cloud Architecture course and focuses on applying cloud-native architectural principles such as stateless services, asynchronous processing, persistent external storage, containerization, scalability, and fault isolation.

> **Current status:** Initial infrastructure and containerized services are operational. Business logic and document processing are under development.

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

The final processing flow will be:

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

A bundle must pass validation before it can be published for download.

---

## Planned Features

The project will progressively implement:

- User registration and authentication.
- Multi-user resource isolation.
- Document upload.
- Persistent document metadata.
- Asynchronous RabbitMQ jobs.
- Independent Go workers.
- Job status tracking.
- Document conversion.
- Multiple input formats.
- OKF bundle generation.
- Bundle validation.
- Assets support.
- Idempotent job processing.
- Retry policies.
- Job cancellation.
- Bundle download.
- Streaming downloads.
- Frontend job dashboard.
- Real-time completion notifications.
- Observability and structured logging.
- OKF conformity reporting.

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
POST /jobs
       ↓
     Go API
       ↓
    RabbitMQ
       ↓
    Go Worker
```

### Milestone 3 — Persistence ✅

- Database migrations.
- Domain entities.
- Repository layer.
- Job persistence.

### Milestone 4 — Authentication ✅

- User registration.
- Password hashing.
- Login.
- JWT authentication.
- Resource authorization.

### Milestone 5 — Document pipeline ✅

- Upload.
- MinIO storage.
- Job creation.
- Worker processing.
- OKF bundle generation.

### Milestone 6 — Validation and reliability

- Bundle validation.
- Idempotency.
- Retry policy.
- Failure handling.
- Cancellation.

### Milestone 7 — Frontend

- Authentication.
- Upload.
- Job dashboard.
- Status tracking.
- Notifications.
- Bundle download.

### Milestone 8 — Extended features

- Multiple document formats.
- Assets.
- Streaming.
- Observability.
- OKF conformity reporting.

---

## Current Status

**Milestone 1 completed.**

The complete infrastructure can currently be built and started using:

```bash
docker compose up -d --build
```

The next development milestone is the first end-to-end asynchronous communication:

```text
API → RabbitMQ → Worker
```

---

## License

This project is developed for academic purposes.