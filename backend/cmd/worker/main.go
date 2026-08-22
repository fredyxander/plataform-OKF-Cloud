package main

import (
	"bytes"
	"context"
	"encoding/json"
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
				"could not get document %s from MinIO: %v",
				document.ID,
				err,
			)

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		content, err := io.ReadAll(object)
		closeErr := object.Close()

		if err != nil {
			log.Printf(
				"could not read document %s: %v",
				document.ID,
				err,
			)

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
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

		// Actualiza estado en postgres a processing del job
		if err := db.UpdateJobStatus(
			job.JobID,
			domain.JobStatusProcessing,
			nil,
		); err != nil {
			log.Printf("could not update job %s to processing: %v", job.JobID, err)

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		// TEMPORAL:
		//
		// Simula una conversión documental que tarda 10 segundos.
		// Esto nos permite detener la API y comprobar
		// que el worker sigue trabajando independientemente.
		// time.Sleep(10 * time.Second)
		concepts, err := okf.Convert(
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
			len(concepts),
		)

		bundle, err := okf.BuildBundle(
			document.Filename,
			document.Format,
			concepts,
		)
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

		if err := okf.ValidateBundle(bundle); err != nil {
			errMsg := err.Error()

			log.Printf(
				"bundle validation failed for job %s: %v",
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
			"bundle built and validated for job %s: %d files, %d concepts",
			persistedJob.ID,
			len(bundle.Files),
			bundle.ConceptCount,
		)

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

		bundlePrefix := fmt.Sprintf(
			"bundles/%s/%s",
			persistedJob.OwnerID,
			persistedJob.ID,
		)

		bundleUploadFailed := false

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
		}

		if bundleUploadFailed {
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

		log.Printf(
			"bundle zip stored in MinIO for job %s: key=%s",
			persistedJob.ID,
			bundleZIPKey,
		)

		persistedBundle, err := db.CreateBundle(
			persistedJob.ID,
			persistedJob.OwnerID,
			bundleZIPKey,
			true,
			bundle.ConceptCount,
		)
		if err != nil {
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
			"bundle metadata persisted: bundle_id=%s job_id=%s",
			persistedBundle.ID,
			persistedBundle.JobID,
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
