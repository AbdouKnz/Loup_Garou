package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// writeJSON sérialise payload en JSON et l'écrit avec le code HTTP fourni.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("sérialisation JSON impossible : %v", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"erreur interne"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Printf("écriture de la réponse impossible : %v", err)
	}
}

// accessLog journalise chaque requête : méthode, chemin, statut, durée.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		log.Printf("%s %s -> %d (%s)",
			r.Method,
			r.URL.Path,
			ww.Status(),
			time.Since(started).Round(time.Millisecond),
		)
	})
}
