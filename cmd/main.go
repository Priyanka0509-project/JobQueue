package main

// @title Job Queue API
// @version 1.0
// @description This is a job processing queue API
// @host localhost:8080
// @BasePath /

import (
	"fmt"
	storage "jobQueue/cloudinary"
	"jobQueue/controller"
	_ "jobQueue/docs"
	"jobQueue/handler"
	"jobQueue/queue"
	ratelimiter "jobQueue/ratelimiter"
	"jobQueue/repository"
	"jobQueue/service"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func main() {
	fmt.Println("==============================================")
	fmt.Println("  Job Queue V4 Starting...")
	fmt.Println("==============================================")

	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	cloudClient, err := storage.NewCloudinaryClient()
	if err != nil {
		fmt.Printf("[Boot] FATAL: Cloudinary config error: %v\n", err)
		os.Exit(1)
	}

	repo, err := repository.NewJobRepository()
	if err != nil {
		fmt.Printf("[Boot] FATAL: Cannot connect to MongoDB: %v\n", err)
		os.Exit(1)

	}
	fmt.Println("[Boot] Repository ready")

	jobQueue := queue.NewQueue()
	fmt.Println("queue")

	jobService := service.NewJobService(repo, jobQueue, cloudClient)
	fmt.Println("Service")

	jobHandler := handler.NewJobHandler(jobQueue, jobService)
	fmt.Println("Handler")

	go jobHandler.StartPool(3)
	fmt.Println("goroutines started in background")

	rateLimiter := ratelimiter.NewRateLimiter(2, time.Minute)
	fmt.Println("Rate limiter")

	mux := http.NewServeMux()
	c := controller.NewJobController(jobService, mux)

	mux.HandleFunc("/upload", rateLimiter.Ratelimiter(c.Upload))
	mux.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	fmt.Println("[Boot] Routes:")
	fmt.Println("       POST  /upload")
	fmt.Println("       GET   /job/{id}")
	fmt.Println("       GET   /jobs")
	fmt.Println("       GET   /health")
	fmt.Println("       GET   /swagger/index.html")

	addr := ":8080"
	fmt.Printf("\n[Boot] Server running → http://localhost%s\n", addr)
	fmt.Println("==============================================")

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("[Boot] Server error: %v\n", err)
		os.Exit(1)
	}
}
