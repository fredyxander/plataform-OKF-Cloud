-- Clasificación del resultado de validación del bundle.
--
-- is_valid se conserva como respuesta rápida a "¿se puede descargar?",
-- mientras que validation_status distingue los tres resultados que
-- exige la rúbrica y las columnas de detalle explican por qué.

ALTER TABLE bundles
    ADD COLUMN IF NOT EXISTS validation_status TEXT NOT NULL DEFAULT 'valid';

ALTER TABLE bundles
    ADD COLUMN IF NOT EXISTS validation_warnings TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE bundles
    ADD COLUMN IF NOT EXISTS validation_errors TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE bundles
    DROP CONSTRAINT IF EXISTS chk_bundle_validation_status;

ALTER TABLE bundles
    ADD CONSTRAINT chk_bundle_validation_status
    CHECK (validation_status IN ('valid', 'valid_with_warnings', 'invalid'));
