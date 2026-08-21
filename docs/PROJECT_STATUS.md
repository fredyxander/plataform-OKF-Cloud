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
│   │   ├── config/
│   │   ├── database/
│   │   │   ├── db.go
│   │   │   ├── migrate.go
│   │   │   ├── user_repo.go
│   │   │   ├── document_repo.go
│   │   │   ├── job_repo.go
│   │   │   └── bundle_repo.go
│   │   ├── domain/
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
POST /jobs/test
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

1.  recibe `POST /jobs/test`;
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
POST /jobs/test
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

**Elementos todavía temporales:** `/jobs/test`, recepción manual de
`ownerId`/`documentId` y `time.Sleep(10 * time.Second)` como simulación
del procesamiento real.

## Milestone 4 --- Autenticación y autorización

**PENDIENTE**, aunque `users` y `owner_id` ya preparan parte de la
persistencia.

Objetivos:

-   registro;
-   password hashing;
-   login;
-   JWT/sesión definida por arquitectura;
-   middleware de autenticación;
-   aislamiento por owner;
-   autorización de documentos/jobs/bundles.

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

**Elemento temporal:** `X-Test-Owner-ID` se utiliza para identificar al
propietario mientras se implementa el Milestone 4. Debe ser reemplazado
por la identidad obtenida del mecanismo de autenticación.

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

Los Milestones 1, 2, 3 y 5 están completados y verificados. El siguiente
bloque de trabajo es:

## Milestone 4 --- Autenticación y autorización

Orden recomendado:

1.  definir el flujo de registro y login;
2.  implementar password hashing;
3.  definir e implementar JWT/sesión según la arquitectura elegida;
4.  agregar middleware de autenticación;
5.  obtener `owner_id` desde la identidad autenticada, eliminando su
    envío manual desde `/jobs/test`;
6.  aplicar y probar autorización sobre documentos, jobs y bundles;
7.  verificar explícitamente que conocer un UUID ajeno no permite
    acceder al recurso ni revela su existencia.

Antes de reemplazar `/jobs/test`, conservar el flujo ya verificado
`CreateJob → RabbitMQ → Worker → PostgreSQL` como referencia de
integración.

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
> Milestones 1, 2, 3 y 5 están completados y verificados. El siguiente
> trabajo es el Milestone 4: autenticación y autorización. Actualmente
> los endpoints de documentos usan temporalmente `X-Test-Owner-ID`; debe
> reemplazarse por la identidad autenticada. Quiero avanzar paso a paso
> y verificar cada cambio. No tengo mucha experiencia con Go, así que
> explica los conceptos del lenguaje cuando aparezcan. Mantén buenas
> prácticas de arquitectura sin sobrecomplicar prematuramente el
> proyecto.

------------------------------------------------------------------------

# 14. Estado resumido

``` text
Infraestructura Docker          ██████████  completa
RabbitMQ async flow             ██████████  completo
Robustez básica RabbitMQ        ██████████  completa para startup
Capa PostgreSQL                 ██████████  completa e integrada
Autenticación/autorización      ░░░░░░░░░░  siguiente milestone
Documentos + MinIO              ██████████  completo y verificado
Pipeline OKF                    ░░░░░░░░░░  pendiente
Confiabilidad avanzada          ████░░░░░░  prefetch/ACK listos; faltan retry/DLQ/idempotencia
Frontend funcional              ░░░░░░░░░░  pendiente
Observabilidad/pruebas finales  ░░░░░░░░░░  pendiente
```

**Siguiente acción:** iniciar el Milestone 4 --- autenticación y
autorización, manteniendo el desarrollo incremental y las pruebas
verificables.
