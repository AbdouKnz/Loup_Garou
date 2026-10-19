// Command server lance le serveur HTTP du jeu Loup-Garou.
//
// Il expose l'API JSON, la passerelle WebSocket (à partir de l'étape M1) et,
// en production, l'interface web construite dans Front/dist.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abdou/loupgarou/internal/config"
	"github.com/abdou/loupgarou/internal/httpapi"
)

func main() {
	// Les réglages locaux (`.env`, non versionné) sont chargés avant la
	// configuration : absents, le serveur démarre comme avant.
	if err := config.LoadEnvFile(config.DefaultEnvFile); err != nil {
		log.Printf("fichier %s illisible : %v", config.DefaultEnvFile, err)
	}

	cfg := config.Load()

	api := httpapi.NewRouter(cfg)
	defer api.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Println("arrêt demandé, fermeture des connexions…")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("arrêt propre impossible (%v), fermeture forcée", err)
			_ = srv.Close()
		}
	}()

	log.Printf("serveur Loup-Garou démarré sur http://localhost:%s", cfg.Port)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("erreur fatale : %v", err)
	}

	log.Println("serveur arrêté")
}
