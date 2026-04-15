package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// EngramMemory represents the persistent memory system
type EngramMemory struct {
	db *sql.DB
}

// Memory represents a single memory entry
type Memory struct {
	ID        int64                  `json:"id"`
	SessionID string                 `json:"session_id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
	Embedding []float64              `json:"embedding,omitempty"`
}

// NewEngramMemory creates a new Engram memory instance
func NewEngramMemory(dbPath string) (*EngramMemory, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable FTS5 and configure
	if _, err := db.Exec(`
		PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
	`); err != nil {
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	engram := &EngramMemory{db: db}
	if err := engram.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return engram, nil
}

// initSchema creates the database schema
func (e *EngramMemory) initSchema() error {
	schema := `
	-- Main memories table
	CREATE TABLE IF NOT EXISTS memories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		content TEXT NOT NULL,
		metadata TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Full-text search virtual table
	CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		content,
		metadata,
		content_rowid=rowid
	);

	-- Sessions table for grouping memories
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		title TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Tags for categorization
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS memory_tags (
		memory_id INTEGER REFERENCES memories(id) ON DELETE CASCADE,
		tag_id INTEGER REFERENCES tags(id) ON DELETE CASCADE,
		PRIMARY KEY (memory_id, tag_id)
	);

	-- Triggers to keep FTS index updated
	CREATE TRIGGER IF NOT EXISTS memories_insert_fts 
	AFTER INSERT ON memories BEGIN
		INSERT INTO memories_fts(rowid, content, metadata) 
		VALUES (new.id, new.content, new.metadata);
	END;

	CREATE TRIGGER IF NOT EXISTS memories_delete_fts 
	AFTER DELETE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, content, metadata) 
		VALUES ('delete', old.id, old.content, old.metadata);
	END;

	CREATE TRIGGER IF NOT EXISTS memories_update_fts 
	AFTER UPDATE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, content, metadata) 
		VALUES ('delete', old.id, old.content, old.metadata);
		INSERT INTO memories_fts(rowid, content, metadata) 
		VALUES (new.id, new.content, new.metadata);
	END;

	-- Index for faster queries
	CREATE INDEX IF NOT EXISTS idx_memories_session ON memories(session_id);
	CREATE INDEX IF NOT EXISTS idx_memories_timestamp ON memories(timestamp);
	`

	_, err := e.db.Exec(schema)
	return err
}

// Save stores a new memory
func (e *EngramMemory) Save(sessionID string, content string, metadata map[string]interface{}) (*Memory, error) {
	if sessionID == "" {
		sessionID = fmt.Sprintf("session-%d", time.Now().Unix())
	}

	// Ensure session exists
	if _, err := e.db.Exec(
		"INSERT OR IGNORE INTO sessions (id, title) VALUES (?, ?)",
		sessionID, sessionID,
	); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	metadataJSON, _ := json.Marshal(metadata)

	result, err := e.db.Exec(
		"INSERT INTO memories (session_id, content, metadata) VALUES (?, ?, ?)",
		sessionID, content, string(metadataJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save memory: %w", err)
	}

	id, _ := result.LastInsertId()

	// Update session timestamp
	e.db.Exec("UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", sessionID)

	return &Memory{
		ID:        id,
		SessionID: sessionID,
		Content:   content,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}, nil
}

// Query searches memories using FTS5
func (e *EngramMemory) Query(query string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 10
	}

	// Use FTS5 for full-text search
	rows, err := e.db.Query(`
		SELECT m.id, m.session_id, m.content, m.metadata, m.timestamp
		FROM memories m
		JOIN memories_fts fts ON m.id = fts.rowid
		WHERE memories_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}
	defer rows.Close()

	return e.scanMemories(rows)
}

// GetRecent retrieves recent memories
func (e *EngramMemory) GetRecent(sessionID string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 50
	}

	var rows *sql.Rows
	var err error

	if sessionID != "" {
		rows, err = e.db.Query(`
			SELECT id, session_id, content, metadata, timestamp
			FROM memories
			WHERE session_id = ?
			ORDER BY timestamp DESC
			LIMIT ?
		`, sessionID, limit)
	} else {
		rows, err = e.db.Query(`
			SELECT id, session_id, content, metadata, timestamp
			FROM memories
			ORDER BY timestamp DESC
			LIMIT ?
		`, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get recent memories: %w", err)
	}
	defer rows.Close()

	return e.scanMemories(rows)
}

// GetSession retrieves all memories from a session
func (e *EngramMemory) GetSession(sessionID string) ([]Memory, error) {
	rows, err := e.db.Query(`
		SELECT id, session_id, content, metadata, timestamp
		FROM memories
		WHERE session_id = ?
		ORDER BY timestamp ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer rows.Close()

	return e.scanMemories(rows)
}

// GetSessions lists all sessions
func (e *EngramMemory) GetSessions(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := e.db.Query(`
		SELECT id, title, created_at, updated_at,
		       (SELECT COUNT(*) FROM memories WHERE session_id = sessions.id) as memory_count
		FROM sessions
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []map[string]interface{}
	for rows.Next() {
		var id, title string
		var createdAt, updatedAt time.Time
		var memoryCount int

		if err := rows.Scan(&id, &title, &createdAt, &updatedAt, &memoryCount); err != nil {
			continue
		}

		sessions = append(sessions, map[string]interface{}{
			"id":           id,
			"title":        title,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
			"memory_count": memoryCount,
		})
	}

	return sessions, nil
}

// Delete removes a memory
func (e *EngramMemory) Delete(id int64) error {
	_, err := e.db.Exec("DELETE FROM memories WHERE id = ?", id)
	return err
}

// Close closes the database connection
func (e *EngramMemory) Close() error {
	return e.db.Close()
}

// Helper function to scan memory rows
func (e *EngramMemory) scanMemories(rows *sql.Rows) ([]Memory, error) {
	var memories []Memory

	for rows.Next() {
		var m Memory
		var metadataStr string

		err := rows.Scan(&m.ID, &m.SessionID, &m.Content, &metadataStr, &m.Timestamp)
		if err != nil {
			continue
		}

		json.Unmarshal([]byte(metadataStr), &m.Metadata)
		memories = append(memories, m)
	}

	return memories, nil
}

// Backup creates a backup of the database
func (e *EngramMemory) Backup(backupPath string) error {
	_, err := e.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	return err
}

// Stats returns database statistics
func (e *EngramMemory) Stats() (map[string]interface{}, error) {
	var memoryCount, sessionCount int

	err := e.db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&memoryCount)
	if err != nil {
		return nil, err
	}

	err = e.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_memories":  memoryCount,
		"total_sessions":  sessionCount,
		"database_size":   "N/A", // Could add file size check
	}, nil
}

func main() {
	// Default database path
	dbPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "engram", "claw-memory.db")
	if len(os.Args) > 1 && os.Args[1] == "--db" {
		dbPath = os.Args[2]
	}

	engram, err := NewEngramMemory(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer engram.Close()

	// Command handling
	if len(os.Args) < 2 {
		fmt.Println("Usage: engram [save|query|recent|sessions|stats]")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "save":
		if len(os.Args) < 3 {
			fmt.Println("Usage: engram save <content> [session_id]")
			os.Exit(1)
		}
		content := os.Args[2]
		sessionID := ""
		if len(os.Args) > 3 {
			sessionID = os.Args[3]
		}

		mem, err := engram.Save(sessionID, content, nil)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Saved memory #%d to session %s\n", mem.ID, mem.SessionID)

	case "query":
		if len(os.Args) < 3 {
			fmt.Println("Usage: engram query <search-term>")
			os.Exit(1)
		}
		query := os.Args[2]

		memories, err := engram.Query(query, 10)
		if err != nil {
			log.Fatal(err)
		}

		for _, m := range memories {
			fmt.Printf("[%s] %s\n", m.Timestamp.Format("2006-01-02 15:04"), m.Content[:min(len(m.Content), 100)])
		}

	case "recent":
		sessionID := ""
		if len(os.Args) > 2 {
			sessionID = os.Args[2]
		}

		memories, err := engram.GetRecent(sessionID, 20)
		if err != nil {
			log.Fatal(err)
		}

		for _, m := range memories {
			fmt.Printf("[%s] %s\n", m.Timestamp.Format("2006-01-02 15:04"), m.Content[:min(len(m.Content), 100)])
		}

	case "sessions":
		sessions, err := engram.GetSessions(10)
		if err != nil {
			log.Fatal(err)
		}

		for _, s := range sessions {
			fmt.Printf("%s: %s (%d memories)\n", s["id"], s["title"], s["memory_count"])
		}

	case "stats":
		stats, err := engram.Stats()
		if err != nil {
			log.Fatal(err)
		}

		for k, v := range stats {
			fmt.Printf("%s: %v\n", k, v)
		}

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
