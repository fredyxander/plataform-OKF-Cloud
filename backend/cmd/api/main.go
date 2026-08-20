package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/fredyxander/okf-platform/backend/internal/config"
	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
	"github.com/fredyxander/okf-platform/backend/internal/queue"
)

func main() {
	cfg := config.Load()

	rabbitMQ, err := queue.NewRabbitMQWithRetry(
		cfg.RabbitMQURL,
		10,
		3*time.Second,
	)

	if err != nil {
		log.Fatalf("could not initialize RabbitMQ: %v", err)
	}

	defer rabbitMQ.Close()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := database.New(dsn)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Database connected")

	if err := db.Migrate(); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	fmt.Println("Database schema up to date")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	http.HandleFunc("/jobs/test", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jobID := uuid.NewString()

		job := domain.JobMessage{
			JobID: jobID,
		}

		if err := rabbitMQ.PublishJob(r.Context(), job); err != nil {
			http.Error(
				w,
				"could not publish job",
				http.StatusInternalServerError,
			)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)

		json.NewEncoder(w).Encode(map[string]string{
			"jobId":  jobID,
			"status": "queued",
		})
	})

	fmt.Println("API listening on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
