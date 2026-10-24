package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/cracked-saves/server/internal/api"
	"github.com/cracked-saves/server/internal/config"
	"github.com/cracked-saves/server/internal/db"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	cfg := &config.Config{Port: "0", DataDir: dir}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return api.New(database, cfg, logger)
}

func do(t *testing.T, client *http.Client, method, url string, body io.Reader) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func TestGameAndSaveLifecycle(t *testing.T) {
	server := httptest.NewServer(newTestServer(t))
	defer server.Close()
	base := server.URL
	client := server.Client()

	resp, _ := do(t, client, http.MethodGet, base+"/api/health", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health: got %d", resp.StatusCode)
	}

	// create game
	payload := `{"name":"Test Game","exe_pattern":"testgame.exe","save_dir_hint":"~/saves/test"}`
	resp, data := do(t, client, http.MethodPost, base+"/api/games", bytes.NewBufferString(payload))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create game: got %d, body=%s", resp.StatusCode, data)
	}
	var game struct{ ID string }
	if err := json.Unmarshal(data, &game); err != nil || game.ID == "" {
		t.Fatalf("parse game: err=%v id=%q", err, game.ID)
	}

	// duplicate name -> 409
	resp, _ = do(t, client, http.MethodPost, base+"/api/games", bytes.NewBufferString(payload))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate create: got %d want 409", resp.StatusCode)
	}

	// upload a save
	content := []byte("saveslot:data:v1")
	resp, data = do(t, client, http.MethodPut, fmt.Sprintf("%s/api/games/%s/saves/save1.sav", base, game.ID), bytes.NewReader(content))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload: got %d body=%s", resp.StatusCode, data)
	}
	var save struct {
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	}
	if err := json.Unmarshal(data, &save); err != nil || save.Size != int64(len(content)) {
		t.Fatalf("parse save: err=%v size=%d want %d", err, save.Size, len(content))
	}
	if save.SHA256 == "" || len(save.SHA256) != 64 {
		t.Fatalf("bad hash: %q", save.SHA256)
	}

	// list saves
	resp, data = do(t, client, http.MethodGet, fmt.Sprintf("%s/api/games/%s/saves", base, game.ID), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list saves: got %d", resp.StatusCode)
	}
	var saves []struct {
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	if err := json.Unmarshal(data, &saves); err != nil || len(saves) != 1 {
		t.Fatalf("parse saves: err=%v n=%d", err, len(saves))
	}

	// download and verify round-trip
	resp, data = do(t, client, http.MethodGet, fmt.Sprintf("%s/api/games/%s/saves/save1.sav", base, game.ID), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download: got %d", resp.StatusCode)
	}
	if string(data) != string(content) {
		t.Fatalf("roundtrip mismatch: got %q want %q", data, content)
	}

	// resave the same content (overwrite) still works
	resp, _ = do(t, client, http.MethodPut, fmt.Sprintf("%s/api/games/%s/saves/save1.sav", base, game.ID), bytes.NewReader(content))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("re-upload: got %d", resp.StatusCode)
	}

	// patch game
	resp, _ = do(t, client, http.MethodPatch, fmt.Sprintf("%s/api/games/%s", base, game.ID),
		bytes.NewBufferString(`{"notes":"patched"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: got %d", resp.StatusCode)
	}

	// get game shows the patch
	resp, data = do(t, client, http.MethodGet, fmt.Sprintf("%s/api/games/%s", base, game.ID), nil)
	var patched struct{ Notes string }
	if err := json.Unmarshal(data, &patched); err != nil || patched.Notes != "patched" {
		t.Fatalf("get after patch: err=%v notes=%q", err, patched.Notes)
	}

	// delete game cascades saves
	resp, _ = do(t, client, http.MethodDelete, fmt.Sprintf("%s/api/games/%s", base, game.ID), nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: got %d", resp.StatusCode)
	}
	resp, _ = do(t, client, http.MethodGet, fmt.Sprintf("%s/api/games/%s/saves/save1.sav", base, game.ID), nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("download after delete: got %d want 404", resp.StatusCode)
	}

	// invalid filenames -> 400 (traversal blocked)
	resp, _ = do(t, client, http.MethodPut, base+"/api/games/unknown/saves/..", bytes.NewReader(content))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("traversal upload: got %d want 400", resp.StatusCode)
	}
}

func TestSaveFilenameWithSpacesAndSymbols(t *testing.T) {
	server := httptest.NewServer(newTestServer(t))
	defer server.Close()

	content := []byte("spacey save")
	resp, data := do(t, server.Client(), http.MethodPost, server.URL+"/api/games", bytes.NewReader([]byte(`{"name":"Symbolic","exe_pattern":"s.exe"}`)))
	var game struct{ ID string }
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d body=%s", resp.StatusCode, data)
	}
	if err := json.Unmarshal(data, &game); err != nil {
		t.Fatalf("decode game: %v (body=%s)", err, data)
	}

	name := "My Save #1 (2025).sav"
	resp, data = do(t, server.Client(), http.MethodPut,
		server.URL+"/api/games/"+game.ID+"/saves/"+url.PathEscape(name), bytes.NewReader(content))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: got %d body=%s", resp.StatusCode, data)
	}

	resp, data = do(t, server.Client(), http.MethodGet,
		server.URL+"/api/games/"+game.ID+"/saves/"+url.PathEscape(name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download: got %d body=%s", resp.StatusCode, data)
	}
	if string(data) != string(content) {
		t.Fatalf("round-trip mismatch: %q vs %q", data, content)
	}

	// escaped traversal (client url_encodes, chi keeps %2F encoded) -> 400
	resp, _ = do(t, server.Client(), http.MethodPut,
		server.URL+"/api/games/"+game.ID+"/saves/"+url.PathEscape("../../evil.sav"), bytes.NewReader(content))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("escaped traversal: got %d want 400", resp.StatusCode)
	}

	// forbidden windows chars -> 400
	for _, bad := range []string{"a:b.sav", `a?b.sav`, "a|b.sav"} {
		resp, _ = do(t, server.Client(), http.MethodPut,
			server.URL+"/api/games/"+game.ID+"/saves/"+url.PathEscape(bad), bytes.NewReader(content))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("upload %q: got %d want 400", bad, resp.StatusCode)
		}
	}
}

func TestAuthRequired(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	cfg := &config.Config{Port: "0", DataDir: dir, APIToken: "secret"}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	server := httptest.NewServer(api.New(database, cfg, logger))
	defer server.Close()

	// no token -> 401
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/games", nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: got %d want 401", resp.StatusCode)
	}

	// wrong token -> 401
	req.Header.Set("Authorization", "Bearer wrong")
	resp, _ = server.Client().Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token: got %d want 401", resp.StatusCode)
	}

	// correct token -> 200
	req.Header.Set("Authorization", "Bearer secret")
	resp, _ = server.Client().Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("with token: got %d want 200", resp.StatusCode)
	}

	// health stays public
	resp, _ = do(t, server.Client(), http.MethodGet, server.URL+"/api/health", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authed health: got %d", resp.StatusCode)
	}
}
