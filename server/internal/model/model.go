package model

// Game is a registered game (usually launched from outside any store launcher).
type Game struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ExePattern  string `json:"exe_pattern"`   // substring match against process names, case-insensitive
	SaveDirHint string `json:"save_dir_hint"` // local directory containing the game's saves
	Notes       string `json:"notes"`
	CreatedAt   int64  `json:"created_at"` // unix seconds
	UpdatedAt   int64  `json:"updated_at"` // unix seconds
}

// Save is the metadata of the latest synced copy of a save file.
type Save struct {
	GameID    string `json:"game_id"`
	Filename  string `json:"filename"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	UpdatedAt int64  `json:"updated_at"` // unix seconds
}
