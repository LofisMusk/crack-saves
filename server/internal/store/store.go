package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cracked-saves/server/internal/model"
)

// Sentinel errors used by handlers to pick HTTP status codes.
var (
	ErrNotFound      = errors.New("not found")
	ErrDuplicateName = errors.New("name already exists")
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// --- Games ---

func (s *Store) CreateGame(g *model.Game) error {
	now := time.Now().Unix()
	g.CreatedAt = now
	g.UpdatedAt = now
	res, err := s.db.Exec(
		`INSERT INTO games (id, name, exe_pattern, save_dir_hint, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.ExePattern, g.SaveDirHint, g.Notes, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("insert game: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("insert game: no rows affected")
	}
	return nil
}

func (s *Store) GetGame(id string) (*model.Game, error) {
	var g model.Game
	err := s.db.QueryRow(
		`SELECT id, name, exe_pattern, save_dir_hint, notes, created_at, updated_at
		 FROM games WHERE id = ?`, id,
	).Scan(&g.ID, &g.Name, &g.ExePattern, &g.SaveDirHint, &g.Notes, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get game: %w", err)
	}
	return &g, nil
}

func (s *Store) ListGames() ([]*model.Game, error) {
	rows, err := s.db.Query(
		`SELECT id, name, exe_pattern, save_dir_hint, notes, created_at, updated_at
		 FROM games ORDER BY name COLLATE NOCASE`,
	)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()

	var games []*model.Game
	for rows.Next() {
		var g model.Game
		if err := rows.Scan(&g.ID, &g.Name, &g.ExePattern, &g.SaveDirHint, &g.Notes, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		games = append(games, &g)
	}
	return games, rows.Err()
}

func (s *Store) UpdateGame(id string, patch func(*model.Game) error) (*model.Game, error) {
	existing, err := s.GetGame(id)
	if err != nil {
		return nil, err
	}
	if err := patch(existing); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(
		`UPDATE games SET name = ?, exe_pattern = ?, save_dir_hint = ?, notes = ?, updated_at = ? WHERE id = ?`,
		existing.Name, existing.ExePattern, existing.SaveDirHint, existing.Notes, time.Now().Unix(), id,
	); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update game: %w", err)
	}
	existing.UpdatedAt = time.Now().Unix()
	return existing, nil
}

func (s *Store) DeleteGame(id string) error {
	res, err := s.db.Exec(`DELETE FROM games WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete game: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Saves ---

func (s *Store) UpsertSave(sv *model.Save) error {
	res, err := s.db.Exec(
		`INSERT INTO saves (game_id, filename, sha256, size, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (game_id, filename) DO UPDATE SET
			sha256 = excluded.sha256,
			size = excluded.size,
			updated_at = excluded.updated_at`,
		sv.GameID, sv.Filename, sv.SHA256, sv.Size, sv.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert save: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("upsert save: no rows affected")
	}
	return nil
}

func (s *Store) GetSave(gameID, filename string) (*model.Save, error) {
	var sv model.Save
	err := s.db.QueryRow(
		`SELECT game_id, filename, sha256, size, updated_at FROM saves WHERE game_id = ? AND filename = ?`,
		gameID, filename,
	).Scan(&sv.GameID, &sv.Filename, &sv.SHA256, &sv.Size, &sv.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get save: %w", err)
	}
	return &sv, nil
}

func (s *Store) ListSaves(gameID string) ([]*model.Save, error) {
	rows, err := s.db.Query(
		`SELECT game_id, filename, sha256, size, updated_at FROM saves WHERE game_id = ? ORDER BY filename`,
		gameID,
	)
	if err != nil {
		return nil, fmt.Errorf("list saves: %w", err)
	}
	defer rows.Close()

	var saves []*model.Save
	for rows.Next() {
		var sv model.Save
		if err := rows.Scan(&sv.GameID, &sv.Filename, &sv.SHA256, &sv.Size, &sv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan save: %w", err)
		}
		saves = append(saves, &sv)
	}
	return saves, rows.Err()
}

func (s *Store) DeleteSave(gameID, filename string) error {
	res, err := s.db.Exec(`DELETE FROM saves WHERE game_id = ? AND filename = ?`, gameID, filename)
	if err != nil {
		return fmt.Errorf("delete save: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueViolation detects SQLite UNIQUE constraint violations.
// modernc.org/sqlite reports them as *sqlite.Error with code 5 (SQLITE_CONSTRAINT).
func isUniqueViolation(err error) bool {
	return errors.Is(err, ErrDuplicateName) ||
		contains(err.Error(), "UNIQUE constraint failed") ||
		contains(err.Error(), "constraint failed")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
