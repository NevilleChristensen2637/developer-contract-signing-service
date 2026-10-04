package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/developer-contract-signing-service/internal/contractsigner"
	"github.com/example/developer-contract-signing-service/internal/infrai"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}

	client := infrai.NewClient(apiKey, &http.Client{Timeout: 30 * time.Second})
	service := contractsigner.NewService(client)
	handler := contractsigner.NewHTTPHandler(service)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("contract signer listening on http://127.0.0.1:8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}

	_ = srv.Shutdown(context.Background())
}
