package main

import (
	"log"
	"net/http"
	"os"

	"github.com/decko/flux/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := api.NewServer()

	addr := ":" + port
	log.Printf("flux listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv))
}
