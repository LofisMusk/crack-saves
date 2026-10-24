package api

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/cracked-saves/server/internal/config"
	"github.com/cracked-saves/server/internal/store"
)

// API bundles the HTTP handler together with its dependencies.
type API struct {
	store  *store.Store
	cfg    *config.Config
	logger *slog.Logger
}

func New(db *sql.DB, cfg *config.Config, logger *slog.Logger) http.Handler {
	api := &API{store: store.New(db), cfg: cfg, logger: logger}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}))
	r.Use(recoverer(logger))

	r.Get("/api/health", api.handleHealth)

	r.Route("/api", func(r chi.Router) {
		if cfg.APIToken != "" {
			r.Use(api.requireAuth)
		}
		r.Get("/games", api.listGames)
		r.Post("/games", api.createGame)
		r.Get("/games/{id}", api.getGame)
		r.Patch("/games/{id}", api.patchGame)
		r.Delete("/games/{id}", api.deleteGame)

		r.Get("/games/{id}/saves", api.listSaves)
		r.Put("/games/{id}/saves/{name}", api.uploadSave)
		r.Get("/games/{id}/saves/{name}", api.downloadSave)
	})

	return r
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			a.writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		token := strings.TrimPrefix(header, prefix)
		if subtle.ConstantTimeCompare([]byte(token), []byte(a.cfg.APIToken)) != 1 {
			a.writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func (a *API) writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (a *API) decodeBody(w http.ResponseWriter, r *http.Request, out any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(out); err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic", "recover", fmt.Sprint(rec), "path", r.URL.Path)
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// re-export model errors as HTTP responses
func statusForStoreError(err error) int {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrDuplicateName):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
