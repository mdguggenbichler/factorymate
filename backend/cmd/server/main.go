package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"factorymate/internal/db"

	"github.com/go-chi/chi/v5"
	_ "golang.org/x/crypto/bcrypt"
)

func main() {
	ctx := context.Background()

	database, err := db.Open(db.DefaultPath())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfigFromEnv()); err != nil {
		log.Fatalf("database init: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Get("/healthz", healthz)

	addr := ":" + port
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
