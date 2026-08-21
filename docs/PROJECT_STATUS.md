# Estado del proyecto --- Plataforma OKF Cloud

**Fecha de corte:** 20 de agosto de 2026\
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
2.  genera UUID;
3.  construye `JobMessage`;
4.  publica el mensaje en `document_jobs`;
5.  responde `202 Accepted`.

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

**Estado: IMPLEMENTADA EN CÓDIGO Y MERGEADA. REQUIERE
INTEGRACIÓN/VERIFICACIÓN END-TO-END CON EL FLUJO DEL MILESTONE 2.**

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

**PARCIALMENTE/AMPLIAMENTE IMPLEMENTADO; SIGUIENTE OBJETIVO: VERIFICAR E
INTEGRAR.**

Ya existen:

-   conexión PostgreSQL;
-   migraciones;
-   tabla de control de migraciones;
-   advisory lock;
-   modelos de dominio relacionados;
-   repositorio de usuarios;
-   repositorio de documentos;
-   repositorio de jobs;
-   repositorio de bundles;
-   idempotency key en jobs.

Lo que falta no es volver a construir esta capa, sino:

1.  verificar las migraciones desde una base vacía;
2.  inspeccionar el esquema final creado por `001_init.sql`;
3.  probar CRUD/repositories;
4.  conectar el endpoint de creación de jobs con `CreateJob`;
5.  hacer que el `JobMessage` transporte el ID persistido en PostgreSQL;
6.  hacer que el worker actualice el estado del job;
7.  verificar el ciclo completo de estados.

Flujo objetivo inmediato:

``` text
POST job
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
                     ├── procesa
                     │
                     └── PostgreSQL → COMPLETED
```

En caso de error:

``` text
PROCESSING
    ↓
 FAILED
 + error_message
```

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

**PENDIENTE.**

Objetivos:

-   configurar cliente MinIO;
-   bucket(s);
-   upload real;
-   guardar archivo original en object storage;
-   crear metadata `Document` en PostgreSQL;
-   usar `storage_key`;
-   límites/tipos de archivo;
-   descarga controlada.

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

## 9.1 RabbitMQ prefetch / QoS

Actualmente debe revisarse/configurarse `prefetch`.

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

## 9.11 Worker + PostgreSQL

El worker todavía debe integrarse con la base de datos.

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

No iniciar un "Milestone 3 desde cero".

El siguiente bloque de trabajo debe llamarse:

## Milestone 3 --- Verificación e integración de persistencia

Orden recomendado:

### Paso 3.1 --- Verificar migraciones

Desde una base vacía:

``` text
docker compose down -v
docker compose up -d --build
```

Confirmar que:

-   PostgreSQL arranca;
-   API conecta;
-   `db.Migrate()` termina;
-   `schema_migrations` contiene `001_init.sql`;
-   las tablas esperadas existen.

**Advertencia:** `down -v` elimina datos locales. Solo hacerlo cuando el
entorno pueda resetearse.

### Paso 3.2 --- Entender el esquema

Revisar juntos `001_init.sql` y los structs de `internal/domain`.

Documentar:

-   PK;
-   FK;
-   índices;
-   constraints;
-   estados;
-   relaciones User → Document → Job → Bundle.

### Paso 3.3 --- Probar repositories

Crear pruebas o endpoints temporales controlados para verificar:

-   users;
-   documents;
-   jobs;
-   bundles.

### Paso 3.4 --- Integrar Job real con RabbitMQ

Reemplazar el UUID aislado del milestone 2 por un registro real de
PostgreSQL.

### Paso 3.5 --- Worker actualiza estado

``` text
QUEUED
 ↓
PROCESSING
 ↓
COMPLETED
```

y luego probar `FAILED`.

### Paso 3.6 --- Configurar prefetch

Antes o durante la prueba con múltiples workers, configurar QoS/prefetch
y comprobar distribución.

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
> Milestones 1 y 2 están terminados. El PR #1 ya agregó una capa
> PostgreSQL con migraciones y repositories, por lo que el siguiente
> trabajo es verificarla e integrarla con el flujo RabbitMQ existente.
> Quiero avanzar paso a paso y verificar cada cambio. No tengo mucha
> experiencia con Go, así que explica los conceptos del lenguaje cuando
> aparezcan. Mantén buenas prácticas de arquitectura sin sobrecomplicar
> prematuramente el proyecto.

------------------------------------------------------------------------

# 14. Estado resumido

``` text
Infraestructura Docker          ██████████  completa
RabbitMQ async flow             ██████████  completo
Robustez básica RabbitMQ        ██████████  completa para startup
Capa PostgreSQL                 ████████░░  implementada; falta verificación/integración
Autenticación                   ░░░░░░░░░░  pendiente
MinIO funcional                 ░░░░░░░░░░  pendiente
Pipeline OKF                    ░░░░░░░░░░  pendiente
Confiabilidad avanzada          ██░░░░░░░░  parcialmente preparada
Frontend funcional              ░░░░░░░░░░  pendiente
Observabilidad/pruebas finales  ░░░░░░░░░░  pendiente
```

**Siguiente acción:** verificar e integrar la capa PostgreSQL ya
mergeada antes de implementar nuevas funcionalidades.
