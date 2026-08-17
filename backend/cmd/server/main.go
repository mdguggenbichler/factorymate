package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"factorymate/internal/api"
	"factorymate/internal/auth"
	"factorymate/internal/db"
	"factorymate/internal/frm"
	"factorymate/internal/notify"
	"factorymate/internal/poller"

	"github.com/go-chi/chi/v5"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(db.DefaultPath())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	if err := db.Init(ctx, database, db.SeedConfigFromEnv()); err != nil {
		log.Fatalf("database init: %v", err)
	}

	phases, err := poller.LoadElevatorPhases(poller.DefaultElevatorPhasesPath())
	if err != nil {
		log.Fatalf("load elevator phases: %v", err)
	}

	go runPoller(ctx, database, phases)

	authSvc := auth.NewService(database)
	go authSvc.StartCleanupJob(ctx)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Get("/healthz", healthz)
	r.Route("/api", func(r chi.Router) {
		api.NewHandler(database, authSvc).Mount(r)
	})

	addr := ":" + port
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func runPoller(ctx context.Context, database *sql.DB, phases *poller.ElevatorPhases) {
	fetcher := &settingsFetcher{db: database}
	dispatcher := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": notify.NewDiscordProvider(),
	})
	onEvent := func(ctx context.Context, ev poller.Event) error {
		return dispatcher.HandleEvent(ctx, ev.MessageTypeKey, ev.Variables)
	}
	p := poller.New(database, fetcher, phases, onEvent)
	p.Run(ctx)
}

type settingsFetcher struct {
	db *sql.DB
}

func (f *settingsFetcher) GetFast(ctx context.Context) frm.FastPollResult {
	client, err := poller.FRMClientFromSettings(ctx, f.db)
	if err != nil {
		return frm.FastPollResult{Errors: map[string]error{"config": err}}
	}
	return client.GetFast(ctx)
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
