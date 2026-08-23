package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/config"
	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
	"github.com/fredyxander/okf-platform/backend/internal/okf"
	"github.com/fredyxander/okf-platform/backend/internal/queue"
	"github.com/fredyxander/okf-platform/backend/internal/storage"
)

const maxJobRetries = 3

// staleJobLease es el tiempo tras el cual un Job en processing se
// considera abandonado y otro worker puede reclamarlo.
const staleJobLease = 5 * time.Minute
var errJobRetryLimitReached = errors.New("job retry limit reached")

func retryJob(
	ctx context.Context,
	db *database.DB,
	rabbitMQ *queue.RabbitMQ,
	job domain.JobMessage,
	failure error,
) error {
	if job.Attempt >= maxJobRetries {
		return errJobRetryLimitReached
	}

	errorMessage := failure.Error()

	if err := db.RequeueJob(job.JobID, &errorMessage); err != nil {
		return fmt.Errorf("requeue job in database: %w", err)
	}

	if err := rabbitMQ.PublishJobRetry(ctx, job); err != nil {
		return fmt.Errorf("publish job retry: %w", err)
	}

	return nil
}

func failJobToDLQ(
	ctx context.Context,
	db *database.DB,
	rabbitMQ *queue.RabbitMQ,
	job domain.JobMessage,
	failure error,
) error {
	errorMessage := failure.Error()

	if err := db.UpdateJobStatus(
		job.JobID,
		domain.JobStatusFailed,
		&errorMessage,
	); err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}

	if err := rabbitMQ.PublishJobDLQ(ctx, job); err != nil {
		return fmt.Errorf("publish job to DLQ: %w", err)
	}

	return nil
}

// recordInvalidBundle deja constancia en PostgreSQL de que la
// validación rechazó el bundle. La fila se guarda sin published_at y
// con is_valid = false, de modo que el bundle queda registrado como
// evidencia de la validación pero nunca es descargable.
func recordInvalidBundle(
	db *database.DB,
	job *domain.Job,
	storageKey string,
	conceptCount int,
	validation domain.BundleValidation,
) {
	if _, err := db.CreateBundle(
		job.ID,
		job.OwnerID,
		storageKey,
		conceptCount,
		validation,
	); err != nil {
		log.Printf(
			"could not record invalid bundle for job %s: %v",
			job.ID,
			err,
		)
	}
}

func cleanupObjects(
	ctx context.Context,
	storage *storage.MinIO,
	keys []string,
) {
	for _, key := range keys {
		if err := storage.DeleteObject(ctx, key); err != nil {
			log.Printf(
				"could not clean up partial object %s: %v",
				key,
				err,
			)
		}
	}
}


func main() {
	// 1. Cargamos la configuración desde variables de entorno.
	cfg := config.Load()

	// 2. Creamos la conexión con RabbitMQ.
	rabbitMQ, err := queue.NewRabbitMQWithRetry(
		cfg.RabbitMQURL,
		10,
		3*time.Second,
	)
	if err != nil {
		log.Fatalf("could not initialize RabbitMQ: %v", err)
	}

	// 3. Cuando el programa termine, cerramos correctamente
	//    la conexión y el channel de RabbitMQ.
	defer rabbitMQ.Close()

	// 3.1. Conexión a base de datos postgres
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := database.New(dsn)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connected")

	// Conexión a MinIO.
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	minioBucket := os.Getenv("MINIO_BUCKET")

	if minioEndpoint == "" ||
		minioAccessKey == "" ||
		minioSecretKey == "" ||
		minioBucket == "" {
		log.Fatal("MinIO configuration is incomplete")
	}

	minioUseSSL, err := strconv.ParseBool(os.Getenv("MINIO_USE_SSL"))
	if err != nil {
		log.Fatalf("invalid MINIO_USE_SSL: %v", err)
	}

	minioStorage, err := storage.NewMinIO(
		minioEndpoint,
		minioAccessKey,
		minioSecretKey,
		minioUseSSL,
		minioBucket,
	)
	if err != nil {
		log.Fatalf("initialize MinIO client: %v", err)
	}

	if err := minioStorage.EnsureBucket(context.Background()); err != nil {
		log.Fatalf("ensure MinIO bucket: %v", err)
	}

	log.Println("MinIO connected and bucket ready")

	// Inyección de fallo para la sustentación. Desactivada salvo que
	// se defina explícitamente OKF_FAULT_INJECTION.
	bundleFault := os.Getenv("OKF_FAULT_INJECTION")
	if bundleFault != "" {
		log.Printf(
			"WARNING: bundle fault injection enabled: %s",
			bundleFault,
		)
	}

	// Retardo artificial del procesamiento, también solo para la
	// sustentación. El pipeline real tarda milisegundos, así que sin
	// esto no hay ventana para demostrar que la API respondió sin
	// esperar y que el trabajo continúa aunque el cliente cierre la
	// conexión.
	//
	// Debe mantenerse por debajo del lease de recuperación de Jobs
	// abandonados (5 minutos): un retardo mayor haría que otro worker
	// considerase el Job como abandonado y lo reclamase.
	processingDelay := time.Duration(0)

	if raw := os.Getenv("OKF_PROCESSING_DELAY"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			log.Fatalf("invalid OKF_PROCESSING_DELAY: %v", err)
		}

		if parsed >= staleJobLease {
			log.Fatalf(
				"OKF_PROCESSING_DELAY must be shorter than the %s stale job lease",
				staleJobLease,
			)
		}

		processingDelay = parsed

		log.Printf(
			"WARNING: artificial processing delay enabled: %s",
			processingDelay,
		)
	}

	// 4. Nos suscribimos a la cola de trabajos.
	messages, err := rabbitMQ.ConsumeJobs()
	if err != nil {
		log.Fatalf("could not consume jobs: %v", err)
	}

	log.Println("worker waiting for jobs")

	// 5. El worker permanece esperando mensajes.
	//
	// Cada vez que RabbitMQ entrega uno,
	// este ciclo procesa el mensaje.
	for message := range messages {

		// Esta variable contendrá el mensaje convertido
		// desde JSON hacia una estructura Go.
		var job domain.JobMessage

		// 6. Convertimos el JSON recibido a JobMessage.
		err := json.Unmarshal(message.Body, &job)

		if err != nil {
			log.Printf("invalid job message: %v", err)

			// Nack indica que el mensaje NO fue procesado.
			//
			// Primer false:
			// no aplicar el Nack a múltiples mensajes.
			//
			// Segundo false:
			// no volver a poner el mensaje inválido en la cola.
			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf(
					"could not reject invalid message: %v",
					nackErr,
				)
			}

			continue
		}

		// 7. Validamos que el mensaje tenga un jobId.
		//
		// Un JSON como:
		// {}
		//
		// es JSON válido, por lo que Unmarshal no falla.
		// Pero JobID quedaría vacío.
		if job.JobID == "" {
			log.Println("invalid job message: missing jobId")

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf(
					"could not reject job without id: %v",
					nackErr,
				)
			}

			continue
		}

		// 8. Aquí empieza conceptualmente el procesamiento.
		log.Printf("processing job: %s", job.JobID)

		persistedJob, err := db.GetJobByIDForProcessing(job.JobID)
		if err != nil {
			log.Printf("could not load job %s: %v", job.JobID, err)

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		log.Printf(
			"loaded job %s: document_id=%s owner_id=%s status=%s",
			persistedJob.ID,
			persistedJob.DocumentID,
			persistedJob.OwnerID,
			persistedJob.Status,
		)

		// Idempotencia ante redelivery:
		// si RabbitMQ vuelve a entregar un Job que ya terminó correctamente,
		// no ejecutamos nuevamente el pipeline.
		if persistedJob.Status == domain.JobStatusCompleted {
			log.Printf(
				"job %s already completed; acknowledging duplicate delivery",
				persistedJob.ID,
			)

			if ackErr := message.Ack(false); ackErr != nil {//job completo elimina mensaje duplicado de la cola -evitar redelivery.
				log.Printf(
					"could not acknowledge already completed job %s: %v",
					persistedJob.ID,
					ackErr,
				)
			}

			continue
		}

		// Un Job FAILED es terminal.
		// No debe volver a entrar al claim ni al pipeline.
		if persistedJob.Status == domain.JobStatusFailed {
			log.Printf(
				"job %s already failed; handling terminal delivery",
				persistedJob.ID,
			)

			if job.Attempt >= maxJobRetries {
				if err := rabbitMQ.PublishJobDLQ(
					context.Background(),
					job,
				); err != nil {
					log.Printf(
						"could not publish failed job %s to DLQ: %v",
						job.JobID,
						err,
					)

					if nackErr := message.Nack(false, true); nackErr != nil {
						log.Printf(
							"could not requeue failed job %s: %v",
							job.JobID,
							nackErr,
						)
					}

					continue
				}
			}

			if ackErr := message.Ack(false); ackErr != nil {
				log.Printf(
					"could not acknowledge already failed job %s: %v",
					job.JobID,
					ackErr,
				)
			}

			continue
		}


		// rechazo de mismo job entre dos workers, solo uno toma el job y lo procesa.
		staleBefore := time.Now().Add(-staleJobLease)

		claimedJob, err := db.ClaimJobForProcessing(
			job.JobID,
			staleBefore,
		)
		
		if err != nil {
			if errors.Is(err, database.ErrJobNotClaimable) {
				log.Printf(
					"job %s is temporarily not claimable; deferring delivery (attempt=%d)",
					job.JobID,
					job.Attempt,
				)

				if retryErr := rabbitMQ.PublishJobDeferred(
					context.Background(),
					job,
				); retryErr != nil {
					log.Printf(
						"could not publish retry for job %s: %v",
						job.JobID,
						retryErr,
					)

					if nackErr := message.Nack(false, true); nackErr != nil {
						log.Printf(
							"could not requeue job %s after retry publish failure: %v",
							job.JobID,
							nackErr,
						)
					}

					continue
				}

				if ackErr := message.Ack(false); ackErr != nil {
					log.Printf(
						"could not acknowledge job %s after scheduling retry: %v",
						job.JobID,
						ackErr,
					)
				}

				continue
			}

			log.Printf(
				"could not claim job %s for processing: %v",
				job.JobID,
				err,
			)

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf(
					"could not nack unclaimable job %s: %v",
					job.JobID,
					nackErr,
				)
			}

			continue
		}

		persistedJob = claimedJob

		log.Printf(
			"job %s claimed for processing",
			persistedJob.ID,
		)

		// Ventana artificial para poder observar el Job en processing.
		// En el pipeline real esta espera no existe.
		if processingDelay > 0 {
			log.Printf(
				"job %s: simulating a long conversion for %s",
				persistedJob.ID,
				processingDelay,
			)

			time.Sleep(processingDelay)
		}

		// Carga document para converion
		document, err := db.GetDocumentByID(
			persistedJob.DocumentID,
			persistedJob.OwnerID,
		)
		if err != nil {
			log.Printf(
				"could not load document %s for job %s: %v",
				persistedJob.DocumentID,
				persistedJob.ID,
				err,
			)

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		log.Printf(
			"loaded document %s: filename=%s format=%s storage_key=%s",
			document.ID,
			document.Filename,
			document.Format,
			document.StorageKey,
		)

		object, err := minioStorage.GetObject(
			context.Background(),
			document.StorageKey,
		)

		if err != nil {
			log.Printf(
				"could not get document %s from MinIO for job %s: %v",
				document.ID,
				job.JobID,
				err,
			)

			retryErr := retryJob(
				context.Background(),
				db,
				rabbitMQ,
				job,
				err,
			)

			if retryErr != nil {
				if errors.Is(retryErr, errJobRetryLimitReached) {
					log.Printf(
						"job %s exhausted retries at attempt=%d; moving to DLQ",
						job.JobID,
						job.Attempt,
					)

					if terminalErr := failJobToDLQ(
						context.Background(),
						db,
						rabbitMQ,
						job,
						err,
					); terminalErr != nil {
						log.Printf(
							"could not finalize failed job %s: %v",
							job.JobID,
							terminalErr,
						)

						if nackErr := message.Nack(false, true); nackErr != nil {
							log.Printf(
								"could not requeue job %s after terminal failure: %v",
								job.JobID,
								nackErr,
							)
						}

						continue
					}

					log.Printf(
						"job %s marked failed and moved to DLQ",
						job.JobID,
					)

					if ackErr := message.Ack(false); ackErr != nil {
						log.Printf(
							"could not acknowledge failed job %s: %v",
							job.JobID,
							ackErr,
						)
					}

					continue
				}

				log.Printf(
					"could not schedule retry for job %s: %v",
					job.JobID,
					retryErr,
				)

				if nackErr := message.Nack(false, true); nackErr != nil {
					log.Printf(
						"could not requeue current message for job %s: %v",
						job.JobID,
						nackErr,
					)
				}

				continue
			}

			log.Printf(
				"retry scheduled for job %s: next attempt=%d",
				job.JobID,
				job.Attempt+1,
			)

			if ackErr := message.Ack(false); ackErr != nil {
				log.Printf(
					"could not acknowledge job %s after scheduling retry: %v",
					job.JobID,
					ackErr,
				)
			}

			continue
		}

		content, err := io.ReadAll(object)
		closeErr := object.Close()

		if err != nil {
			log.Printf(
				"could not read document %s for job %s: %v",
				document.ID,
				job.JobID,
				err,
			)

			retryErr := retryJob(
				context.Background(),
				db,
				rabbitMQ,
				job,
				err,
			)

			if retryErr != nil {
				if errors.Is(retryErr, errJobRetryLimitReached) {
					log.Printf(
						"job %s exhausted retries at attempt=%d; moving to DLQ",
						job.JobID,
						job.Attempt,
					)

					if terminalErr := failJobToDLQ(
						context.Background(),
						db,
						rabbitMQ,
						job,
						err,
					); terminalErr != nil {
						log.Printf(
							"could not finalize failed job %s: %v",
							job.JobID,
							terminalErr,
						)

						if nackErr := message.Nack(false, true); nackErr != nil {
							log.Printf(
								"could not requeue job %s after terminal failure: %v",
								job.JobID,
								nackErr,
							)
						}

						continue
					}

					log.Printf(
						"job %s marked failed and moved to DLQ",
						job.JobID,
					)

					if ackErr := message.Ack(false); ackErr != nil {
						log.Printf(
							"could not acknowledge failed job %s: %v",
							job.JobID,
							ackErr,
						)
					}

					continue
				}

				log.Printf(
					"could not schedule retry for job %s: %v",
					job.JobID,
					retryErr,
				)

				if nackErr := message.Nack(false, true); nackErr != nil {
					log.Printf(
						"could not requeue current message for job %s: %v",
						job.JobID,
						nackErr,
					)
				}

				continue
			}

			log.Printf(
				"retry scheduled for job %s: next attempt=%d",
				job.JobID,
				job.Attempt+1,
			)

			if ackErr := message.Ack(false); ackErr != nil {
				log.Printf(
					"could not acknowledge job %s after scheduling retry: %v",
					job.JobID,
					ackErr,
				)
			}

			continue
		}

		if closeErr != nil {
			log.Printf(
				"could not close document %s: %v",
				document.ID,
				closeErr,
			)
		}

		log.Printf(
			"document %s loaded from MinIO: %d bytes",
			document.ID,
			len(content),
		)

		// TEMPORAL:
		//
		// Simula una conversión documental que tarda 10 segundos.
		// Esto nos permite detener la API y comprobar
		// que el worker sigue trabajando independientemente.
		// time.Sleep(10 * time.Second)
		conversion, err := okf.Convert(
			document.Filename,
			document.Format,
			content,
		)
		if err != nil {
			errMsg := err.Error()

			log.Printf(
				"could not convert document %s for job %s: %v",
				document.ID,
				persistedJob.ID,
				err,
			)

			if updateErr := db.UpdateJobStatus(
				job.JobID,
				domain.JobStatusFailed,
				&errMsg,
			); updateErr != nil {
				log.Printf(
					"could not update job %s to failed: %v",
					job.JobID,
					updateErr,
				)
			}

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		log.Printf(
			"document %s converted successfully: %d concepts",
			document.ID,
			len(conversion.Concepts),
		)

		bundle, err := okf.BuildBundle(conversion)
		if err != nil {
			errMsg := err.Error()

			log.Printf(
				"could not build bundle for job %s: %v",
				persistedJob.ID,
				err,
			)

			if updateErr := db.UpdateJobStatus(
				job.JobID,
				domain.JobStatusFailed,
				&errMsg,
			); updateErr != nil {
				log.Printf(
					"could not update job %s to failed: %v",
					job.JobID,
					updateErr,
				)
			}

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		if applied := okf.ApplyFault(bundle, bundleFault); applied != "" {
			log.Printf(
				"fault injection applied to bundle of job %s: %s",
				persistedJob.ID,
				applied,
			)
		}

		bundlePrefix := fmt.Sprintf(
			"bundles/%s/%s",
			persistedJob.OwnerID,
			persistedJob.ID,
		)

		// La validación ocurre antes de subir cualquier objeto: un
		// bundle inválido no llega nunca al object storage.
		validation := okf.ValidateBundle(bundle)

		if !validation.IsPublishable() {
			validationErr := validation.Err()
			errMsg := validationErr.Error()

			log.Printf(
				"bundle validation failed for job %s: %v",
				persistedJob.ID,
				validationErr,
			)

			recordInvalidBundle(
				db,
				persistedJob,
				bundlePrefix,
				bundle.ConceptCount,
				validation,
			)

			if updateErr := db.UpdateJobStatus(
				job.JobID,
				domain.JobStatusFailed,
				&errMsg,
			); updateErr != nil {
				log.Printf(
					"could not update job %s to failed: %v",
					job.JobID,
					updateErr,
				)
			}

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		log.Printf(
			"bundle built and validated for job %s: %d files, %d concepts, validation=%s",
			persistedJob.ID,
			len(bundle.Files),
			bundle.ConceptCount,
			validation.Status,
		)

		for _, warning := range validation.Warnings {
			log.Printf(
				"bundle warning for job %s: %s",
				persistedJob.ID,
				warning,
			)
		}

		// La trazabilidad de log.md se completa con el resultado de la
		// validación antes de empaquetar el bundle.
		okf.AppendValidationLog(bundle, validation)

		bundleZIP, err := okf.PackageBundle(bundle)
		if err != nil {
			errMsg := err.Error()

			log.Printf(
				"could not package bundle for job %s: %v",
				persistedJob.ID,
				err,
			)

			if updateErr := db.UpdateJobStatus(
				job.JobID,
				domain.JobStatusFailed,
				&errMsg,
			); updateErr != nil {
				log.Printf(
					"could not update job %s to failed: %v",
					job.JobID,
					updateErr,
				)
			}

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		bundleUploadFailed := false

		uploadedKeys := make([]string, 0, len(bundle.Files)+1)

		for _, file := range bundle.Files {
			objectKey := fmt.Sprintf(
				"%s/%s",
				bundlePrefix,
				file.Name,
			)

			if err := minioStorage.PutObject(
				context.Background(),
				objectKey,
				bytes.NewReader(file.Content),
				int64(len(file.Content)),
				"text/markdown",
			); err != nil {
				log.Printf(
					"could not store bundle file %s for job %s: %v",
					file.Name,
					persistedJob.ID,
					err,
				)

				bundleUploadFailed = true
				break
			}
			uploadedKeys = append(uploadedKeys, objectKey)
		}

		if bundleUploadFailed {
			cleanupObjects(
				context.Background(),
				minioStorage,
				uploadedKeys,
			)
			errMsg := "could not store bundle in object storage"

			if updateErr := db.UpdateJobStatus(
				job.JobID,
				domain.JobStatusFailed,
				&errMsg,
			); updateErr != nil {
				log.Printf(
					"could not update job %s to failed: %v",
					job.JobID,
					updateErr,
				)
			}

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		log.Printf(
			"bundle stored in MinIO for job %s: prefix=%s",
			persistedJob.ID,
			bundlePrefix,
		)

		bundleZIPKey := fmt.Sprintf(
			"%s/bundle.zip",
			bundlePrefix,
		)

		if err := minioStorage.PutObject(
			context.Background(),
			bundleZIPKey,
			bytes.NewReader(bundleZIP),
			int64(len(bundleZIP)),
			"application/zip",
		); err != nil {
			cleanupObjects(
				context.Background(),
				minioStorage,
				uploadedKeys,
			)
			errMsg := err.Error()

			log.Printf(
				"could not store bundle zip for job %s: %v",
				persistedJob.ID,
				err,
			)

			if updateErr := db.UpdateJobStatus(
				job.JobID,
				domain.JobStatusFailed,
				&errMsg,
			); updateErr != nil {
				log.Printf(
					"could not update job %s to failed: %v",
					job.JobID,
					updateErr,
				)
			}

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		uploadedKeys = append(uploadedKeys, bundleZIPKey)

		log.Printf(
			"bundle zip stored in MinIO for job %s: key=%s",
			persistedJob.ID,
			bundleZIPKey,
		)

		persistedBundle, err := db.CreateBundle(
			persistedJob.ID,
			persistedJob.OwnerID,
			bundleZIPKey,
			bundle.ConceptCount,
			validation,
		)
		if err != nil {
			cleanupObjects(
				context.Background(),
				minioStorage,
				uploadedKeys,
			)
			errMsg := err.Error()

			log.Printf(
				"could not persist bundle metadata for job %s: %v",
				persistedJob.ID,
				err,
			)

			if updateErr := db.UpdateJobStatus(
				job.JobID,
				domain.JobStatusFailed,
				&errMsg,
			); updateErr != nil {
				log.Printf(
					"could not update job %s to failed: %v",
					job.JobID,
					updateErr,
				)
			}

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		log.Printf(
			"bundle metadata persisted: bundle_id=%s job_id=%s validation=%s",
			persistedBundle.ID,
			persistedBundle.JobID,
			persistedBundle.Validation.Status,
		)

		// Actualiza estado a completed en postgres
		if err := db.UpdateJobStatus(
			job.JobID,
			domain.JobStatusCompleted,
			nil,
		); err != nil {
			log.Printf("could not update job %s to completed: %v", job.JobID, err)

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		//log en terminal de completed: significa que existe un bundle válido, almacenado y registrado.
		log.Printf("job completed: %s", job.JobID)

		// 9. Solo hacemos ACK después de terminar correctamente.
		//
		// Esto le dice a RabbitMQ:
		//
		// "Este mensaje ya fue procesado correctamente".
		if ackErr := message.Ack(false); ackErr != nil {
			log.Printf(
				"could not acknowledge job %s: %v",
				job.JobID,
				ackErr,
			)
		}
	}
}
