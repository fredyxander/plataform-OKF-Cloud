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

`OKF_FAULT_INJECTION` is optional and only used to demonstrate the incomplete
bundle case. Leave it empty for normal operation. See
[Bundle validation and result classification](#bundle-validation-and-result-classification).

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

`index.md` provides ordered navigation through the generated concepts, plus the
bundle data (source file, format and concept count).

`log.md` records the conversion traceability: source, operations applied,
detected units in order, and the validation result.

### Segmentation into logical units

Markdown is segmented by the **highest heading level that actually divides the
document**. H1 is tried first, then H2, then H3.

| Document | Segmentation |
| --- | --- |
| `# A` / `# B` / `# C` | 3 units, split by H1 |
| `# Title` + `## A` / `## B` | 3 units: the title block, then split by H2 |
| `## A` / `## B` (no H1) | 2 units, split by H2 |
| a single heading, or none | 1 unit, the whole document |

This avoids the common case where H1 is the document title and every section is
H2, which would otherwise yield a single unit.

Additional rules:

- headings inside fenced code blocks (``` or `~~~`) are **not** headings, so a
  `# comment` inside a shell snippet never splits the document;
- text before the first heading becomes its own unit, so no content is lost;
- each unit keeps its own heading, so every concept document is self-contained
  Markdown;
- Windows line endings are normalized before segmenting;
- section titles containing Markdown link syntax are sanitized for the index
  label, so a title like `# See [the source](https://x.org)` cannot break index
  link resolution;
- plain text is always kept as a single unit: it has no detectable structure.

Segmentation is deterministic: the same document always produces the same units
in the same order.

### Bundle validation and result classification

Every bundle is validated **before** any object is written to MinIO. The
validator does not stop at the first problem: it collects every finding in a
single pass and classifies the result in the three levels required by the
rubric.

| Result | Meaning | Published | Downloadable |
| --- | --- | --- | --- |
| `valid` | Minimum structure complete, index links resolve, no observations. | yes | yes |
| `valid_with_warnings` | Publishable, but the conversion left observations worth reporting. | yes | yes |
| `invalid` | The minimum structure or the index links are broken. | no | no |

Conditions that make a bundle **invalid**:

- `index.md` missing or empty;
- `log.md` missing;
- no concepts at all;
- a declared concept file missing from the bundle;
- a concept that `index.md` does not link;
- an `index.md` link that does not resolve to a file in the bundle;
- duplicated or unnamed files.

Conditions that only produce **warnings**:

- a concept with no content;
- an empty `log.md` (the conversion has no traceability);
- an `index.md` link without a title;
- a file present in the bundle that `index.md` does not reference.

A short document without divisions produces exactly one concept and is
classified as `valid` — a single unit never produces warnings.

The classification is persisted with the bundle metadata and exposed by
`GET /jobs/{id}`:

```json
{
  "status": "completed",
  "bundle": {
    "concept_count": 3,
    "is_valid": true,
    "validation": { "status": "valid", "warnings": [], "errors": [] },
    "download_url": "/jobs/{id}/bundle"
  }
}
```

A rejected bundle is still recorded in PostgreSQL as evidence of the
validation, but it has no `published_at`, no objects in MinIO and no
`download_url`. The Job ends as `failed` with the validation errors in
`error_message`, and `GET /jobs/{id}/bundle` answers `409 Conflict`.

#### Demonstrating an incomplete bundle

The pipeline always generates the minimum structure, so the "incomplete
bundle" condition of the rubric cannot occur on its own. A controlled fault
injection exists for the demo only, disabled unless the worker receives
`OKF_FAULT_INJECTION`:

| Value | Effect | Expected classification |
| --- | --- | --- |
| *(empty)* | normal pipeline | — |
| `drop-index` | removes `index.md` | `invalid` |
| `drop-log` | removes `log.md` | `invalid` |
| `empty-concept` | empties the first concept | `valid_with_warnings` |

```bash
OKF_FAULT_INJECTION=drop-index docker compose up -d worker
# upload a document, then check GET /jobs/{id} and GET /jobs/{id}/bundle
docker compose up -d worker   # back to the normal pipeline
```

---

## Manual verification with curl

End-to-end check of upload, processing and bundle download against the running
stack. Test documents live in `docs/filesTest/` — see the table there for the
expected result of each one.

Run these in **Git Bash**. In Windows PowerShell `curl` is an alias of
`Invoke-WebRequest`, so use `curl.exe` explicitly there.

Steps 2 to 5 are the normal flow. Steps 6 to 10 each demonstrate one of the
verifiable conditions the project specification requires.

### The six verifiable conditions

Every condition from section 6 of the specification, where to reproduce it and
what proves it:

| # | Condition | Section | What proves it |
| --- | --- | --- | --- |
| 1 | **Effective asynchrony.** The upload returns an id without waiting; the client may close the connection and the work goes on. | [§9](#9-effective-asynchrony) | The upload answers in under a second while the conversion takes twenty. The job reaches `completed` with the API stopped. |
| 2 | **Short document.** A short document with no divisions yields `index.md`, `log.md` and a single concept, with no failure and no warnings. | [§6](#6-run-every-test-document) with `01-breve.txt` | `concept_count: 1`, `validation.status: valid`, empty `warnings`. |
| 3 | **Structured document.** A document with several sections yields one concept per unit, linked in order from `index.md`. | [§3](#3-upload-a-document)–[§5](#5-download-and-inspect-the-bundle) with `03-manual-tecnico.md` | `concept_count: 4` and `index.md` linking `concept-01..04.md` in the source order. |
| 4 | **Incomplete bundle.** With `index.md` or `log.md` missing, validation fails, the bundle is not published and no download is offered. | [§7](#7-rejected-bundle) | Job `failed`, `validation.status: invalid`, no `download_url`, download answers `409`, nothing written to MinIO. |
| 5 | **Isolation.** A user who knows another user's resource id is denied without leaking information. | [§8](#8-owner-isolation) | `404` on both the job detail and the bundle download, `401` with no token. |
| 6 | **No duplicates.** If the queue delivers the same job twice, there is a single final effect and at most one published bundle. | [§10](#10-no-duplicate-effect-on-redelivery) | Identical job and bundle rows before and after the redelivery, and `count = 1` in `bundles`. |

Conditions 1 and 6 need the worker in a non-default mode; each section sets up
what it needs and says how to restore the normal pipeline.

### 1. Start the stack

```bash
docker compose up -d --build
curl -s http://localhost:8080/health
```

### 2. Register and get a token

```bash
EMAIL="demo-1@example.com"

curl -s -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"password123\"}"

RESPONSE=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"password123\"}")

echo "$RESPONSE"

TOKEN=$(echo "$RESPONSE" | sed -E 's/.*"token":"([^"]+)".*/\1/')

# Also keep it on disk: shell variables are lost when the terminal
# changes, and every later step needs the token.
echo "$TOKEN" > .token

echo "${#TOKEN} characters"   # a JWT, ~250 characters
```

`sed` returns its input unchanged when it finds no match, so a failed login
leaves the error JSON inside `TOKEN` instead of failing. If the character count
is not around 250, read `$RESPONSE` before continuing.

Every following step uses `$TOKEN`. If a request answers `unauthorized`, check
`echo "${#TOKEN}"` first: `0` means the variable is not set in **this** shell.
Either re-run this step, or read the token from disk instead:

```bash
curl -s ... -H "Authorization: Bearer $(cat .token)"
```

### 3. Upload a document

The `type=` part is required: without it curl sends
`application/octet-stream` and the API answers `415 Unsupported Media Type`.
Only `text/plain` and `text/markdown` are accepted, up to 10 MB.

```bash
RESPONSE=$(curl -s -X POST http://localhost:8080/documents \
  -H "Authorization: Bearer $(cat .token)" \
  -F "file=@docs/filesTest/03-manual-tecnico.md;type=text/markdown")

echo "$RESPONSE"

# Keep the jobId on disk so the next steps work from any shell.
echo "$RESPONSE" | sed -E 's/.*"jobId":"([^"]+)".*/\1/' > .job
echo "jobId = $(cat .job)"
```

The response is immediate — the conversion has not run yet:

```json
{ "document": { "...": "..." }, "jobId": "<uuid>", "status": "queued" }
```

### 4. Follow the job

```bash
curl -s http://localhost:8080/jobs/$(cat .job) -H "Authorization: Bearer $(cat .token)"
```

Expected for `03-manual-tecnico.md`: `"status": "completed"`,
`"concept_count": 4`, `"validation": {"status": "valid", ...}` and a
`download_url`.

The full list is at `GET /jobs`:

```bash
curl -s http://localhost:8080/jobs -H "Authorization: Bearer $(cat .token)"
```

### 5. Download and inspect the bundle

```bash
curl -s -o bundle.zip http://localhost:8080/jobs/$(cat .job)/bundle \
  -H "Authorization: Bearer $(cat .token)"

unzip -o bundle.zip -d bundle/
cat bundle/index.md
cat bundle/log.md
cat bundle/concept-02.md
```

Check that `index.md` links every concept in the original order, that `log.md`
lists the operations, the detected units and the validation result, and that
the fenced code block inside `concept-02.md` arrived intact.

### 6. Run every test document

This covers the **short document** and **structured document** conditions in one
go: it uploads all six test documents, keeps each `jobId` next to its filename,
and then prints the result of each one.

```bash
docker compose up -d --scale worker=1 worker   # one worker, normal pipeline
sleep 5

rm -f .jobs
for f in docs/filesTest/0*; do
  case "$f" in *.txt) CT=text/plain;; *) CT=text/markdown;; esac
  JOB=$(curl -s -X POST http://localhost:8080/documents \
    -H "Authorization: Bearer $(cat .token)" \
    -F "file=@$f;type=$CT" \
    | sed -E 's/.*"jobId":"([^"]+)".*/\1/')
  echo "$JOB $(basename "$f")" >> .jobs
done

sleep 5

while read -r job name; do
  printf '%-24s ' "$name"
  curl -s http://localhost:8080/jobs/"$job" -H "Authorization: Bearer $(cat .token)" \
    | grep -o '"status":"[^"]*"\|"concept_count":[0-9]*' | tr '\n' ' '
  echo
done < .jobs
```

The three values per line are the job status, the concept count and the
validation status:

```text
01-breve.txt             "status":"completed" "concept_count":1 "status":"valid"
02-estructurado.md       "status":"completed" "concept_count":3 "status":"valid"
03-manual-tecnico.md     "status":"completed" "concept_count":4 "status":"valid"
04-preambulo.md          "status":"completed" "concept_count":3 "status":"valid"
05-titulo-con-enlace.md  "status":"completed" "concept_count":2 "status":"valid"
06-seccion-vacia.md      "status":"completed" "concept_count":3 "status":"valid_with_warnings"
```

`01-breve.txt` is the short-document condition: one concept, `valid`, no
warnings. `06-seccion-vacia.md` is the only one that may report
`valid_with_warnings`, and it is still downloadable. See
`docs/filesTest/README.md` for what each document exercises.

Run this with the worker in its normal mode — hence the first line. With
`OKF_PROCESSING_DELAY` still enabled the six documents are processed one after
another and the batch takes six times the delay.

### 7. Rejected bundle

```bash
OKF_FAULT_INJECTION=drop-index docker compose up -d worker

curl -s -X POST http://localhost:8080/documents \
  -H "Authorization: Bearer $(cat .token)" \
  -F "file=@docs/filesTest/02-estructurado.md;type=text/markdown" \
  | sed -E 's/.*"jobId":"([^"]+)".*/\1/' > .job

sleep 3

curl -s http://localhost:8080/jobs/$(cat .job) -H "Authorization: Bearer $(cat .token)"
echo
curl -s -o /dev/null -w "download: %{http_code}\n" \
  http://localhost:8080/jobs/$(cat .job)/bundle -H "Authorization: Bearer $(cat .token)"

docker compose up -d worker   # back to the normal pipeline
```

Expected: the Job ends `failed` with
`bundle validation failed: bundle is missing index.md`, the bundle reports
`"status": "invalid"` with no `download_url`, and the download answers `409`.
Nothing is written to MinIO for that job.

### 8. Owner isolation

Register a second user, then ask for the first user's job with the second
user's token:

```bash
EMAIL="demo-2@example.com"
curl -s -X POST http://localhost:8080/auth/register   -H "Content-Type: application/json"   -d "{\"email\":\"$EMAIL\",\"password\":\"password123\"}"

curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"password123\"}" \
  | sed -E 's/.*"token":"([^"]+)".*/\1/' > .token-other

echo "$(wc -c < .token-other) characters"   # a JWT, ~250 characters
```

`.job` still holds a job that belongs to `demo-1`. Ask for it with the second
user's token:

```bash
curl -s -o /dev/null -w "job detail:     %{http_code}\n" \
  http://localhost:8080/jobs/$(cat .job) -H "Authorization: Bearer $(cat .token-other)"

curl -s -o /dev/null -w "bundle download: %{http_code}\n" \
  http://localhost:8080/jobs/$(cat .job)/bundle -H "Authorization: Bearer $(cat .token-other)"

curl -s -o /dev/null -w "no token at all: %{http_code}\n" \
  http://localhost:8080/jobs/$(cat .job)
```

Expected: `404`, `404` and `401`. The foreign resource answers `404`, not
`403` — the server does not reveal that it exists.

### 9. Effective asynchrony

The real pipeline finishes in milliseconds, which leaves no window to observe
that the upload did not wait for the conversion. `OKF_PROCESSING_DELAY` adds an
artificial pause to the worker so the `processing` state is visible. It is off
by default and must stay below the 5 minute stale-job lease.

```bash
OKF_PROCESSING_DELAY=20s docker compose up -d worker
docker compose logs worker --tail 5   # confirms the delay is enabled
```

**The upload returns immediately and the job progresses on its own.** Paste the
whole block at once: there is no time to copy the `jobId` by hand between the
upload and the first status check.

`--max-time 2` makes curl give up after two seconds, far less than the twenty
the conversion now takes — yet the upload still answers.

```bash
printf '%s  upload sent\n' "$(date +%T)"

RESPONSE=$(curl -s --max-time 2 -X POST http://localhost:8080/documents \
  -H "Authorization: Bearer $(cat .token)" \
  -F "file=@docs/filesTest/03-manual-tecnico.md;type=text/markdown")

printf '%s  answered: %s\n' "$(date +%T)" "$RESPONSE"

echo "$RESPONSE" | sed -E 's/.*"jobId":"([^"]+)".*/\1/' > .job

for i in $(seq 1 10); do
  printf "%s  " "$(date +%T)"
  curl -s http://localhost:8080/jobs/$(cat .job) \
    -H "Authorization: Bearer $(cat .token)" \
    | grep -o '"status":"[^"]*"' | head -1
  sleep 3
done
```

`grep -o ... | head -1` prints only the job's own status: the response also
carries the bundle validation status, and a greedy match would return that one
instead.

Expected: the upload answers in under a second, then `"processing"` for about
twenty seconds, then `"completed"` — without the client doing anything.

**The work survives the API.** Upload another document and stop the API while
the worker is still busy:

```bash
curl -s -X POST http://localhost:8080/documents \
  -H "Authorization: Bearer $(cat .token)" \
  -F "file=@docs/filesTest/02-estructurado.md;type=text/markdown" \
  | sed -E 's/.*"jobId":"([^"]+)".*/\1/' > .job

docker compose stop api
curl -s -o /dev/null -w "%{http_code}\n" --max-time 3 http://localhost:8080/health
docker compose logs -f worker
```

The API answers `000` (unreachable) while the worker log still reaches
`job completed: <jobId>`. Bring the API back and the result is there:

```bash
docker compose start api
sleep 4
curl -s http://localhost:8080/jobs/$(cat .job) -H "Authorization: Bearer $(cat .token)"
echo
curl -s -o /dev/null -w "download: %{http_code}\n" \
  http://localhost:8080/jobs/$(cat .job)/bundle -H "Authorization: Bearer $(cat .token)"
```

Expected: `"status": "completed"` with its bundle, and `200` on the download.

Return the worker to the normal pipeline when you are done:

```bash
docker compose up -d worker
```

### 10. No duplicate effect on redelivery

Message queues guarantee *at-least-once* delivery, so the same job can arrive
twice. The same job message can be pushed back into the queue through the
RabbitMQ management API — no application code is involved, the queue really
does deliver the job a second time:

```bash
curl -s -u okf:okf -X POST \
  http://localhost:15672/api/exchanges/%2F/amq.default/publish \
  -H "Content-Type: application/json" \
  -d "{\"properties\":{},\"routing_key\":\"document_jobs\",\"payload\":\"{\\\"jobId\\\":\\\"$(cat .job)\\\"}\",\"payload_encoding\":\"string\"}"
```

`{"routed":true}` confirms the queue accepted it.

**Redelivery of a finished job.** Paste the whole block: it forces a single
worker with no delay, uploads a document, prints the state once the job is
done, makes the queue deliver that same job again, and prints the state a
second time.

The first line matters — the other scenarios in this README leave two workers
or an artificial delay behind, and either would change what you see here.

```bash
docker compose up -d --scale worker=1 worker   # one worker, normal pipeline
sleep 5

curl -s -X POST http://localhost:8080/documents \
  -H "Authorization: Bearer $(cat .token)" \
  -F "file=@docs/filesTest/02-estructurado.md;type=text/markdown" \
  | sed -E 's/.*"jobId":"([^"]+)".*/\1/' > .job
sleep 3

echo "=== before the duplicate ==="
docker compose exec -T postgres psql -U okf -d okf -c \
  "SELECT j.status, j.updated_at, b.id AS bundle, b.created_at
     FROM jobs j LEFT JOIN bundles b ON b.job_id = j.id
    WHERE j.id = '$(cat .job)';"

echo "=== redelivering the same job ==="
curl -s -u okf:okf -X POST \
  http://localhost:15672/api/exchanges/%2F/amq.default/publish \
  -H "Content-Type: application/json" \
  -d "{\"properties\":{},\"routing_key\":\"document_jobs\",\"payload\":\"{\\\"jobId\\\":\\\"$(cat .job)\\\"}\",\"payload_encoding\":\"string\"}"
echo
sleep 3

echo "=== what the worker did with it ==="
docker compose logs worker --since 30s | grep "$(cat .job)" | tail -3

echo "=== after the duplicate ==="
docker compose exec -T postgres psql -U okf -d okf -c \
  "SELECT j.status, j.updated_at, b.id AS bundle, b.created_at
     FROM jobs j LEFT JOIN bundles b ON b.job_id = j.id
    WHERE j.id = '$(cat .job)';"
```

Expected in the log:

```text
job <id> already completed; acknowledging duplicate delivery
```

And the two `psql` outputs must be identical: same `updated_at`, same bundle
id, same `created_at`. The pipeline never ran a second time.

**Redelivery while the job is still running.** A single worker with
`prefetch = 1` cannot receive a second message while one is unacknowledged, so
this case needs two workers. The block takes about a minute: the deferred
duplicate waits thirty seconds in the retry queue before coming back.

```bash
OKF_PROCESSING_DELAY=20s docker compose up -d --scale worker=2 worker
sleep 5

curl -s -X POST http://localhost:8080/documents \
  -H "Authorization: Bearer $(cat .token)" \
  -F "file=@docs/filesTest/02-estructurado.md;type=text/markdown" \
  | sed -E 's/.*"jobId":"([^"]+)".*/\1/' > .job

# Redeliver at once, while the first worker is still in its 20s conversion.
sleep 1
curl -s -u okf:okf -X POST \
  http://localhost:15672/api/exchanges/%2F/amq.default/publish \
  -H "Content-Type: application/json" \
  -d "{\"properties\":{},\"routing_key\":\"document_jobs\",\"payload\":\"{\\\"jobId\\\":\\\"$(cat .job)\\\"}\",\"payload_encoding\":\"string\"}"
echo

sleep 55
echo "=== worker-1 ==="
docker logs okf-project-worker-1 --since 90s 2>&1 | grep "$(cat .job)"
echo "=== worker-2 ==="
docker logs okf-project-worker-2 --since 90s 2>&1 | grep "$(cat .job)"
```

Two different defences fire, in order:

```text
worker A  status=queued     -> claimed -> simulating a long conversion for 20s
worker B  status=processing -> temporarily not claimable; deferring delivery
worker A  job completed
worker A  status=completed  -> already completed; acknowledging duplicate delivery
```

Which container plays A and which plays B varies: RabbitMQ hands the two
messages to its consumers in round robin, so the roles swap between runs. What
must not vary is the sequence.

The atomic `queued -> processing` claim stops the concurrent duplicate, which
is deferred to `document_jobs_retry` and comes back thirty seconds later; by
then the job is finished and idempotency acknowledges it without reprocessing.

Confirm a single published bundle:

```bash
docker compose exec -T postgres psql -U okf -d okf -c \
  "SELECT count(*) FROM bundles WHERE job_id = '$(cat .job)';"

docker compose up -d --scale worker=1 worker   # back to one worker, no delay
```

Expected: `1`. `bundles.job_id` also carries a `UNIQUE` constraint, so a second
publication is impossible even if every check above were bypassed.

---

## Current Scope and Remaining Work

### Completed core scope

- User registration, bcrypt password hashing, login and JWT authentication.
- Multi-user resource isolation by `owner_id`.
- Document upload to MinIO with persistent metadata in PostgreSQL.
- Automatic Job creation from `POST /documents` and immediate `202 Accepted` response with `jobId`.
- Asynchronous RabbitMQ processing with independent Go workers.
- Job status tracking and authorized bundle download.
- OKF conversion, segmentation, bundle generation and bundle validation classified as `valid` / `valid_with_warnings` / `invalid`.
- Idempotent handling of duplicate/redelivered completed Jobs.
- Atomic Job claiming to avoid concurrent processing of the same queued Job.
- Recovery of stale `processing` Jobs.
- Deferred retry queue, bounded processing retries, DLQ and `FAILED` state with `error_message`.
- Compensation/cleanup of partially uploaded bundle objects in MinIO.

### Required backend/rubric checklist before frontend

1. ~~Review bundle validation classification and explicitly support `VALID / VALID_WITH_WARNINGS / INVALID`.~~ **Done** — see [Bundle validation and result classification](#bundle-validation-and-result-classification).
2. ~~Review bundle conversion with representative configurations.~~ **Done** — see [Segmentation into logical units](#segmentation-into-logical-units).
3. ~~Run and document the six verifiable conditions required by the project specification.~~ **Done** — see [The six verifiable conditions](#the-six-verifiable-conditions).
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

- `VALID / VALID_WITH_WARNINGS / INVALID` classification ✅.
- Representative conversion configurations ✅.
- Six specification verification scenarios ✅.
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

**Backend/rubric closure is in progress**, three of six items done: the
validation classification, the conversion review against representative
document configurations, and the six verifiable conditions, each with a
reproducible procedure in [Manual verification with curl](#manual-verification-with-curl).

Next: reproducibility from a clean environment, a two-worker scalability
demonstration, and the `jobId` notification contract. The frontend starts only
after the remaining backend checklist has been verified.

---

## License

This project is developed for academic purposes.