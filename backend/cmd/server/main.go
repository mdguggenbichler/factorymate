package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"factorymate/internal/api"
	"factorymate/internal/auth"
	"factorymate/internal/connection"
	"factorymate/internal/db"
	"factorymate/internal/discord"
	"factorymate/internal/frm"
	"factorymate/internal/health"
	"factorymate/internal/mods"
	"factorymate/internal/notify"
	"factorymate/internal/poller"
	"factorymate/internal/registration"

	"github.com/go-chi/chi/v5"
)

var appVersion = "dev"

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

	authSvc := auth.NewService(database)
	regSvc := registration.NewService(database, authSvc)
	modsSvc := mods.NewService(database, func(ctx context.Context) (*frm.Client, error) {
		return poller.FRMClientFromSettings(ctx, database)
	})

	bot, err := discord.NewBot(database, regSvc, nil, modsSvc)
	if err != nil {
		log.Fatalf("discord bot: %v", err)
	}

	connSvc := connection.NewService(database, bot)
	bot.SetConnection(connSvc)

	notify.SendEnabled = func(ctx context.Context) (bool, error) {
		return discord.BotEnabled(ctx, database)
	}

	if err := bot.Start(ctx); err != nil {
		log.Printf("discord bot start: %v — dashboard and FRM polling will continue without bot features", err)
	}

	go runPoller(ctx, database, phases, bot)
	go runSlowPoller(ctx, database)
	go authSvc.StartCleanupJob(ctx)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Get("/healthz", health.Handler(appVersion))
	r.Route("/api", func(r chi.Router) {
		api.NewHandler(database, authSvc, bot, regSvc, connSvc, modsSvc).Mount(r)
	})

	addr := ":" + port
	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	bot.Stop()
}

func runSlowPoller(ctx context.Context, database *sql.DB) {
	fetcher := &settingsSlowFetcher{db: database}
	sp := poller.NewSlowPoller(database, fetcher)
	sp.Run(ctx)
}

type settingsSlowFetcher struct {
	db *sql.DB
}

func (f *settingsSlowFetcher) GetSlow(ctx context.Context) frm.SlowPollResult {
	client, err := poller.FRMClientFromSettings(ctx, f.db)
	if err != nil {
		return frm.SlowPollResult{Errors: map[string]error{"config": err}}
	}
	return client.GetSlow(ctx)
}

func runPoller(ctx context.Context, database *sql.DB, phases *poller.ElevatorPhases, bot *discord.Bot) {
	fetcher := &settingsFetcher{db: database}
	provider := notify.NewDiscordProviderWithSessionFn(func() notify.DiscordSession {
		if bot == nil {
			return nil
		}
		return bot.Session()
	})
	dispatcher := notify.NewDispatcher(database, map[string]notify.Provider{
		"discord": provider,
	})
	poller.OnPlayersAutoLinked = func(ctx context.Context, links []auth.ResolvedPlayerLink) error {
		autoLinks := make([]notify.PlayerAutoLink, 0, len(links))
		for _, link := range links {
			autoLinks = append(autoLinks, notify.PlayerAutoLink{
				ExternalUserID: link.ExternalUserID,
				PlayerName:     link.PlayerName,
			})
		}
		return dispatcher.NotifyPlayerAutoLinked(ctx, autoLinks)
	}
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
