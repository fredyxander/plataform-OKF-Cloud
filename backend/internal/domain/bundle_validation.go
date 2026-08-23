package domain

import (
	"fmt"
	"strings"
)

// BundleValidationStatus clasifica el resultado de validar un bundle
// antes de publicarlo.
//
// La rúbrica exige distinguir tres resultados y no solo "válido/inválido":
//
//	valid                -> estructura mínima correcta y sin observaciones
//	valid_with_warnings  -> publicable, pero con observaciones registradas
//	invalid              -> no se publica ni se habilita su descarga
type BundleValidationStatus string

const (
	BundleValid             BundleValidationStatus = "valid"
	BundleValidWithWarnings BundleValidationStatus = "valid_with_warnings"
	BundleInvalid           BundleValidationStatus = "invalid"
)

// BundleValidation es el resultado completo de la validación.
//
// Se conserva junto con la metadata del bundle para que el estado del
// Job pueda explicar por qué un bundle se publicó, se publicó con
// advertencias o fue rechazado.
type BundleValidation struct {
	Status   BundleValidationStatus `json:"status"`
	Warnings []string               `json:"warnings"`
	Errors   []string               `json:"errors"`
}

// IsPublishable indica si el bundle puede almacenarse y descargarse.
//
// Un bundle con advertencias sí es publicable: las advertencias
// describen observaciones, no incumplimientos de la estructura mínima.
func (v BundleValidation) IsPublishable() bool {
	return v.Status != BundleInvalid
}

// Err devuelve el error asociado a un bundle inválido, o nil si el
// bundle es publicable. Se usa para persistir `error_message` del Job.
func (v BundleValidation) Err() error {
	if len(v.Errors) == 0 {
		return nil
	}

	return fmt.Errorf(
		"bundle validation failed: %s",
		strings.Join(v.Errors, "; "),
	)
}
