package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/config"
	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
	"github.com/fredyxander/okf-platform/backend/internal/queue"
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

		// TEMPORAL: simular un error de procesamiento.
		processingErr := fmt.Errorf("simulated processing error")

		errMsg := processingErr.Error()

		if err := db.UpdateJobStatus(
			job.JobID,
			domain.JobStatusFailed,
			&errMsg,
		); err != nil {
			log.Printf("could not update job %s to failed: %v", job.JobID, err)

			if nackErr := message.Nack(false, false); nackErr != nil {
				log.Printf("could not nack job %s: %v", job.JobID, nackErr)
			}

			continue
		}

		log.Printf("job failed: %s: %v", job.JobID, processingErr)

		if ackErr := message.Ack(false); ackErr != nil {
			log.Printf("could not acknowledge failed job %s: %v", job.JobID, ackErr)
		}

		continue

		// TEMPORAL:
		//
		// Simula una conversión documental que tarda 10 segundos.
		// Esto nos permite detener la API y comprobar
		// que el worker sigue trabajando independientemente.
		time.Sleep(10 * time.Second)

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

		//log en terminal de completed
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