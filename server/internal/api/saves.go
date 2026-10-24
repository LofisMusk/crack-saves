package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cracked-saves/server/internal/model"
)

// maxUploadSize is 2 GiB — plenty for even the most bloated save file.
const maxUploadSize = 2 << 30

func (a *API) savesDirForGame(gameID string) (string, error) {
	dir := filepath.Join(a.cfg.SavesDir(), gameID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create save dir: %w", err)
	}
	return dir, nil
}

func (a *API) listSaves(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")
	if _, err := a.store.GetGame(gameID); err != nil {
		a.handleStoreError(w, err)
		return
	}
	saves, err := a.store.ListSaves(gameID)
	if err != nil {
		a.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saves)
}

func (a *API) uploadSave(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")
	filename, ok := decodeFilenameParam(r)
	if !ok {
		a.writeError(w, http.StatusBadRequest, "invalid save filename")
		return
	}

	if _, err := a.store.GetGame(gameID); err != nil {
		a.handleStoreError(w, err)
		return
	}

	dir, err := a.savesDirForGame(gameID)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tmpPath := filepath.Join(dir, filename+".part")
	file, err := os.Create(tmpPath)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed creating temp file: "+err.Error())
		return
	}
	defer os.Remove(tmpPath) // no-op once renamed into place

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(r.Body, maxUploadSize)); err != nil {
		file.Close()
		a.writeError(w, http.StatusInternalServerError, "failed writing file: "+err.Error())
		return
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := file.Close(); err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	finalPath := filepath.Join(dir, filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed moving file: "+err.Error())
		return
	}

	save := &model.Save{
		GameID:    gameID,
		Filename:  filename,
		SHA256:    hex.EncodeToString(hasher.Sum(nil)),
		Size:      size,
		UpdatedAt: time.Now().Unix(),
	}
	if err := a.store.UpsertSave(save); err != nil {
		a.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, save)
}

func (a *API) downloadSave(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "id")
	filename, ok := decodeFilenameParam(r)
	if !ok {
		a.writeError(w, http.StatusBadRequest, "invalid save filename")
		return
	}

	save, err := a.store.GetSave(gameID, filename)
	if err != nil {
		a.handleStoreError(w, err)
		return
	}

	path := filepath.Join(a.cfg.SavesDir(), gameID, filename)
	file, err := os.Open(path)
	if err != nil {
		os.Remove(path) // metadata exists but file vanished — treat as missing
		a.writeError(w, http.StatusNotFound, "save file not found on disk")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", save.Size))
	w.Header().Set("X-SHA256", save.SHA256)
	w.WriteHeader(http.StatusOK)
	io.Copy(w, file)
}

// sanitizeFilename allows [A-Za-z0-9._-] and "/" for subfolders; rejects
// empty parts, "." and ".." to avoid traversal and hidden-file surprises.
// sanitizeFilename validates a (possibly nested) save file name. It rejects the
// characters that are invalid on at least one major OS plus empty/"."/".." path
// segments (traversal), so unicode and spaces in names such as "Save #1 (2025).sav"
// pass through unchanged.
// decodeFilenameParam decodes the {name} URL parameter. chi leaves %2F encoded
// (to keep it from breaking the route segment), so a client-escaped traversal
// like "..%2F..%2Fevil.sav" arrives still escaped — unescape first, then sanitize.
func decodeFilenameParam(r *http.Request) (string, bool) {
	raw := chi.URLParam(r, "name")
	name, err := url.PathUnescape(raw)
	if err != nil {
		return "", false
	}
	filename := sanitizeFilename(name)
	return filename, filename != ""
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.ContainsRune(name, '\u0000') {
		return ""
	}
	for _, part := range strings.Split(name, "/") {
		switch part {
		case ".", "..":
			return ""
		}
		for _, c := range part {
			// Forbidden on Windows (and separators elsewhere): / : * ? " < > |
			if c == '/' || c == ':' || c == '*' || c == '?' || c == '"' ||
				c == '<' || c == '>' || c == '|' {
				return ""
			}
		}
	}
	return name
}
