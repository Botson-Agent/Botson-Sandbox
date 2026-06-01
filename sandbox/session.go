package sandbox

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// MessageToolCallFunction holds the function name and raw JSON arguments for a tool call.
type MessageToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// MessageToolCall represents a single tool invocation returned by the model.
type MessageToolCall struct {
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Function MessageToolCallFunction `json:"function"`
}

// Message represents a single chat conversation turn, supporting the full
// OpenAI-compatible tool calling protocol (assistant tool_calls + tool results).
type Message struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []MessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

// Session represents a persistent named or random agent development session
type Session struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	History    []Message `json:"history"`
}

var db *sql.DB

// InitDB guarantees that the SQLite database is initialized and tables are configured.
func InitDB() error {
	if db != nil {
		return nil
	}

	baseDir, err := GetSessionsDir()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(baseDir, 0755)

	dbPath := filepath.Join(filepath.Dir(baseDir), "sessions.db")

	var sqlErr error
	db, sqlErr = sql.Open("sqlite", dbPath)
	if sqlErr != nil {
		return fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, sqlErr)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		last_active DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS messages (
		session_id TEXT NOT NULL,
		sequence INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT,
		tool_calls TEXT,
		tool_call_id TEXT,
		PRIMARY KEY (session_id, sequence),
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);
	`
	_, err = db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize SQLite schema: %w", err)
	}
	return nil
}

// GetSessionsDir resolves the standard local base session cache directory
func GetSessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".botson-agent", "sessions"), nil
}

// GetDir returns the absolute path to this session's folder
func (s *Session) GetDir() (string, error) {
	base, err := GetSessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, s.ID), nil
}

// Save persists the session's metadata and conversation history to local SQLite
func (s *Session) Save() error {
	if err := InitDB(); err != nil {
		return err
	}
	s.LastActive = time.Now()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert or replace session
	_, err = tx.Exec("INSERT OR REPLACE INTO sessions (id, created_at, last_active) VALUES (?, ?, ?)", s.ID, s.CreatedAt, s.LastActive)
	if err != nil {
		return err
	}

	// Delete existing messages to rewrite
	_, err = tx.Exec("DELETE FROM messages WHERE session_id = ?", s.ID)
	if err != nil {
		return err
	}

	// Insert new messages
	for seq, m := range s.History {
		var toolCallsStr *string
		if len(m.ToolCalls) > 0 {
			tcBytes, err := json.Marshal(m.ToolCalls)
			if err == nil {
				str := string(tcBytes)
				toolCallsStr = &str
			}
		}

		_, err = tx.Exec("INSERT INTO messages (session_id, sequence, role, content, tool_calls, tool_call_id) VALUES (?, ?, ?, ?, ?, ?)",
			s.ID, seq, m.Role, m.Content, toolCallsStr, m.ToolCallID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadSession restores a saved session's context from SQLite
func LoadSession(id string) (*Session, error) {
	if err := InitDB(); err != nil {
		return nil, err
	}

	row := db.QueryRow("SELECT id, created_at, last_active FROM sessions WHERE id = ?", id)
	var s Session
	var createdAt, lastActive time.Time
	err := row.Scan(&s.ID, &createdAt, &lastActive)
	if err != nil {
		return nil, err
	}
	s.CreatedAt = createdAt
	s.LastActive = lastActive

	rows, err := db.Query("SELECT role, content, tool_calls, tool_call_id FROM messages WHERE session_id = ? ORDER BY sequence ASC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	s.History = []Message{}
	for rows.Next() {
		var m Message
		var toolCallsStr *string
		var toolCallID *string
		err := rows.Scan(&m.Role, &m.Content, &toolCallsStr, &toolCallID)
		if err != nil {
			return nil, err
		}
		if toolCallsStr != nil && *toolCallsStr != "" {
			var tc []MessageToolCall
			if err := json.Unmarshal([]byte(*toolCallsStr), &tc); err == nil {
				m.ToolCalls = tc
			}
		}
		if toolCallID != nil {
			m.ToolCallID = *toolCallID
		}
		s.History = append(s.History, m)
	}
	return &s, nil
}

// ListSessions scans the session cache DB and returns all saved session metadata
func ListSessions() ([]Session, error) {
	if err := InitDB(); err != nil {
		return nil, err
	}
	rows, err := db.Query("SELECT id, created_at, last_active FROM sessions ORDER BY last_active DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Session
	for rows.Next() {
		var s Session
		err := rows.Scan(&s.ID, &s.CreatedAt, &s.LastActive)
		if err != nil {
			return nil, err
		}

		histRows, err := db.Query("SELECT role, content, tool_calls, tool_call_id FROM messages WHERE session_id = ? ORDER BY sequence ASC", s.ID)
		if err == nil {
			s.History = []Message{}
			for histRows.Next() {
				var m Message
				var toolCallsStr *string
				var toolCallID *string
				if err := histRows.Scan(&m.Role, &m.Content, &toolCallsStr, &toolCallID); err == nil {
					if toolCallsStr != nil && *toolCallsStr != "" {
						var tc []MessageToolCall
						if err := json.Unmarshal([]byte(*toolCallsStr), &tc); err == nil {
							m.ToolCalls = tc
						}
					}
					if toolCallID != nil {
						m.ToolCallID = *toolCallID
					}
					s.History = append(s.History, m)
				}
			}
			histRows.Close()
		}

		list = append(list, s)
	}
	return list, nil
}
