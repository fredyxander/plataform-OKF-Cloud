# Estado del proyecto --- Plataforma OKF Cloud

**Fecha de corte:** 21 de agosto de 2026\
**Repositorio:** `fredyxander/plataform-OKF-Cloud`\
**Propósito de este documento:** servir como checkpoint técnico para
continuar el desarrollo en una nueva sesión sin perder decisiones,
avances, pruebas realizadas ni deuda técnica conocida.

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

**Limitación conocida:** este retry protege principalmente el arranque.
Todavía no existe reconexión AMQP completa cuando RabbitMQ se cae
durante runtime.

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

### Deuda técnica no bloqueante identificada durante M4

-   varios handlers de jobs permanecen inline en `cmd/api/main.go`;
    conviene extraer posteriormente `JobHandler` y, cuando aporte valor,
    `JobService`;
-   `application` conoce actualmente errores definidos en `database`
    (`ErrNotFound` / `ErrAlreadyExists`); puede desacoplarse mediante
    errores de aplicación/repositorio compartidos;
-   configuración de RabbitMQ está centralizada en `internal/config`,
    mientras PostgreSQL, MinIO y JWT todavía se leen parcialmente desde
    `main.go`;
-   varias pruebas end-to-end de autorización se verificaron manualmente
    y conviene automatizarlas antes del cierre;
-   autorización de bundles queda pendiente hasta implementar el flujo
    de bundles en el Milestone 6.

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

**PENDIENTE.**

Objetivos:

-   worker obtiene documento desde MinIO;
-   conversión;
-   generación de `index.md`;
-   `log.md`;
-   conceptos/secciones;
-   assets;
-   validación;
-   almacenamiento de bundle en MinIO;
-   creación de registro `Bundle`.

## Milestone 7 --- Confiabilidad del procesamiento

**PENDIENTE.**

Objetivos detallados en la siguiente sección.

## Milestone 8 --- Frontend funcional

**PENDIENTE.**

-   autenticación;
-   upload;
-   lista de documentos;
-   creación de job;
-   estado de jobs;
-   bundle disponible;
-   descarga;
-   notificación de finalización.

## Milestone 9 --- Observabilidad, pruebas y cierre

**PENDIENTE.**

-   logs estructurados;
-   health/readiness;
-   métricas si aplica;
-   pruebas;
-   pruebas de fallos;
-   documentación;
-   demostración de escalabilidad;
-   validación de requisitos OKF.

------------------------------------------------------------------------

# 9. Deuda técnica y decisiones que NO deben olvidarse

## 9.1 RabbitMQ prefetch / QoS --- RESUELTO EN MILESTONE 3

Se configuró `prefetch = 1` mediante QoS en el consumer y se verificó
con dos jobs consecutivos.

Con múltiples workers y trabajos largos queremos evitar que un consumer
acumule demasiados mensajes sin confirmar.

Objetivo probable:

``` text
prefetch = 1
```

Conceptualmente:

``` text
Worker A ocupado → no recibe otro job todavía
Worker B libre   → puede recibir el siguiente
```

Esto será importante al probar escalamiento horizontal.

## 9.2 Publisher confirms

El hecho de que `Publish` retorne sin error no cubre todos los
escenarios de pérdida entre producer y broker.

Evaluar publisher confirms para saber que RabbitMQ aceptó el mensaje.

## 9.3 Retry de jobs

Actualmente un mensaje inválido se descarta con NACK sin requeue.

Para fallos transitorios del procesamiento se necesita una política
explícita:

``` text
Job
 ↓
fallo
 ↓
retry limitado
 ↓
si sigue fallando
 ↓
DLQ / FAILED
```

No usar requeue infinito.

## 9.4 Dead-letter queue

Agregar una DLQ para mensajes que excedan el número de intentos o sean
imposibles de procesar.

## 9.5 Idempotencia

El esquema de jobs ya incluye `idempotency_key`, pero debe integrarse al
flujo.

El worker debe poder recibir el mismo mensaje más de una vez sin generar
efectos duplicados.

RabbitMQ + ACK manual implica semántica compatible con entregas
repetidas en determinados fallos. El diseño debe asumir
**at-least-once**, no "exactly once".

## 9.6 Reconexión RabbitMQ en runtime

`NewRabbitMQWithRetry` resuelve el arranque, no una desconexión
posterior.

Más adelante evaluar:

-   `NotifyClose`;
-   backoff;
-   recrear connection;
-   recrear channel;
-   redeclarar cola;
-   volver a registrar consumer;
-   recuperación del publisher.

## 9.7 Graceful shutdown

API y worker deberían manejar señales de terminación.

El worker idealmente debe:

1.  dejar de aceptar nuevos trabajos;
2.  terminar o manejar correctamente el actual;
3.  ACK/NACK según resultado;
4.  cerrar channel/conexiones.

## 9.8 Estados de job

Definir y validar formalmente las transiciones permitidas.

Propuesta:

``` text
QUEUED
  ↓
PROCESSING
  ├──→ COMPLETED
  ├──→ FAILED
  └──→ CANCELLED
```

Evitar transiciones arbitrarias.

## 9.9 Consistencia DB + RabbitMQ

Existe un problema arquitectónico futuro importante:

``` text
CreateJob en PostgreSQL ✓
Publish RabbitMQ ✗
```

El job podría quedar `QUEUED` pero nunca entrar a la cola.

Debe evaluarse una estrategia, por ejemplo:

-   transactional outbox; o
-   reconciliación/republicación controlada.

No es necesario implementarlo inmediatamente, pero no debe ignorarse
antes del cierre del proyecto.

## 9.10 Migraciones

El sistema de migraciones tiene una buena base (tracking, transacción y
advisory lock).

Se debe verificar especialmente:

-   comportamiento desde volumen PostgreSQL vacío;
-   ejecución repetida sin volver a aplicar `001_init.sql`;
-   nueva migración `002_...sql`;
-   comportamiento con dos instancias de API;
-   rollback ante SQL inválido.

## 9.11 Worker + PostgreSQL --- RESUELTO EN MILESTONE 3

El worker está integrado directamente con PostgreSQL y actualiza los
estados de los jobs.

No debe depender de la API para actualizar jobs.

Arquitectura correcta:

``` text
Worker ─────────► PostgreSQL
Worker ─────────► MinIO
Worker ─────────► RabbitMQ

Worker ─X───────► API
```

## 9.12 Healthchecks adicionales

RabbitMQ ya tiene healthcheck.

Revisar/agregar:

-   PostgreSQL;
-   MinIO;
-   API readiness.

Diferenciar cuando sea útil:

-   liveness: el proceso está vivo;
-   readiness: puede atender correctamente sus dependencias.

------------------------------------------------------------------------

## 9.13 Autenticación/autorización --- BASE RESUELTA EN MILESTONE 4

Registro, login, bcrypt, JWT, middleware y aislamiento por owner están
implementados para documentos y jobs.

Pendientes no bloqueantes:

-   automatizar más pruebas HTTP end-to-end de aislamiento multiusuario;
-   centralizar `JWT_SECRET`, PostgreSQL y MinIO en la capa de
    configuración;
-   desacoplar errores de aplicación respecto de `internal/database`;
-   extraer handlers de jobs actualmente inline en `cmd/api/main.go`;
-   aplicar autorización a bundles cuando sus endpoints se implementen
    en M6.

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

Los Milestones 1, 2, 3, 4 y 5 están completados y verificados.

El siguiente bloque funcional recomendado es:

## Milestone 6 --- Pipeline OKF

Orden sugerido:

1.  reemplazar el procesamiento temporal (`time.Sleep`) por
    procesamiento real;
2.  hacer que el worker obtenga el documento original desde MinIO;
3.  generar la estructura mínima del bundle OKF, comenzando por
    `index.md`;
4.  generar `log.md` y estructura de conceptos/secciones según el
    alcance;
5.  validar el bundle antes de marcar el Job como completado;
6.  almacenar el bundle generado en MinIO;
7.  crear/persistir el registro `Bundle` en PostgreSQL;
8.  implementar los endpoints necesarios para consultar/descargar
    bundles;
9.  aplicar a bundles el patrón de autorización de M4 (`owner_id` desde
    JWT);
10. verificar end-to-end `documento → job → worker → bundle`.

No mezclar todavía M7 (retry/DLQ/idempotencia avanzada) salvo que una
necesidad concreta del pipeline obligue a resolver una parte antes.

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
> `PROJECT_STATUS.md`. Continúa desde el estado documentado. Los
> Milestones 1, 2, 3, 4 y 5 están completados y verificados. M4
> implementó registro, bcrypt, login, JWT, middleware y autorización por
> `owner_id` para documentos y jobs. `X-Test-Owner-ID` y `/jobs/test`
> fueron eliminados. Los endpoints actuales de jobs son `POST /jobs`,
> `GET /jobs` y `GET /jobs/{id}`. El siguiente trabajo recomendado es el
> Milestone 6: pipeline OKF y generación de bundles. La autorización
> HTTP de bundles debe implementarse cuando exista ese flujo. Quiero
> avanzar paso a paso y verificar cada cambio. Mantén buenas prácticas
> de arquitectura sin sobrecomplicar prematuramente el proyecto.

------------------------------------------------------------------------

# 14. Estado resumido

``` text
Infraestructura Docker          ██████████  completa
RabbitMQ async flow             ██████████  completo
Robustez básica RabbitMQ        ██████████  completa para startup
Capa PostgreSQL                 ██████████  completa e integrada
Autenticación/autorización      ██████████  completa para documentos/jobs
Documentos + MinIO              ██████████  completo y verificado
Pipeline OKF                    ░░░░░░░░░░  siguiente milestone
Confiabilidad avanzada          ████░░░░░░  prefetch/ACK listos; faltan retry/DLQ/idempotencia
Frontend funcional              ░░░░░░░░░░  pendiente
Observabilidad/pruebas finales  ░░░░░░░░░░  pendiente
```

**Siguiente acción:** iniciar el Milestone 6 --- pipeline OKF,
manteniendo el desarrollo incremental y las pruebas verificables. La
autorización de bundles se completa cuando se implementen sus endpoints.
