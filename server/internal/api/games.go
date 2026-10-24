package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cracked-saves/server/internal/model"
	"github.com/cracked-saves/server/internal/store"
)

func (a *API) listGames(w http.ResponseWriter, r *http.Request) {
	games, err := a.store.ListGames()
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, games)
}

func (a *API) getGame(w http.ResponseWriter, r *http.Request) {
	game, err := a.store.GetGame(chi.URLParam(r, "id"))
	if err != nil {
		a.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, game)
}

type createGameRequest struct {
	Name        string `json:"name"`
	ExePattern  string `json:"exe_pattern"`
	SaveDirHint string `json:"save_dir_hint"`
	Notes       string `json:"notes"`
}

func (a *API) createGame(w http.ResponseWriter, r *http.Request) {
	var req createGameRequest
	if !a.decodeBody(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		a.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	game := &model.Game{
		ID:          uuid.NewString(),
		Name:        req.Name,
		ExePattern:  strings.TrimSpace(req.ExePattern),
		SaveDirHint: strings.TrimSpace(req.SaveDirHint),
		Notes:       req.Notes,
	}

	err := a.store.CreateGame(game)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateName) {
			a.writeError(w, http.StatusConflict, "a game with this name already exists")
			return
		}
		a.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, game)
}

type patchGameRequest struct {
	Name        *string `json:"name"`
	ExePattern  *string `json:"exe_pattern"`
	SaveDirHint *string `json:"save_dir_hint"`
	Notes       *string `json:"notes"`
}

func (a *API) patchGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req patchGameRequest
	if !a.decodeBody(w, r, &req) {
		return
	}

	game, err := a.store.UpdateGame(id, func(g *model.Game) error {
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				return errors.New("name cannot be empty")
			}
			g.Name = name
		}
		if req.ExePattern != nil {
			g.ExePattern = strings.TrimSpace(*req.ExePattern)
		}
		if req.SaveDirHint != nil {
			g.SaveDirHint = strings.TrimSpace(*req.SaveDirHint)
		}
		if req.Notes != nil {
			g.Notes = *req.Notes
		}
		return nil
	})
	if err != nil {
		a.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, game)
}

func (a *API) deleteGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := a.store.DeleteGame(id); err != nil {
		a.handleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleStoreError(w http.ResponseWriter, err error) {
	status := statusForStoreError(err)
	if status == http.StatusInternalServerError {
		a.logger.Error("store error", "error", err.Error())
	}
	a.writeError(w, status, err.Error())
}
