# Estado del proyecto --- Plataforma OKF Cloud

**Fecha de corte:** 22 de agosto de 2026\
**Repositorio:** `fredyxander/plataform-OKF-Cloud`\
**Propósito de este documento:** servir como checkpoint técnico para
continuar el desarrollo en una nueva sesión sin perder decisiones,
avances, pruebas realizadas ni pendientes relevantes.

------------------------------------------------------------------------

## 1. Objetivo general

Construir una plataforma web multiusuario para cargar documentos,
procesarlos de forma asíncrona y generar bundles compatibles con Open
Knowledge Format (OKF).

La arquitectura busca aplicar criterios cloud-native incluso en
ejecución local:

-   API stateless.
-   Procesamiento asíncrono mediante cola.
-   Workers independientes.
-   Persistencia relacional externa.
-   Almacenamiento de objetos externo.
-   Contenedores desechables.
-   Separación de responsabilidades.
-   Posibilidad de escalar workers horizontalmente.
-   Recuperación ante fallos y procesamiento confiable.

------------------------------------------------------------------------

## 2. Arquitectura definida

``` text
                         ┌──────────────────┐
                         │ React + Nginx    │
                         │ Frontend         │
                         └────────┬─────────┘
                                  │ HTTP
                                  ▼
                         ┌──────────────────┐
                         │ Go API           │
                         │ Stateless        │
                         └───┬────────┬─────┘
                             │        │
                 metadata    │        │ jobs
                             ▼        ▼
                       PostgreSQL   RabbitMQ
                                      │
                                      │ entrega trabajos
                                      ▼
                                  Go Worker
                                  ┌───┴────┐
                                  │        │
                                  ▼        ▼
                            PostgreSQL    MinIO
```

MinIO se utilizará para documentos originales, assets y bundles
generados. PostgreSQL será la fuente de verdad para usuarios,
documentos, jobs y bundles. RabbitMQ coordina el trabajo asíncrono entre
API y workers.

------------------------------------------------------------------------

## 3. Stack tecnológico actual

-   **Frontend:** React + TypeScript + Vite.
-   **Servidor frontend:** Nginx.
-   **Backend:** Go.
-   **API:** Go HTTP.
-   **Workers:** Go.
-   **Base de datos:** PostgreSQL.
-   **Broker de mensajes:** RabbitMQ.
-   **Object storage:** MinIO.
-   **Contenedores:** Docker.
-   **Orquestación local:** Docker Compose.
-   **Driver PostgreSQL en Go:** `github.com/lib/pq`.
-   **Cliente RabbitMQ:** `github.com/rabbitmq/amqp091-go`.
-   **UUID:** `github.com/google/uuid`.
-   **Password hashing:** `golang.org/x/crypto/bcrypt`.
-   **JWT:** `github.com/golang-jwt/jwt/v5`.

------------------------------------------------------------------------

## 4. Estructura relevante del repositorio

``` text
okf-project/
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   │   └── main.go
│   │   └── worker/
│   │       └── main.go
│   ├── internal/
│   │   ├── application/
│   │   │   ├── auth_service.go
│   │   │   └── document_service.go
│   │   ├── auth/
│   │   │   ├── password.go
│   │   │   └── token.go
│   │   ├── config/
│   │   ├── database/
│   │   │   ├── db.go
│   │   │   ├── migrate.go
│   │   │   ├── user_repo.go
│   │   │   ├── document_repo.go
│   │   │   ├── job_repo.go
│   │   │   └── bundle_repo.go
│   │   ├── domain/
│   │   ├── http/
│   │   │   ├── auth_handler.go
│   │   │   ├── auth_middleware.go
│   │   │   └── document_handler.go
│   │   ├── queue/
│   │   └── storage/
│   ├── migrations/
│   │   ├── 001_init.sql
│   │   └── embed.go
│   ├── Dockerfile
│   ├── .dockerignore
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   ├── Dockerfile
│   ├── .dockerignore
│   ├── package.json
│   └── package-lock.json
├── docker-compose.yml
├── .env
├── .env.example
├── .gitignore
└── README.md
```

------------------------------------------------------------------------

# 5. Milestone 1 --- Infraestructura

**Estado: COMPLETADO Y VERIFICADO.**

Se creó la infraestructura Docker Compose con los servicios:

-   frontend;
-   api;
-   worker;
-   postgres;
-   rabbitmq;
-   minio.

El frontend se compila con Node/Vite en un build multistage y el
resultado `dist/` es servido por Nginx. `node_modules` del host no entra
al contenedor Linux.

Se solucionaron problemas de dependencias nativas de Rolldown entre
Windows y Linux. La regla adoptada es no declarar manualmente bindings
específicos de Windows y excluir `node_modules` mediante
`.dockerignore`.

También se alineó la versión de Go requerida por `go.mod` con la imagen
utilizada en Docker.

### Persistencia Docker

Se utilizan volúmenes para conservar datos locales:

``` text
postgres_data
rabbitmq_data
minio_data
```

`docker compose down` elimina contenedores y red, pero conserva los
volúmenes.\
`docker compose down -v` elimina también esos volúmenes y reinicia el
estado persistente local.

Los volúmenes no viajan por Git ni entre computadores. La
reproducibilidad de PostgreSQL se obtiene mediante migraciones, no
copiando el volumen.

### Verificaciones realizadas

Se comprobó:

-   frontend accesible;
-   API `/health`;
-   worker activo;
-   PostgreSQL disponible;
-   RabbitMQ Management disponible;
-   MinIO Console disponible;
-   independencia de contenedores.

------------------------------------------------------------------------

# 6. Milestone 2 --- Mensajería asíncrona API → RabbitMQ → Worker

**Estado: COMPLETADO Y VERIFICADO.**

Se implementó el primer flujo asíncrono funcional:

``` text
POST /jobs  (antes `/jobs/test`; reemplazado en M4)
      │
      ▼
    Go API
      │
      │ JobMessage JSON
      ▼
   RabbitMQ
 document_jobs
      │
      ▼
   Go Worker
      │
      ▼
 procesamiento
      │
      ▼
     ACK
```

## 6.1 Contrato de mensaje

Se creó un `JobMessage` con `JobID`, serializado como JSON.

Ejemplo conceptual:

``` json
{
  "jobId": "ecbb7858-7cbd-45f5-8243-4e6a7a913cc4"
}
```

La API serializa el struct con `json.Marshal`; el worker lo reconstruye
mediante `json.Unmarshal`.

## 6.2 Producer

La API funciona como producer:

1.  originalmente recibía `POST /jobs/test`; desde el Milestone 4 el
    endpoint definitivo es `POST /jobs`;
2.  originalmente generaba un UUID aislado (comportamiento del Milestone
    2);
3.  construye `JobMessage`;
4.  publica el mensaje en `document_jobs`;
5.  responde `202 Accepted`.

Desde el Milestone 3, el UUID publicado corresponde a un Job persistido
previamente en PostgreSQL con estado `queued`.

La API no realiza el procesamiento.

## 6.3 Consumer

El worker se registra como consumer de `document_jobs`.

No realiza polling manual. RabbitMQ entrega los mensajes a los consumers
registrados.

El worker utiliza un Go channel retornado por el cliente AMQP y espera
mensajes mediante:

``` go
for message := range messages {
    // procesar
}
```

## 6.4 ACK / NACK

El consumer utiliza acknowledgements manuales.

Flujo correcto:

``` text
recibir
  ↓
validar
  ↓
procesar
  ↓
terminar
  ↓
ACK
```

El ACK no se envía antes de terminar.

Los mensajes con JSON inválido o sin `jobId` reciben NACK y actualmente
no se reencolan.

## 6.5 Prueba de procesamiento largo

Se utilizó temporalmente `time.Sleep(10 * time.Second)` para simular una
conversión documental y comprobar visualmente que el worker continúa
trabajando aunque la API se detenga.

Debe retirarse cuando se implemente procesamiento real.

## 6.6 Pruebas realizadas

### Worker detenido

Se detuvo el worker y se enviaron varios jobs.

Resultado: los mensajes permanecieron en RabbitMQ como `Ready`.

### Worker reiniciado

Al volver a iniciar el worker, RabbitMQ entregó los trabajos pendientes
y fueron procesados.

### API detenida

Se publicó un trabajo y se detuvo la API durante el procesamiento.

Resultado: el worker continuó y terminó el job.

Esto verifica el desacoplamiento:

``` text
API ✗
RabbitMQ ✓
Worker ✓
```

## 6.7 Healthcheck de RabbitMQ

Se agregó un healthcheck con `rabbitmq-diagnostics -q ping` para que
Docker pueda diferenciar entre un contenedor simplemente iniciado y un
broker saludable.

## 6.8 Retry de conexión

Se implementó `NewRabbitMQWithRetry(...)`.

Configuración usada durante las pruebas:

-   hasta 10 intentos;
-   aproximadamente 3 segundos entre intentos.

Se verificó realmente:

``` text
attempt 1 → connection refused
attempt 2 → connected
```

tanto en API como en worker.

El retry implementado en esta etapa protege el arranque de API y worker, que es el comportamiento requerido para el alcance actual.

------------------------------------------------------------------------

# 7. Capa de PostgreSQL incorporada por PR #1

**Estado: IMPLEMENTADA, INTEGRADA Y VERIFICADA END-TO-END EN EL
MILESTONE 3.**

El PR #1, `feat: add database migrations and repository layer`, fue
mergeado a `main` el 20 de agosto de 2026.

Por esta razón, el antiguo Milestone 3 ya no debe considerarse "por
implementar desde cero". Existe una base considerable de persistencia
que debe revisarse, probarse e integrarse con API y worker.

## 7.1 Conexión PostgreSQL

Se agregó `internal/database/db.go`.

`database.New(dsn)`:

1.  abre el pool mediante `database/sql`;
2.  usa el driver PostgreSQL `lib/pq`;
3.  ejecuta `Ping()` para comprobar conectividad;
4.  devuelve un `DB` que encapsula `*sql.DB`.

La API obtiene `DATABASE_URL` desde variables de entorno, abre
PostgreSQL y cierra la conexión mediante `defer`.

## 7.2 Migraciones

Se agregó un sistema propio de migraciones:

``` text
backend/migrations/
├── 001_init.sql
└── embed.go
```

Las migraciones SQL se embeben en el binario Go y `db.Migrate()` aplica
las pendientes en orden.

Existe una tabla:

``` text
schema_migrations
```

que registra qué archivos ya fueron aplicados.

Cada migración se ejecuta dentro de una transacción y solo se registra
como aplicada si el SQL finaliza correctamente.

Además se utiliza un PostgreSQL advisory lock (`pg_advisory_lock`) para
serializar migraciones. Esto es relevante si varias instancias de API
arrancan simultáneamente: evita que intenten migrar el esquema al mismo
tiempo.

## 7.3 Repository layer

Ya existen repositorios para:

### Usuarios

`user_repo.go`

Debe considerarse parte de la base para el futuro milestone de
autenticación y aislamiento multiusuario.

### Documentos

`document_repo.go`

Incluye al menos operaciones para:

-   crear documento;
-   obtener documento por ID y owner.

El modelo almacena metadata como:

-   owner;
-   filename;
-   storage key;
-   formato;
-   tamaño.

El archivo real seguirá correspondiendo a MinIO; PostgreSQL conserva
metadata/referencias.

### Jobs

`job_repo.go`

Incluye:

-   `CreateJob`;
-   `GetJobByID`;
-   `UpdateJobStatus`;
-   `ListJobsByOwner`.

Los jobs ya contemplan:

-   document ID;
-   owner ID;
-   status;
-   idempotency key;
-   error message;
-   timestamps.

Esto adelanta parte importante de los requisitos de seguimiento e
idempotencia.

### Bundles

`bundle_repo.go`

Incluye:

-   creación de bundle;
-   búsqueda de bundle por job y owner.

Se persiste metadata como:

-   job;
-   owner;
-   storage key/prefix de MinIO;
-   validez;
-   concept count;
-   fecha de publicación;
-   fecha de creación.

## 7.4 Integración actual en API

El arranque de API ahora incluye conceptualmente:

``` text
config
  ↓
RabbitMQ connection
  ↓
DATABASE_URL
  ↓
database.New()
  ↓
db.Ping()
  ↓
db.Migrate()
  ↓
HTTP server
```

Por tanto, la API ya verifica conexión PostgreSQL y actualiza el esquema
durante el arranque.

------------------------------------------------------------------------

# 8. Revisión de milestones después del PR de base de datos

La planificación debe actualizarse.

## Milestone 1 --- Infraestructura

**COMPLETADO.**

Docker, frontend, API, worker, PostgreSQL, RabbitMQ y MinIO operativos.

## Milestone 2 --- Mensajería asíncrona

**COMPLETADO.**

Producer/consumer, cola, ACK/NACK, pruebas de desacoplamiento,
healthcheck y startup retry.

## Milestone 3 --- Persistencia PostgreSQL

**COMPLETADO Y VERIFICADO.**

Se verificó e integró la capa PostgreSQL incorporada por el PR #1 con el
flujo asíncrono del Milestone 2.

Resultados principales:

-   migración `001_init.sql` aplicada correctamente desde una base
    limpia;
-   `schema_migrations` registra la migración y una segunda ejecución no
    la reaplica;
-   tablas `users`, `documents`, `jobs`, `bundles` y `schema_migrations`
    verificadas;
-   modelos Go revisados contra el esquema SQL;
-   repositories de usuarios, documentos, jobs y bundles probados contra
    PostgreSQL real;
-   aislamiento por `owner_id` comprobado en las consultas
    correspondientes;
-   API crea primero un Job persistido con estado `queued` y publica ese
    mismo `JobID` en RabbitMQ;
-   worker conectado directamente a PostgreSQL, sin depender de la API;
-   ciclo `queued → processing → completed` verificado en PostgreSQL;
-   transición `processing → failed` y persistencia de `error_message`
    verificadas mediante un fallo controlado temporal;
-   trigger `updated_at` verificado durante las transiciones;
-   RabbitMQ QoS configurado con `prefetch = 1` y comprobado con dos
    jobs consecutivos;
-   suite `go test ./...` ejecutada satisfactoriamente con PostgreSQL
    disponible.

Flujo actualmente verificado:

``` text
POST /jobs
   │
   ▼
API
   │
   ├── PostgreSQL: CreateJob → QUEUED
   │
   └── RabbitMQ: publish JobID
                     │
                     ▼
                   Worker
                     │
                     ├── PostgreSQL → PROCESSING
                     │
                     ├── procesamiento temporal
                     │
                     └── PostgreSQL → COMPLETED
                                   ↓
                                  ACK
```

El camino `FAILED + error_message` también fue probado, pero el error
simulado usado para verificarlo fue retirado después de la prueba.

**Actualización posterior (Milestone 4):** `/jobs/test` fue eliminado y
reemplazado por `POST /jobs`. El `ownerId` ya no se recibe desde el
cliente: se obtiene de la identidad autenticada mediante JWT.
`documentId` continúa siendo parte legítima del request para indicar qué
documento propio debe procesarse. La API valida `documentId + owner_id`
antes de crear el Job.

**Elemento todavía temporal:** `time.Sleep(10 * time.Second)` continúa
como simulación del procesamiento real y debe retirarse al implementar
el pipeline OKF.

## Milestone 4 --- Autenticación y autorización

**COMPLETADO Y VERIFICADO.**

Se implementó autenticación stateless mediante JWT y autorización por
propietario para los recursos funcionales actuales: documentos y jobs.

Resultados principales:

-   registro de usuarios mediante `POST /auth/register`;
-   contraseñas almacenadas exclusivamente como hashes bcrypt;
-   email normalizado y restricción de email único;
-   emails duplicados traducidos a `409 Conflict`;
-   login mediante `POST /auth/login`;
-   credenciales inválidas retornan `401 Unauthorized` sin distinguir
    entre email inexistente y contraseña incorrecta;
-   generación de JWT firmado con HS256;
-   claims con `user_id`, `sub`, `iat` y `exp`;
-   validación explícita del algoritmo HS256, firma y expiración;
-   `JWT_SECRET` suministrado mediante variable de entorno;
-   `AuthMiddleware` implementado para procesar
    `Authorization: Bearer <token>`;
-   identidad autenticada propagada mediante `context.Context`;
-   `UserIDFromContext(...)` utilizado por handlers protegidos;
-   `X-Test-Owner-ID` eliminado completamente;
-   `ownerId` controlado por el cliente eliminado del flujo de jobs;
-   `/jobs/test` eliminado y reemplazado por rutas REST definitivas;
-   autorización por `owner_id` verificada tanto para documentos como
    jobs;
-   acceso a recursos ajenos retorna `404`, evitando revelar su
    existencia;
-   requests a recursos protegidos sin JWT retornan `401`;
-   suite `go test ./...` ejecutada satisfactoriamente después de los
    cambios;
-   flujo asíncrono `API → RabbitMQ → Worker → PostgreSQL` revalidado
    después de integrar autenticación y autorización.

### Endpoints de autenticación

``` text
POST /auth/register
POST /auth/login
```

El login responde conceptualmente:

``` json
{
  "token": "<jwt>",
  "user": {
    "id": "<uuid>",
    "email": "user@example.com",
    "created_at": "..."
  }
}
```

`PasswordHash` utiliza `json:"-"`, por lo que no se expone en respuestas
HTTP.

### Flujo de autenticación

``` text
POST /auth/login
      │
      ▼
 AuthHandler
      │
      ▼
 AuthService
      │
      ├── GetUserByEmail
      ├── bcrypt password check
      └── TokenManager.Generate
                    │
                    ▼
                   JWT
```

Para endpoints protegidos:

``` text
Authorization: Bearer <JWT>
          │
          ▼
    AuthMiddleware
          │
          ├── valida firma/expiración
          ▼
 userID en context.Context
          │
          ▼
       Handler
          │
          ▼
consulta/operación filtrada por owner_id
```

### Documentos autenticados

Rutas actuales:

``` text
POST /documents
GET  /documents/{id}/download
```

El owner ya no llega mediante headers de prueba. Se obtiene
exclusivamente desde el JWT/contexto.

Se verificó end-to-end:

``` text
JWT válido + documento propio  → operación permitida
sin JWT                        → 401
X-Test-Owner-ID sin JWT        → 401
documento de otro usuario      → 404
```

### Jobs autenticados

Rutas actuales:

``` text
POST /jobs
GET  /jobs
GET  /jobs/{id}
```

`POST /jobs` recibe únicamente el `documentId` necesario para indicar el
documento a procesar. El `ownerID` se obtiene del contexto autenticado.

Antes de crear el Job, la API valida:

``` text
GetDocumentByID(documentID, authenticatedOwnerID)
```

Por tanto, conocer el UUID de un documento ajeno no permite crear un Job
sobre él.

Se verificó manualmente:

``` text
user1 + documento de user1 → 202 Accepted
user1 + documento de user2 → 404 Not Found
```

Después de un caso autorizado se volvió a verificar:

``` text
POST /jobs
   ↓
QUEUED
   ↓
RabbitMQ
   ↓
Worker → PROCESSING
   ↓
COMPLETED
```

El worker completó correctamente el Job, confirmando que M4 no rompió el
flujo asíncrono existente.

`GET /jobs` utiliza `ListJobsByOwner(authenticatedOwnerID)` y solo
retorna los jobs del usuario autenticado.

`GET /jobs/{id}` utiliza `GetJobByID(id, authenticatedOwnerID)`. Se
verificó:

``` text
job propio + JWT válido → 200
job ajeno + JWT válido  → 404
sin JWT                 → 401
```

### Tests agregados/verificados en M4

Se cubrieron, entre otros:

-   hash y verificación de contraseña;
-   contraseña incorrecta;
-   generación de JWT;
-   validación de JWT;
-   rechazo de token firmado con secret diferente;
-   registro correcto;
-   normalización de email;
-   email duplicado;
-   email vacío;
-   contraseña menor de 8 caracteres;
-   login correcto;
-   login con contraseña incorrecta;
-   login con usuario inexistente;
-   middleware con token válido;
-   middleware sin token;
-   middleware con token inválido;
-   adaptación de tests HTTP de documentos al nuevo contexto
    autenticado.

### Bundles y autorización

La autorización HTTP de bundles **no se implementó dentro de M4** porque
el flujo funcional de bundles todavía pertenece al Milestone 6.

Cuando se implementen sus endpoints deben seguir obligatoriamente el
mismo patrón:

``` text
JWT → authenticated ownerID → consulta por recurso + owner_id → 404 si es ajeno
```

`ownerId` nunca debe aceptarse como identidad suministrada por el
cliente.

### Cierre de M4

La autenticación y autorización por `owner_id` quedaron completadas. Las pruebas de aislamiento requeridas se volverán a ejecutar dentro del checklist final contra la rúbrica.

## Milestone 5 --- Documentos + MinIO

**COMPLETADO Y VERIFICADO.**

Se implementó la persistencia real de documentos originales combinando
MinIO para objetos y PostgreSQL para metadata.

Resultados principales:

-   cliente MinIO implementado en Go y conectado desde la API;
-   creación/verificación automática del bucket durante el arranque;
-   operaciones `PutObject`, `GetObject` y `DeleteObject` implementadas
    y probadas;
-   `DocumentService` agregado como capa de aplicación para coordinar
    object storage y repository sin acoplarlos directamente;
-   flujo `MinIO PutObject → PostgreSQL CreateDocument` probado contra
    infraestructura real;
-   compensación implementada: si falla PostgreSQL después del upload,
    el objeto se elimina de MinIO;
-   test automático de la compensación con repository simulado en fallo;
-   endpoint `POST /documents` implementado con `multipart/form-data`;
-   endpoint `GET /documents/{id}/download` implementado mediante
    streaming desde MinIO;
-   aislamiento por `owner_id` verificado: un owner diferente obtiene
    `404` aun conociendo el UUID del documento;
-   límite de upload de 10 MB mediante `http.MaxBytesReader`;
-   archivos que exceden el límite retornan
    `413 Request Entity Too Large`;
-   normalización del filename antes de construir el `storage_key`;
-   formatos admitidos actualmente: `text/plain` (`plaintext`) y
    `text/markdown` (`markdown`);
-   formatos no soportados retornan `415 Unsupported Media Type`;
-   Markdown queda definido como el formato estructurado mínimo para el
    posterior pipeline OKF;
-   tests HTTP automáticos para owner ausente, archivo ausente, formato
    no soportado y archivo mayor de 10 MB;
-   suite de tests ejecutada satisfactoriamente con las variables de
    entorno requeridas.

Flujo verificado:

``` text
POST /documents
      │
      ▼
DocumentHandler
      │
      ▼
DocumentService
   ┌──┴──────────────┐
   ▼                 ▼
 MinIO           PostgreSQL
 original        Document metadata
   │                 │
   └──── storage_key ┘

GET /documents/{id}/download
      │
      ├── PostgreSQL: valida id + owner_id
      ▼
    MinIO
      │
      ▼
 streaming HTTP
```

**Actualización posterior (Milestone 4):** `X-Test-Owner-ID` fue
eliminado. Los endpoints de documentos están protegidos por JWT y
obtienen el `owner_id` desde `context.Context` después de pasar por
`AuthMiddleware`.

**Decisión de alcance:** PDF, DOCX y EPUB no son necesarios para el
alcance mínimo. Se mantiene Markdown como formato con estructura
detectable y texto plano como formato simple soportado.

## Milestone 6 --- Pipeline OKF

**COMPLETADO Y VERIFICADO END-TO-END.**

Se reemplazó el procesamiento temporal del worker por el pipeline OKF real y se consolidó el flujo de carga y procesamiento asíncrono.

Resultados principales:

- el worker obtiene el documento original desde MinIO usando la metadata persistida en PostgreSQL;
- conversión implementada para `plaintext` y `markdown`;
- Markdown se segmenta determinísticamente por encabezados H1 (`#`), preservando el orden;
- si un Markdown no contiene H1, se conserva como un único concepto;
- generación de bundle con `index.md`, `log.md` y `concept-NN.md`;
- validación del bundle antes de publicarlo y antes de marcar el Job como `completed`;
- `index.md` referencia los conceptos generados y conserva su orden;
- `log.md` registra archivo fuente, formato y cantidad de conceptos;
- generación de `bundle.zip` con el bundle completo;
- archivos individuales y ZIP persistidos en MinIO bajo `bundles/{ownerID}/{jobID}/`;
- `bundles.storage_key` apunta al objeto concreto `bundle.zip`;
- metadata del Bundle persistida en PostgreSQL con `job_id`, `owner_id`, `storage_key`, `is_valid` y `concept_count`;
- contrato verificado: un Job solo pasa a `completed` después de convertir, construir, validar, almacenar el bundle y persistir su metadata;
- ante error de conversión, el Job pasa a `failed`, conserva `error_message` y no se publica un bundle válido;
- endpoint protegido `GET /jobs/{id}/bundle` implementado para descargar el ZIP mediante streaming desde MinIO;
- aislamiento de bundles verificado: propietario + JWT → `200`; usuario ajeno → `404`; sin JWT → `401`;
- `GET /jobs/{id}` fue extraído a `JobHandler` y, para jobs `completed`, incluye metadata del bundle y `download_url`;
- tests agregados para el contrato del detalle: `processing → bundle:null`, `failed → bundle:null + error_message`, `completed → bundle disponible`;
- `ProcessingService` agregado como capa de aplicación para orquestar `CreateDocument → CreateJob → PublishJob`;
- `ProcessingService` depende de interfaces (`DocumentCreator`, `JobRepository`, `JobPublisher`) y fue probado con fakes;
- tests verificados para happy path, fallo de `CreateJob` sin publicación y fallo de `PublishJob` marcando el Job como `failed`;
- `POST /documents` ahora inicia automáticamente el procesamiento y responde `202 Accepted` con `document`, `jobId` y `status`;
- el antiguo `POST /jobs` fue eliminado por redundante: existe un único flujo oficial para iniciar procesamiento;
- `GET /jobs` continúa retornando correctamente los Jobs del usuario autenticado;
- descarga real de `bundle.zip` verificada con `curl`;
- suite `go test ./...` ejecutada satisfactoriamente después de los cambios.

Flujo final verificado:

``` text
POST /documents + JWT
      │
      ▼
DocumentHandler
      │
      ▼
ProcessingService
      │
      ├── DocumentService
      │      ├── MinIO: original
      │      └── PostgreSQL: Document
      │
      ├── PostgreSQL: Job → QUEUED
      └── RabbitMQ: publish JobID
                     │
                     ▼
                   Worker
                     │
                     ├── Job → PROCESSING
                     ├── obtiene original desde MinIO
                     ├── Convert / Segment
                     ├── BuildBundle
                     ├── ValidateBundle
                     ├── PackageBundle → bundle.zip
                     ├── MinIO: archivos + bundle.zip
                     ├── PostgreSQL: Bundle metadata
                     └── Job → COMPLETED → ACK
                                  │
                                  ▼
                         GET /jobs/{id}
                                  │
                                  ▼
                      GET /jobs/{id}/bundle
```

Endpoints relevantes al cierre de M6:

``` text
POST /documents
GET  /documents/{id}/download
GET  /jobs
GET  /jobs/{id}
GET  /jobs/{id}/bundle
```

`POST /jobs` ya no existe. La creación del Job forma parte del flujo de `POST /documents`.

### Alcance actual de segmentación

La segmentación reconocía únicamente encabezados H1 al cerrar M6. El punto 2 del checklist de cierre backend la extendió a H1/H2/H3 eligiendo el nivel que realmente divide el documento (ver sección 8.2). Siguen fuera de alcance las jerarquías anidadas, los assets embebidos y los conversores PDF/DOCX/EPUB: se mantienen Markdown y texto plano.

### Aspectos de M6 resueltos posteriormente en M7

Los riesgos principales que quedaron abiertos al terminar M6 fueron tratados en M7: idempotencia ante redelivery, prevención de procesamiento concurrente, retries limitados, DLQ y compensación de objetos parciales en MinIO.

La clasificación del resultado de validación (`VALID / VALID_WITH_WARNINGS / INVALID`) quedó resuelta en el punto 1 del checklist de cierre backend. Ver la sección 8.1.

## Milestone 7 --- Confiabilidad del procesamiento

**COMPLETADO Y VERIFICADO.**

Se implementó y verificó el núcleo de confiabilidad requerido para procesamiento at-least-once:

- idempotencia ante redelivery: un Job `completed` se reconoce y se ACKea sin reprocesar;
- claim atómico `queued -> processing` y protección ante procesamiento concurrente;
- recuperación de Jobs `processing` abandonados mediante lease basado en `updated_at`;
- distinción entre Job inexistente y Job temporalmente no reclamable;
- colas RabbitMQ `document_jobs_retry` y `document_jobs_dlq`;
- retry diferido con TTL para contención, sin consumir el contador de intentos;
- `Attempt` reservado para fallos reales del pipeline;
- transición controlada `processing -> queued` mediante `RequeueJob`;
- máximo de 3 retries reales; al agotarse: `FAILED` + `error_message` + DLQ + ACK;
- compensación de objetos parciales en MinIO si falla la subida o persistencia del bundle;
- verificación E2E del flujo normal posterior al cleanup: `queued -> processing -> completed`;
- verificación de ausencia de reprocesamiento para Jobs `completed` reentregados;
- verificación de recuperación de un Job `processing` stale;
- verificación de que un fallo real consume `Attempt` y un defer por contención no lo incrementa.

Con esto M7 se considera cerrado para el alcance académico del proyecto.

## Checklist obligatorio previo a M8 --- cierre backend y rúbrica

**COMPLETADO Y VERIFICADO.** Los seis puntos quedaron cerrados; el backend está listo contra la rúbrica y los contratos que consumirá React están definidos y probados.

Orden acordado:

1. **Clasificación de validación del bundle. COMPLETADO Y VERIFICADO** (ver sección 8.1).
2. **Conversión y generación del bundle. COMPLETADO Y VERIFICADO** (ver sección 8.2).
3. **Seis condiciones verificables del PDF. COMPLETADO Y VERIFICADO** (ver sección 8.3).
4. **Reproducibilidad desde entorno limpio. COMPLETADO Y VERIFICADO** (ver sección 8.4).
5. **Escalabilidad con dos workers. COMPLETADO Y VERIFICADO** (ver sección 8.5).
6. **Contrato de seguimiento del Job. COMPLETADO Y VERIFICADO** (ver sección 8.7).

### 8.1 Clasificación de validación del bundle --- COMPLETADO Y VERIFICADO

Punto 1 del checklist. La validación pasó de devolver un `error` a devolver un
resultado clasificado en los tres niveles que exige la rúbrica.

#### Modelo

`domain.BundleValidation` concentra el vocabulario y las reglas de decisión:

``` text
valid                -> estructura mínima correcta y sin observaciones
valid_with_warnings  -> publicable, con observaciones registradas
invalid              -> no se publica ni se habilita su descarga
```

`IsPublishable()` responde si el bundle puede almacenarse y descargarse.
`Err()` produce el `error_message` del Job cuando el bundle es inválido.

#### Validador

`okf.ValidateBundle` ya no se detiene en el primer problema: recorre el bundle
completo y acumula todos los hallazgos en una sola pasada.

Errores (INVALID):

-   `index.md` ausente o vacío;
-   `log.md` ausente;
-   cero conceptos;
-   archivo de concepto declarado pero ausente;
-   concepto no enlazado desde `index.md`;
-   enlace de `index.md` que no resuelve a un archivo del bundle;
-   archivos duplicados o sin nombre.

Advertencias (VALID_WITH_WARNINGS):

-   concepto sin contenido;
-   `log.md` vacío (conversión sin trazabilidad);
-   enlace del índice sin título;
-   archivo presente que el índice no referencia.

Los enlaces externos (`http://`, `https://`, `mailto:`) y las anclas internas
se ignoran al resolver enlaces, porque no apuntan a archivos del bundle.

Un documento breve sin divisiones produce un único concepto y se clasifica como
`valid`: la rúbrica exige explícitamente que una sola unidad no genere ni
fallos ni advertencias.

#### Persistencia

Migración `002_bundle_validation.sql`:

``` text
bundles.validation_status    TEXT   (valid | valid_with_warnings | invalid)
bundles.validation_warnings  TEXT[]
bundles.validation_errors    TEXT[]
```

`is_valid` se conserva como respuesta rápida a la pregunta "se puede
descargar" y se deriva de `IsPublishable()`. Solo un bundle publicable recibe
`published_at`.

Un bundle rechazado se registra igualmente en PostgreSQL como evidencia de la
validación, pero sin `published_at`, sin objetos en MinIO y sin descarga.

#### Orden del pipeline

La validación se movió delante de cualquier escritura en el object storage:

``` text
Convert -> BuildBundle -> ValidateBundle
                              |
              +---------------+---------------+
           INVALID                      VALID / VALID_WITH_WARNINGS
              |                                |
   Bundle registrado sin publicar     PackageBundle -> MinIO -> Bundle metadata
   Job -> FAILED + error_message      Job -> COMPLETED
   Sin objetos en MinIO
```

Un bundle inválido no llega nunca al object storage.

#### Contrato HTTP

`GET /jobs/{id}` expone la clasificación completa:

``` json
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

`download_url` solo se envía cuando el Job está `completed` y el bundle es
publicable. Para un Job `failed` con bundle rechazado, el detalle incluye la
clasificación y sus errores, pero ninguna URL de descarga.
`GET /jobs/{id}/bundle` responde `409 Conflict` sobre un bundle inválido.

#### Inyección de fallo para la sustentación

El pipeline siempre genera la estructura mínima, por lo que la condición
verificable "bundle incompleto" no puede ocurrir por sí sola sobre el sistema
desplegado. Se agregó una inyección de fallo controlada, desactivada salvo que
el worker reciba `OKF_FAULT_INJECTION`:

``` text
(vacío)        -> pipeline normal
drop-index     -> elimina index.md      -> invalid
drop-log       -> elimina log.md        -> invalid
empty-concept  -> vacía el concepto 1   -> valid_with_warnings
```

#### Verificación realizada

Tests automáticos:

-   documento breve -> `valid`, sin advertencias;
-   documento estructurado -> `valid`;
-   `index.md` ausente, `log.md` ausente, concepto no enlazado, enlace
    colgante y archivo duplicado -> `invalid` y no publicable;
-   concepto vacío, `log.md` vacío y archivo no referenciado ->
    `valid_with_warnings` y publicable;
-   enlaces externos del índice ignorados;
-   acumulación de todos los errores en una sola pasada;
-   inyección de fallo desactivada por defecto y cada modo produciendo su
    clasificación esperada;
-   round-trip de la clasificación en PostgreSQL;
-   bundle inválido persistido sin `published_at` y con `is_valid = false`;
-   contrato del detalle del Job para `valid`, `valid_with_warnings` e
    `invalid`;
-   suite `go test ./...` completa contra PostgreSQL y MinIO reales.

End-to-end sobre el sistema desplegado:

``` text
breve.txt        -> completed, 1 concepto,  valid,                  descarga 200
estructurado.md  -> completed, 3 conceptos, valid,                  descarga 200
drop-index       -> failed,    invalid + "bundle is missing index.md",
                    sin objetos en MinIO,                           descarga 409
empty-concept    -> completed, valid_with_warnings + advertencia,   descarga 200
```

El índice del bundle estructurado conserva el orden del documento de origen y
sus tres enlaces resuelven.

#### Corrección adicional

`.env.example` estaba listado en `.gitignore`, por lo que no viajaba en el
repositorio pese a que el README lo referencia. Se retiró de `.gitignore` para
no bloquear el punto 4 del checklist (reproducibilidad desde entorno limpio).

### 8.2 Conversión y generación del bundle --- COMPLETADO Y VERIFICADO

Punto 2 del checklist. Se auditó el conversor con configuraciones
representativas y se corrigieron los defectos encontrados.

#### Defectos detectados en la auditoría

La auditoría se hizo ejecutando el conversor real sobre casos límite. Cuatro
comportamientos eran incorrectos:

1.  **Pérdida silenciosa de contenido.** El texto anterior al primer H1 se
    descartaba por completo. Un documento con resumen introductorio perdía ese
    resumen sin aviso.

2.  **Bloques de código partidos.** Un comentario `# ...` dentro de un bloque
    ```` ```bash ```` se trataba como encabezado H1. Un manual técnico se
    partía en conceptos absurdos y el bloque de código quedaba roto.

3.  **Título con enlace = bundle inválido.** Un título legítimo como
    `# Ver [la fuente](https://x.org)` generaba un enlace anidado en
    `index.md`, la resolución de enlaces fallaba y el bundle se clasificaba
    como `invalid`.

4.  **Contenido perdido tras un encabezado sin título.** Un `#` seguido solo de
    espacios anulaba la unidad en curso y todo lo que venía después se
    descartaba.

#### Regla de segmentación

El documento se divide por el **nivel de encabezado más alto que realmente lo
divide**. Se intenta H1, luego H2 y luego H3.

``` text
# A / # B / # C            -> 3 unidades por H1
# Título + ## A / ## B     -> 3 unidades: bloque del título, luego por H2
## A / ## B (sin H1)       -> 2 unidades por H2
un solo encabezado o nada  -> 1 unidad, documento completo
```

Esto resuelve el caso más habitual en documentación real: H1 como título del
documento y H2 como secciones, que antes producía una sola unidad.

Reglas adicionales:

-   los encabezados dentro de bloques de código delimitados por ``` o `~~~` no
    son encabezados;
-   el texto anterior al primer encabezado forma su propia unidad;
-   cada unidad conserva su encabezado, de modo que cada `concept-NN.md` es
    Markdown autocontenido;
-   los finales de línea de Windows se normalizan antes de segmentar;
-   los títulos con sintaxis de enlace se sanean para la etiqueta del índice;
-   el texto plano se conserva siempre como una única unidad;
-   H4 en adelante no se considera unidad lógica.

#### index.md y log.md

`index.md` incorpora los datos del bundle además de la navegación ordenada:

``` markdown
# Index

- Source: tecnico.md
- Format: markdown
- Concepts: 4

## Concepts

1. [Manual de despliegue](concept-01.md)
2. [Requisitos](concept-02.md)
```

`log.md` pasó de tres líneas a la trazabilidad que pide la rúbrica: origen,
operaciones aplicadas, unidades detectadas en orden y resultado de la
validación. La sección de validación se añade después de validar, porque el
resultado no existe antes; solo agrega texto a un archivo ya presente, por lo
que no puede alterar la estructura ni los enlaces comprobados.

#### Cambio de contrato interno

`Convert` devuelve ahora un `okf.Conversion` (nombre, formato, conceptos y
operaciones aplicadas) en lugar de solo `[]Concept`, y `BuildBundle` recibe esa
conversión. Es lo que permite que `log.md` describa las transformaciones sin
que el worker tenga que reconstruirlas.

#### Efecto sobre la clasificación

Como cada unidad conserva su encabezado, una sección con título pero sin cuerpo
ya no parecía vacía. El validador ahora descarta el encabezado inicial antes de
decidir si un concepto está vacío, de modo que esa unidad sigue produciendo la
advertencia correspondiente y el bundle se clasifica `valid_with_warnings`.

#### Verificación realizada

Tests automáticos añadidos o reescritos:

-   documento breve, documento estructurado y documento sin encabezados;
-   preámbulo anterior al primer encabezado conservado;
-   encabezados dentro de bloques ``` y `~~~` ignorados;
-   segmentación por H2 cuando el H1 es el título del documento;
-   segmentación por H2 cuando no hay H1, y por H3 cuando solo hay H3;
-   H4 en adelante no segmenta;
-   normalización de CRLF;
-   encabezados con sangría y con cierre ATX (`## Título ##`);
-   contenido conservado tras un encabezado sin título;
-   orden de los enlaces del índice y datos del bundle en `index.md`;
-   trazabilidad de `log.md` con operaciones y unidades detectadas;
-   título con enlace saneado y bundle resultante `valid`;
-   unidad sin título etiquetada de forma utilizable;
-   `AppendValidationLog` no altera la clasificación ni el conjunto de archivos.

End-to-end sobre el sistema desplegado, con un manual técnico real que combina
H1 de título, secciones H2 y bloques de código:

``` text
tecnico.md -> completed, 4 conceptos, valid
              segmentado por H2, el bloque bash intacto dentro de concept-02.md
              index.md con datos + navegación ordenada
              log.md con operaciones, unidades y resultado de validación
breve.txt        -> completed, 1 concepto,  valid
estructurado.md  -> completed, 3 conceptos, valid
```

### 8.3 Las seis condiciones verificables --- COMPLETADO Y VERIFICADO

Punto 3 del checklist. Cada condición de la sección 6 del PDF quedó con un
procedimiento reproducible en el README, ejecutado y comprobado contra el
sistema desplegado.

``` text
1. Asincronía efectiva      -> README §9
2. Documento breve          -> README §6  (01-breve.txt)
3. Documento estructurado   -> README §3-§5 y §6
4. Bundle incompleto        -> README §7
5. Aislamiento              -> README §8
6. Ausencia de duplicados   -> README §10
```

El README abre con una tabla que mapea condición -> sección -> evidencia
esperada, para poder recorrerlas en orden durante la sustentación.

#### Lo que hizo falta añadir

Cuatro de las seis condiciones ya eran demostrables. Las otras dos estaban
implementadas pero no podían observarse:

**Asincronía efectiva.** El pipeline real termina en milisegundos, así que no
había ventana para ver que la carga no esperó a la conversión. Se añadió
`OKF_PROCESSING_DELAY` al worker, desactivado por defecto y con el mismo patrón
que la inyección de fallo. Se valida al arrancar: una duración inválida o mayor
que el lease de 5 minutos detiene el worker con un mensaje explícito, porque un
retardo mayor que el lease haría que otro worker considerase el Job abandonado.
El `5 * time.Minute` que estaba repetido en el claim salió a la constante
`staleJobLease`.

**Ausencia de duplicados.** No hacía falta código: la reentrega se provoca
republicando el mismo `jobId` en `document_jobs` con la API de management de
RabbitMQ. Así la segunda entrega la hace la cola de verdad, no un atajo de la
aplicación.

#### Evidencia obtenida

Asincronía, con retardo de 20 s:

``` text
20:19:21  upload sent
20:19:21  answered: {... "jobId":"5a15651d-...","status":"queued"}
20:19:22  "status":"processing"
   ...
20:19:44  "status":"completed"
```

Y con la API detenida a mitad del procesamiento:

``` text
19:23:25  upload -> jobId
19:23:26  docker compose stop api  ->  /health responde 000
19:23:44  worker: job completed        (siguió sin la API)
          docker compose start api ->  status completed, descarga 200
```

Duplicados, reentrega de un Job ya terminado:

``` text
=== before ===  completed | 02:20:46.112717+00 | 3e0b1a93-... | 02:20:46.108738+00
=== redelivery ===  {"routed":true}
                job ... already completed; acknowledging duplicate delivery
=== after  ===  completed | 02:20:46.112717+00 | 3e0b1a93-... | 02:20:46.108738+00
```

Duplicados, reentrega mientras el Job se procesa. Con un solo worker y
`prefetch = 1` este caso no puede ocurrir: RabbitMQ no entrega un segundo
mensaje mientras hay uno sin ACK. Con dos workers se encadenan las dos
defensas:

``` text
worker A  status=queued     -> claimed -> simulating a long conversion for 20s
worker B  status=processing -> temporarily not claimable; deferring delivery
worker A  job completed
worker A  status=completed  -> already completed; acknowledging duplicate delivery
          bundles para ese job = 1
```

El claim atómico frena al duplicado concurrente, que se difiere a
`document_jobs_retry` y vuelve 30 segundos después; para entonces el Job ya
terminó y la idempotencia lo reconoce. Qué contenedor hace de A y cuál de B
varía entre ejecuciones porque RabbitMQ reparte por turnos; lo que no varía es
la secuencia.

Los seis documentos de prueba, en una sola pasada:

``` text
01-breve.txt             completed  1 concepto   valid
02-estructurado.md       completed  3 conceptos  valid
03-manual-tecnico.md     completed  4 conceptos  valid
04-preambulo.md          completed  3 conceptos  valid
05-titulo-con-enlace.md  completed  2 conceptos  valid
06-seccion-vacia.md      completed  3 conceptos  valid_with_warnings
```

#### Documentos de prueba

Se creó `docs/filesTest/` con seis documentos, uno por configuración
representativa, y un README con la tabla de resultados esperados. El
`file.md` que ya existía era un fragmento de PowerShell de una prueba manual
anterior, no un documento; se dejó donde estaba y se anotó como tal.

#### Ergonomía de las pruebas

Los procedimientos se reescribieron para poder pegarse y ejecutarse de una vez:

-   el token y el `jobId` se guardan en `.token`, `.token-other`, `.job` y
    `.jobs`, porque las variables de shell no sobreviven entre terminales y su
    pérdida se manifestaba como un `unauthorized` engañoso;
-   cada bloque monta su propia precondición, ya que los escenarios de
    asincronía y duplicados dejan el entorno con dos workers o con retardo;
-   el estado del Job se extrae con `grep -o ... | head -1` y no con `sed`,
    porque un regex codicioso devuelve el `status` de `validation` en lugar del
    del Job;
-   los archivos auxiliares están en `.gitignore`.

### 8.4 Reproducibilidad desde entorno limpio --- COMPLETADO Y VERIFICADO

Punto 4 del checklist.

#### Problema encontrado

`docker-compose.yml` definía las credenciales en dos sitios incompatibles:

``` text
postgres   POSTGRES_PASSWORD: okf          <- literal
rabbitmq   RABBITMQ_DEFAULT_USER: okf      <- literal
api        DATABASE_URL: postgres://okf:okf@postgres:5432/okf   <- literal
api        RABBITMQ_URL: amqp://${RABBITMQ_USER}:${RABBITMQ_PASSWORD}@...
```

Quien clonara el repositorio y cambiara una contraseña en su `.env` siguiendo
el README obtenía un sistema que no arrancaba: el broker seguía con la
credencial literal mientras la API intentaba conectarse con la del `.env`.

Además `.env.example` declaraba `MINIO_ACCESS_KEY` y `MINIO_SECRET_KEY` como
copias de `MINIO_ROOT_USER` y `MINIO_ROOT_PASSWORD`. La misma credencial en dos
variables: cambiar solo una rompía el acceso al object storage.

#### Corrección

Cada credencial se define una sola vez en `.env` y se propaga:

``` text
POSTGRES_USER / POSTGRES_PASSWORD / POSTGRES_DB
    -> contenedor postgres
    -> healthcheck pg_isready
    -> DATABASE_URL de API y worker

RABBITMQ_USER / RABBITMQ_PASSWORD
    -> contenedor rabbitmq
    -> RABBITMQ_URL de API y worker
    -> consola de management

MINIO_ROOT_USER / MINIO_ROOT_PASSWORD
    -> contenedor minio
    -> MINIO_ACCESS_KEY / MINIO_SECRET_KEY de API y worker
```

Se eliminaron `MINIO_ACCESS_KEY` y `MINIO_SECRET_KEY` de `.env.example`. Las
variables que compose necesita y las que `.env.example` declara coinciden
exactamente: ni sobra ni falta ninguna.

Propagación comprobada sobrescribiendo las credenciales:

``` text
DATABASE_URL: postgres://otro:clave123@postgres:5432/otradb?sslmode=disable
RABBITMQ_URL: amqp://ruser:rpass@rabbitmq:5672/
MINIO_ACCESS_KEY: miniouser
pg_isready -U otro -d otradb
```

#### README

La sección de configuración pedía crear un `.env` a mano y mostraba un ejemplo
que omitía `JWT_SECRET`, `MINIO_ENDPOINT`, `MINIO_USE_SSL` y `MINIO_BUCKET`:
seguirlo al pie de la letra no levantaba el sistema. Ahora la instrucción es
`cp .env.example .env` y `.env.example` es la única fuente de verdad, con cada
variable comentada.

También se corrigieron los requisitos: no hace falta Go ni Node para ejecutar
ni probar la plataforma, solo Docker y Compose v2. Se añadió que las pruebas
manuales usan `curl` y `unzip`, ambos disponibles en Git Bash.

#### Verificación desde cero

Se levantó la plataforma completa usando únicamente `.env.example`, bajo un
nombre de proyecto distinto para no destruir los datos locales:

``` text
docker compose stop
docker compose -p okf-clean --env-file .env.example up -d --build
```

Resultado sobre una base de datos y un object storage vacíos:

``` text
migration applied: 001_init.sql
migration applied: 002_bundle_validation.sql
MinIO connected and bucket ready
API listening on :8080
worker waiting for jobs

frontend (5173)        200
rabbitmq mgmt (15672)  200
minio console (9001)   200

registro + login          usuario creado, JWT de 254 caracteres
03-manual-tecnico.md      completed, 4 conceptos, valid, descarga 200
bundle descargado         index.md, log.md, concept-01..04.md
los seis documentos       idénticos a la tabla esperada
```

No hizo falta ningún paso de migración aparte: la API aplica el esquema al
arrancar.

La pila limpia se eliminó con `down -v` y la original se restauró intacta
(171 usuarios, 155 jobs, 89 bundles).

El procedimiento quedó documentado en el README como *Verifying from a clean
environment*, y sirve directamente para el segmento 3 del video, que exige
mostrar el despliegue con un solo comando desde un entorno limpio.

### 8.5 Escalabilidad con dos workers --- COMPLETADO Y VERIFICADO

Punto 5 del checklist. La sección 8.3 ya mostraba dos workers compitiendo por
el mismo Job; faltaba demostrar el reparto de Jobs distintos y el efecto real
de escalar.

#### Medición

Los seis documentos de prueba, con un retardo artificial de 10 s para que el
trabajo dure lo suficiente como para observarse:

``` text
2 workers                      1 worker
22:07:49  completados 0/6      22:09:16  completados 0/6
22:07:57  completados 2/6      22:09:24  completados 1/6
22:08:05  completados 2/6      22:09:32  completados 1/6
22:08:13  completados 4/6      22:09:40  completados 2/6
22:08:21  completados 6/6      22:09:48  completados 3/6
                               22:09:57  completados 4/6
TOTAL 38 s                     22:10:05  completados 5/6
                               22:10:12  completados 6/6
                               TOTAL 63 s
```

Con dos workers los Jobs terminan de dos en dos, porque `prefetch = 1` hace que
cada worker retenga un solo mensaje a la vez. Con uno terminan de uno en uno y
el lote tarda aproximadamente el doble. La API no se tocó en ningún momento:
solo cambió el número de workers.

#### Reparto y ausencia de duplicados

``` text
01-breve.txt             worker-1=0 worker-2=1 total=1
02-estructurado.md       worker-1=1 worker-2=0 total=1
03-manual-tecnico.md     worker-1=0 worker-2=1 total=1
04-preambulo.md          worker-1=1 worker-2=0 total=1
05-titulo-con-enlace.md  worker-1=0 worker-2=1 total=1
06-seccion-vacia.md      worker-1=1 worker-2=0 total=1

worker-1: 3 jobs reclamados
worker-2: 3 jobs reclamados
```

Cada `total` es 1: el trabajo se repartió y ningún Job se reclamó dos veces. El
reparto 3/3 viene del round robin de RabbitMQ y puede variar con carga
desigual; lo que no puede variar es que ningún Job se reclame dos veces.

Un bundle por Job, con cualquier número de workers:

``` text
6 filas, todas completed, todas con bundles = 1
```

#### Decisión: dos réplicas por defecto

A raíz de esta medición, `docker-compose.yml` declara `deploy.replicas: 2` en
el servicio worker. Un único `docker compose up -d` deja el sistema con el
procesamiento repartido entre dos workers.

Razones:

-   el segmento 3 del video exige mostrar el despliegue completo con un solo
    comando; con dos réplicas por defecto, el `docker compose ps` posterior ya
    evidencia la arquitectura replicada sin pasos añadidos;
-   el criterio de procesamiento asíncrono menciona explícitamente workers
    escalables de forma independiente: que el despliegue por defecto traiga dos
    demuestra que la capacidad es real y no teórica;
-   el camino multi-worker deja de ser un caso especial y se ejercita en cada
    prueba, en lugar de solo cuando se monta la demostración.

`--scale worker=1` sigue funcionando para las comprobaciones que necesitan un
único worker para ser deterministas; el README indica en cada caso cuándo se
reduce a propósito.

#### Detalle encontrado al escribir el procedimiento

`docker compose exec -T` lee de la entrada estándar, así que dentro de un bucle
`while read ... done < .jobs` se traga las líneas restantes y el bucle termina
tras el primer Job. El procedimiento del README usa `< /dev/null` en esa
llamada y lo explica, para que la comprobación no dé un falso correcto.

### 8.6 Comportamiento con múltiples usuarios --- REVISADO

Revisión hecha a raíz de la pregunta "¿qué pasa con varios usuarios subiendo
documentos a la vez?". Se auditó el código en lugar de suponer.

#### Lo que aguanta

-   El aislamiento no depende del timing: toda consulta filtra por `owner_id` y
    las claves de almacenamiento van namespaceadas
    (`documents/{ownerID}/{uuid}/{filename}`, `bundles/{ownerID}/{jobID}/`).
    Dos usuarios que suban un archivo con el mismo nombre no colisionan.
-   La API no comparte estado mutable entre peticiones: Go atiende cada una en
    su goroutine. Escalarla a varias réplicas no requeriría cambios.
-   El claim atómico ya cubre el único punto donde dos procesamientos podrían
    pisarse.

#### Corregido: pool de conexiones sin límite

`database/sql` no limita las conexiones abiertas por defecto. Bajo concurrencia
la API podía superar el `max_connections` de PostgreSQL (100 por defecto) y
fallar con `too many clients already`: un fallo por agotamiento, no por carga.

Se acotó el pool en `database.New`:

``` text
SetMaxOpenConns(25)
SetMaxIdleConns(5)
SetConnMaxLifetime(5 * time.Minute)
```

25 por proceso deja margen para la API y varios workers sobre la configuración
por defecto de PostgreSQL. Con el tope, el exceso de peticiones espera turno en
lugar de agotar el servidor. `TestConnectionPoolIsBounded` comprueba el límite
contra PostgreSQL real.

#### Limitaciones aceptadas y documentadas

Quedan registradas en el README como *Known limitations*, para el cierre del
video que exige declarar limitaciones conocidas:

-   **Sin equidad entre usuarios.** Una única cola `document_jobs` FIFO. El
    throughput total es el número de workers. Si un usuario encola cien
    documentos, el siguiente espera detrás de los cien: head-of-line blocking.
    `prefetch = 1` reparte con equidad entre *workers*, no entre usuarios, y no
    hay cuotas ni rate limiting. Colas por usuario o prioridades lo
    resolverían, muy por encima del alcance.
-   **Uploads en memoria.** `multipart` bufferiza cada subida antes de que la
    API la envíe a MinIO. El límite de 10 MB lo mantiene acotado. Importante:
    subir `maxUploadSize` por encima de 32 MB haría que `multipart` escribiera
    archivos temporales en el disco local del contenedor, justo lo que la
    arquitectura evita; el límite es lo que lo impide.
-   **El reintento de upload no es idempotente.** `idempotency_key` es un UUID
    nuevo en cada llamada, así que garantiza unicidad pero nunca deduplica:
    reintentar `POST /documents` crea un segundo documento y un segundo Job.
    Es distinto de la idempotencia ante reentrega de la cola, que sí está
    implementada y verificada. El arreglo correcto sería una clave de
    idempotencia suministrada por el cliente; mientras tanto, el frontend debe
    deshabilitar el control de subida tras el primer clic.

### 8.7 Contrato de seguimiento del Job --- COMPLETADO Y VERIFICADO

Punto 6 del checklist, y último antes de M8. Se definió y verificó lo que el
frontend consumirá para seguir un `jobId`, detectar el final y llevar al
usuario a la descarga, conservando la vista general de Jobs.

El seguimiento se resuelve consultando `GET /jobs/{id}` o refrescando
`GET /jobs` hasta que el Job sea terminal. Se descartaron las notificaciones
push: el enunciado pide seguimiento del estado, no notificaciones, y un canal
push sería un segundo mecanismo sin crédito adicional.

#### Estado terminal explícito

`JobStatus.IsTerminal()` distingue los estados finales de los transitorios, y
tanto `GET /jobs` como `GET /jobs/{id}` exponen un booleano `terminal`.

``` text
queued, processing   -> terminal: false   (seguir consultando)
completed, failed    -> terminal: true    (dejar de consultar)
```

El cliente no tiene que codificar por su cuenta qué estados son finales: si
mañana se añadiera un estado, el criterio de parada seguiría siendo correcto.

#### `download_url` como única autoridad

La URL de descarga se emite solo cuando el Job terminó correctamente **y** la
validación permitió publicar el bundle. Su ausencia significa que no hay nada
que descargar. El README lo dice de forma explícita: el cliente no debe
construir la URL a mano, porque acabaría ofreciendo una descarga que responde
`409`.

Un bundle con advertencias sigue siendo descargable: `valid_with_warnings` es
un resultado exitoso, no un fallo parcial.

#### `GET /jobs` reescrito

Era un closure dentro de `main.go` que devolvía `[]*domain.Job` en crudo. Tenía
tres problemas para una vista de lista:

1.  **Sin nombre de documento.** El usuario habría visto una lista de UUIDs sin
    forma de reconocer qué subió.
2.  **Sin bundle.** Pintar un botón de descarga habría exigido una petición de
    detalle por fila.
3.  **Devolvía `null` con cero Jobs**, no `[]`. Un cliente que recorriera la
    respuesta habría fallado con un usuario nuevo.

Ahora es `JobHandler.List` y `ListJobsByOwner` resuelve en una sola consulta el
join de `jobs`, `documents` y `bundles`. Cada entrada trae el nombre y formato
del documento, el estado, el flag `terminal`, el `error_message` y el bundle
con su clasificación y su `download_url` cuando corresponde.

Garantías del listado, documentadas para que el frontend pueda apoyarse en
ellas:

-   orden descendente por `created_at` con `id` como desempate, para que las
    filas no se intercambien entre refrescos y la lista no parpadee;
-   siempre un array JSON, nunca `null`;
-   solo los Jobs del propietario autenticado;
-   `bundle` es `null` mientras no exista.

La vista general de Jobs es navegación normal y existe con independencia de que
el cliente esté siguiendo un Job concreto: notificar que uno terminó no la
sustituye.

#### Verificación

Bucle de seguimiento tal como lo hará el frontend, con retardo de 15 s:

``` text
23:05:03  subida aceptada, jobId=018850e8-...
23:05:04  status=processing terminal=False download=-
23:05:09  status=processing terminal=False download=-
23:05:15  status=processing terminal=False download=-
23:05:20  status=completed  terminal=True  download=/jobs/018850e8-.../bundle
```

Camino fallido con bundle rechazado, en detalle y en listado:

``` text
status=failed  terminal=true
error_message="bundle validation failed: bundle is missing index.md"
bundle.validation.status=invalid  (sin download_url)
```

Usuario nuevo sin Jobs:

``` text
GET /jobs -> []
```

Tests añadidos: flag `terminal` para los cuatro estados, serialización de la
lista vacía como `[]`, listado con documento y bundle, listado de un bundle
rechazado sin descarga, y contra PostgreSQL real el join con documento, el
bundle publicado con su clasificación y la lista vacía no nula.

#### Cierre del checklist

Con esto los seis puntos del checklist previo a M8 quedan completados y
verificados. El backend está cerrado contra la rúbrica.

### 8.8 Nombre y apellido en el registro --- COMPLETADO Y VERIFICADO

El formulario de registro del frontend pide nombre y apellido, así que
`users` los almacena (migración `003_user_names.sql`) y viajan en la
respuesta de `/auth/register` y en el `user` de `/auth/login`.

Son **opcionales**: el enunciado solo exige credenciales, de modo que un
registro sin ellos responde `201` y los guarda como cadena vacía, no como
`NULL`. La columna es `NOT NULL DEFAULT ''`, lo que evita que el frontend
tenga que distinguir entre "vacío" y "ausente". `Register` los recorta
igual que al email, porque llegan de un formulario.

Verificado contra el stack real: registro con `"  Pepe  "` devuelve
`"Pepe"`, el login lo repite, y un registro sin los campos responde `201`.



## Milestone 8 --- Frontend funcional

**PENDIENTE.** El backend y su checklist están cerrados; los contratos que debe
consumir están documentados en la sección *What the frontend must build* del
README.

Alcance, exactamente lo que exigen el enunciado y el video:

- autenticación y manejo del JWT;
- carga mediante `POST /documents` mostrando de inmediato el `jobId` devuelto;
- listado de Jobs accesible por navegación normal, refrescado mientras haya
  Jobs en curso;
- detalle del Job en una ruta direccionable `/jobs/:id`;
- visualización de los estados `queued`, `processing`, `completed` y `failed`,
  incluido el caso de bundle rechazado sin descarga;
- descarga de `GET /jobs/{id}/bundle` cuando el Job esté completado;
- manejo claro de errores de autorización y procesamiento;
- deshabilitar el control de subida tras el primer clic: el reintento de
  `POST /documents` no es idempotente (ver sección 8.6).

Dos decisiones que vienen del video y no del gusto:

1.  **El detalle debe ser direccionable por URL.** El segmento 6 exige
    demostrar que un usuario no alcanza un recurso ajeno; con el id en la URL
    basta con pegar el `jobId` de otro usuario y mostrar que la vista responde
    "no encontrado".
2.  **El detalle debe pintar el camino fallido.** El segmento 7 exige mostrar
    un bundle incompleto que no se publica: un Job `failed` con su
    `error_message`, la validación `invalid` y sin botón de descarga.

Y un detalle de implementación que cuesta horas descubrir: `download_url` exige
la cabecera `Authorization`, así que no funciona como `href` de un enlace. El
síntoma engaña, porque la descarga parece correcta y entrega un archivo de 13
bytes que dice `unauthorized`. Hay que pedirlo con `fetch` y convertir la
respuesta en descarga.

**Fuera de alcance por decisión:** notificaciones push. El enunciado pide
seguimiento del estado, no notificaciones, y refrescar `GET /jobs` ya lo
cubre. Añadir un canal push supondría un segundo mecanismo que construir,
demostrar y mantener sin crédito adicional.

### Cómo alcanza el frontend a la API

El frontend llamaba a rutas relativas (`/auth/register`) confiando en el
proxy de `vite.config.ts`. Eso solo existe en `npm run dev`: en el
contenedor los estáticos los sirve nginx sin configuración de proxy, así
que la llamada moría en su propio 404 y la SPA mostraba el HTML de error
de nginx dentro del formulario.

Se resolvió sirviendo la API bajo el mismo origen con el prefijo `/api`:

-   `frontend/nginx.conf` --- nuevo --- redirige `/api/` a `http://api:8080/`
    y devuelve `index.html` para el resto de rutas;
-   `vite.config.ts` usa el mismo prefijo, de modo que desarrollo y
    contenedor resuelven URLs idénticas;
-   `App.tsx` pasa de `const API = ''` a `const API = '/api'`.

El prefijo no es cosmético. Un proxy directo sobre `/jobs` haría que abrir
`/jobs/:id` en el navegador devolviera JSON en lugar de la aplicación, y
esa ruta tiene que ser navegable para el segmento 6 del video. Al ser mismo
origen tampoco hace falta CORS en el backend.

Verificado por el puerto 5173: registro, login, subida multipart, consulta
del Job y descarga del ZIP (`200 application/zip`, 6 archivos), y
`GET /jobs/algun-id` devolviendo `200 text/html`.

## Milestone 9 --- Entrega, sustentación y cierre

**PENDIENTE.**

- ejecutar el recorrido final desde un entorno limpio;
- consolidar las pruebas requeridas por la rúbrica;
- preparar evidencia de arquitectura, asincronía, aislamiento, validación e idempotencia;
- preparar el video de sustentación de máximo 20 minutos;
- documentar limitaciones conocidas y decisiones de diseño.

------------------------------------------------------------------------

# 9. Alcance opcional / bonus si sobra tiempo

Estas capacidades no bloquean el cierre de backend ni el inicio de M8:

- segundo formato de entrada adicional al formato estructurado mínimo;
- extracción y publicación de `assets/`;
- cancelación de Jobs;
- métricas y observabilidad adicional;
- cálculo separado de conformidad OKF / OKF conformity score;
- streaming de bundles grandes;
- reintento idempotente de Jobs fallidos con vínculo al anterior.

La prioridad es completar primero todos los requisitos evaluables del backend, después el frontend funcional y, solo si queda tiempo, implementar bonus.

------------------------------------------------------------------------

# 10. Problemas ya encontrados y solucionados

## Vite / Rolldown en Windows

Se produjo un error de native binding con Node/Vite/Rolldown.

Se evitó convertir bindings Windows en dependencias obligatorias del
proyecto y se corrigió la instalación.

## Node

La versión local inicial no satisfacía correctamente los requisitos de
Vite. Se actualizó/alineó.

## Docker multiplataforma

Se adoptó la regla de nunca copiar `node_modules` del host al
contenedor.

El frontend usa build Linux dentro de Docker y `.dockerignore`.

## Go

`go.mod` requería una versión de Go superior a la imagen Docker inicial.
Se alineó el Dockerfile con la versión requerida.

## RabbitMQ startup race

API y worker intentaron conectar antes de que RabbitMQ aceptara
conexiones.

Se agregó:

-   healthcheck;
-   `depends_on`/condición de salud;
-   retry en Go.

La prueba mostró un primer `connection refused` y conexión exitosa en el
segundo intento.

## Mensaje sin jobId

Existían mensajes de prueba antiguos/incompatibles.

Se agregó validación de `job.JobID == ""` y NACK.

------------------------------------------------------------------------

# 11. Próximo paso recomendado

Los Milestones 1, 2, 3, 4, 5, 6 y 7 están completados y verificados.

Antes de iniciar M8 se completará el **checklist de cierre backend y rúbrica** en este orden:

1. ~~revisar `VALID / VALID_WITH_WARNINGS / INVALID`~~ --- COMPLETADO (sección 8.1);
2. ~~revisar conversión/generación del bundle con distintas configuraciones representativas~~ --- COMPLETADO (sección 8.2);
3. ~~ejecutar y documentar las seis condiciones verificables del PDF~~ --- COMPLETADO (sección 8.3);
4. ~~verificar README + `.env.example` desde un entorno limpio~~ --- COMPLETADO (sección 8.4);
5. ~~demostrar procesamiento con dos workers y ausencia de duplicados~~ --- COMPLETADO (sección 8.5);
6. ~~definir/probar el contrato de seguimiento por `jobId`~~ --- COMPLETADO (sección 8.7);
7. iniciar M8 --- frontend funcional.

Los desarrollos opcionales quedan fuera de esta ruta crítica y solo se realizarán si sobra tiempo.

------------------------------------------------------------------------

# 12. Estrategia de trabajo para próximas sesiones

Continuar incrementalmente.

Reglas:

1.  No implementar varios milestones simultáneamente.
2.  Explicar conceptos de Go al introducir código nuevo.
3.  Mantener el código sencillo y legible.
4.  Cada cambio debe incluir una prueba verificable.
5.  No asumir que una capa funciona solo porque compila.
6.  Mantener API y worker desacoplados.
7.  La API debe permanecer stateless.
8.  No almacenar documentos en filesystem local de los contenedores.
9.  PostgreSQL guarda metadata; MinIO guarda objetos.
10. RabbitMQ coordina trabajo; no debe convertirse en fuente de verdad
    de estados.
11. PostgreSQL debe ser la fuente de verdad del estado del job.
12. Los workers deben poder escalar horizontalmente.
13. Diseñar asumiendo que un mensaje puede entregarse más de una vez.
14. No introducir complejidad de producción sin una razón asociada a un
    requisito o riesgo concreto.

------------------------------------------------------------------------

# 13. Contexto para una nueva conversación

Usar este texto junto con este archivo:

> Estoy desarrollando la Plataforma OKF Cloud descrita en
> `PROJECT_STATUS.md`. Continúa desde el estado documentado. El backend
> está cerrado y verificado, y también el checklist previo al frontend:
> clasificación `VALID / VALID_WITH_WARNINGS / INVALID`, revisión de la
> conversión con configuraciones representativas, las seis condiciones
> verificables del PDF, reproducibilidad desde entorno limpio,
> escalabilidad con dos workers y el contrato de seguimiento por `jobId`.
> Todos los procedimientos son reproducibles desde el README. El
> frontend (M8) lo desarrolla otra persona contra los contratos
> documentados en la sección *What the frontend must build* del README;
> Queda M9: recorrido final desde entorno limpio,
> evidencia para la rúbrica y video de máximo 20 minutos. Los bonus solo
> se implementan si sobra tiempo. Quiero avanzar paso a paso y verificar
> cada cambio sin sobrecomplicar el proyecto.

------------------------------------------------------------------------

# 14. Estado resumido

``` text
Infraestructura Docker          ██████████  completa
RabbitMQ async flow             ██████████  completo
Robustez / idempotencia         ██████████  completa para alcance actual
Capa PostgreSQL                 ██████████  completa e integrada
Autenticación/autorización      ██████████  completa para documentos/jobs/bundles
Documentos + MinIO              ██████████  completo y verificado
Pipeline OKF                    ██████████  completo y verificado E2E
Confiabilidad M7                ██████████  completa y verificada
Cierre backend contra rúbrica   ██████████  completo (6/6)
Frontend funcional M8           ░░░░░░░░░░  pendiente (otra persona)
Entrega/sustentación            ░░░░░░░░░░  pendiente
```

**Siguiente acción:** Milestone 8 --- frontend funcional. El checklist previo está cerrado: la clasificación de validación, la conversión, las seis condiciones verificables, la reproducibilidad desde entorno limpio, la escalabilidad con dos workers y el contrato de seguimiento están completados y verificados. El contrato que consumirá React está documentado en la sección *API contract* del README.
