# Manual de despliegue

Este manual describe el despliegue de la plataforma OKF.

El H1 es el título del documento, así que la segmentación debe bajar a H2.

## Requisitos

Se necesita Docker y Docker Compose.

```bash
# Este comentario NO debe partir el documento
docker compose pull
```

## Puesta en marcha

Un solo comando levanta todo el sistema:

```bash
# Tampoco este
docker compose up -d --build
```

## Verificación

Consultar el estado de los servicios y el healthcheck de la API.
