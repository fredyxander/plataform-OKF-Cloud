// Package migrations expone los archivos .sql de este directorio
// como un sistema de archivos embebido dentro del binario.
//
// Al quedar compilados dentro del ejecutable, el contenedor final
// no necesita copiar la carpeta migrations.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
