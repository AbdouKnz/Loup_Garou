package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// newSPAHandler sert l'interface web construite (Front/dist).
//
// Si la ressource demandée existe, elle est servie telle quelle ; sinon on
// renvoie index.html afin que le routeur côté React prenne le relais. Quand
// l'interface n'a pas été construite, on renvoie une erreur JSON explicite
// plutôt qu'une page vide, pour ne pas se tromper de port en développement.
func newSPAHandler(webDist string) http.Handler {
	fileServer := http.FileServer(http.Dir(webDist))
	index := filepath.Join(webDist, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "méthode non autorisée",
			})
			return
		}

		// Chemin relatif sûr : path.Clean supprime toute traversée ("../").
		rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		target := filepath.Join(webDist, filepath.FromSlash(rel))

		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		if _, err := os.Stat(index); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "interface web non construite : lancez `npm run build` dans Front/",
			})
			return
		}

		http.ServeFile(w, r, index)
	})
}
