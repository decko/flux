package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/decko/flux/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := api.NewServer()

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("flux listening on %s", httpServer.Addr)
	log.Fatal(httpServer.ListenAndServe())
}
