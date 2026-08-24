# Documentos de prueba

Cada archivo ejercita una configuración distinta del conversor. La tabla es el
resultado esperado: si alguno cambia, hay una regresión.

| Archivo | Qué prueba | Conceptos | Clasificación |
| --- | --- | --- | --- |
| `01-breve.txt` | Documento breve sin divisiones (texto plano). | 1 | `valid` |
| `02-estructurado.md` | Varias unidades separadas por H1, en orden. | 3 | `valid` |
| `03-manual-tecnico.md` | H1 como título + secciones H2 + bloques de código. | 4 | `valid` |
| `04-preambulo.md` | Texto antes del primer encabezado: no debe perderse. | 3 | `valid` |
| `05-titulo-con-enlace.md` | Título con sintaxis de enlace Markdown. | 2 | `valid` |
| `06-seccion-vacia.md` | Sección con título pero sin cuerpo. | 3 | `valid_with_warnings` |
| `07-vacio.md` | Documento sin contenido: solo espacios y saltos. | --- | el Job falla |
| `08-vacio.txt` | Lo mismo en texto plano. | --- | el Job falla |

Detalles de lo que debe observarse al abrir el bundle:

- **01** produce `index.md`, `log.md` y un único `concept-01.md`. No emite
  advertencias por tener una sola unidad.
- **02** enlaza `concept-01/02/03.md` desde `index.md` en el orden del
  documento original.
- **03** se segmenta por **H2**, no por H1: el H1 es el título del documento.
  Los comentarios `#` dentro de los bloques ` ```bash ` no parten el documento
  y el código llega intacto a `concept-02.md` y `concept-03.md`.
- **04** convierte el resumen previo al primer encabezado en `concept-01.md`.
- **05** genera la etiqueta `[Ver la especificación](concept-01.md)` en
  `index.md`. Sin el saneamiento del título, el enlace quedaría anidado, la
  resolución de enlaces fallaría y el bundle sería `invalid`.
- **06** publica el bundle igualmente y registra
  `el concepto concept-02.md no tiene contenido` como advertencia.
- **07** y **08** son el único caso de fallo que no necesita inyección de
  fallos: el conversor los rechaza con `el documento está vacío`, el Job
  termina en `failed` y **no se llega a crear ningún bundle**. Sirven para
  probar el camino de error desde la interfaz sin tocar la configuración del
  worker. No confundirlos con el bundle rechazado por validación
  (`OKF_FAULT_INJECTION=drop-index`), que sí crea la fila del bundle con sus
  errores: son dos fallos distintos y la vista de detalle los explica
  distinto.

`file.md` es un fragmento de PowerShell heredado de una prueba manual anterior,
no un documento de prueba.

## Cómo ejecutarlos

Ver la sección *Manual verification with curl* del `README.md` en la raíz del
repositorio.
