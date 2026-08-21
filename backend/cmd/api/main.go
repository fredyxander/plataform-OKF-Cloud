package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/fredyxander/okf-platform/backend/internal/application"
	"github.com/fredyxander/okf-platform/backend/internal/auth"
	"github.com/fredyxander/okf-platform/backend/internal/config"
	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
	httpapi "github.com/fredyxander/okf-platform/backend/internal/http"
	"github.com/fredyxander/okf-platform/backend/internal/queue"
	"github.com/fredyxander/okf-platform/backend/internal/storage"
)

func main() {
	//rabbitmq
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

	//postgres config
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

	//migrations
	if err := db.Migrate(); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	fmt.Println("Database schema up to date")

	//minIO y bucket config
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

	//servicios y handlers
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is not set")
	}

	tokenManager := auth.NewTokenManager(
		jwtSecret,
		24*time.Hour,
	)

	authService := application.NewAuthService(
		db,
		tokenManager,
	)
	authHandler := httpapi.NewAuthHandler(authService)

	documentService := application.NewDocumentService(
		db,
		minioStorage,
	)

	documentHandler := httpapi.NewDocumentHandler(
		documentService,
	)

	//endpoints
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	http.HandleFunc("/auth/register", authHandler.Register)
	http.HandleFunc("/auth/login", authHandler.Login)

	// ENDPOINTS PROTEGIDOS
	http.HandleFunc(
		"/documents",
		httpapi.AuthMiddleware(tokenManager, documentHandler.Upload),
	)

	http.HandleFunc(
		"GET /documents/{id}/download",
		httpapi.AuthMiddleware(tokenManager, documentHandler.Download),
	)

	http.HandleFunc("/jobs/test",
		httpapi.AuthMiddleware(
			tokenManager,
			func(w http.ResponseWriter, r *http.Request) {

				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}

				var req struct {
					DocumentID string `json:"documentId"`
				}

				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, "invalid request body", http.StatusBadRequest)
					return
				}

				ownerID, ok := httpapi.UserIDFromContext(r.Context())
				if !ok {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}

				if req.DocumentID == "" {
					http.Error(w, "documentId is required", http.StatusBadRequest)
					return
				}

				_, err = db.GetDocumentByID(req.DocumentID, ownerID)
				if err != nil {
					http.Error(w, "document not found", http.StatusNotFound)
					return
				}
				persistedJob, err := db.CreateJob(
					req.DocumentID,
					ownerID,
					uuid.NewString(),
				)
				if err != nil {
					http.Error(w, "could not create job", http.StatusInternalServerError)
					return
				}

				job := domain.JobMessage{
					JobID: persistedJob.ID,
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
					"jobId":  persistedJob.ID,
					"status": "queued",
				})
			}),
		)

	//http server
	fmt.Println("API listening on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
