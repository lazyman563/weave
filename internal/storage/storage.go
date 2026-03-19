package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type Request struct {
	ID      int64
	Name    string
	URL     string
	Method  string
	Headers map[string]string
	Body    string
	Expect  Expectation
}

type Expectation struct {
	Buttons []string `yaml:"buttons" json:"buttons"`
	Fields  []string `yaml:"fields"  json:"fields"`
	Status  int      `yaml:"status"  json:"status"`
}

type HistoryEntry struct {
	ID         int64
	RequestID  int64
	Name       string
	URL        string
	Method     string
	StatusCode int
	Body       string
	Headers    string
	Duration   int64 // ms
	Timestamp  time.Time
	Error      string
}

type Flow struct {
	ID    int64
	Name  string
	Chain []string
	Delay int // ms between steps
}

func Open() (*DB, error) {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".weave", "weave.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir banco: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS requests (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		name     TEXT UNIQUE NOT NULL,
		url      TEXT NOT NULL,
		method   TEXT NOT NULL DEFAULT 'GET',
		headers  TEXT DEFAULT '{}',
		body     TEXT DEFAULT '',
		expect   TEXT DEFAULT '{}'
	);

	CREATE TABLE IF NOT EXISTS history (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id  INTEGER,
		name        TEXT,
		url         TEXT,
		method      TEXT,
		status_code INTEGER,
		body        TEXT,
		headers     TEXT,
		duration_ms INTEGER,
		timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP,
		error       TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS flows (
		id    INTEGER PRIMARY KEY AUTOINCREMENT,
		name  TEXT UNIQUE NOT NULL,
		chain TEXT NOT NULL,
		delay INTEGER DEFAULT 0
	);
	`
	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) SaveRequest(r *Request) error {
	headers, _ := json.Marshal(r.Headers)
	expect, _ := json.Marshal(r.Expect)

	_, err := db.conn.Exec(`
		INSERT INTO requests (name, url, method, headers, body, expect)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			url=excluded.url, method=excluded.method,
			headers=excluded.headers, body=excluded.body, expect=excluded.expect
	`, r.Name, r.URL, r.Method, string(headers), r.Body, string(expect))
	return err
}

func (db *DB) GetRequest(name string) (*Request, error) {
	row := db.conn.QueryRow(`SELECT id, name, url, method, headers, body, expect FROM requests WHERE name=?`, name)

	var r Request
	var headers, expect string
	err := row.Scan(&r.ID, &r.Name, &r.URL, &r.Method, &headers, &r.Body, &expect)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("request '%s' não encontrada", name)
	}
	if err != nil {
		return nil, err
	}

	r.Headers = make(map[string]string)
	json.Unmarshal([]byte(headers), &r.Headers)
	json.Unmarshal([]byte(expect), &r.Expect)
	return &r, nil
}

func (db *DB) ListRequests() ([]Request, error) {
	rows, err := db.conn.Query(`SELECT id, name, url, method, headers, body, expect FROM requests ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Request
	for rows.Next() {
		var r Request
		var headers, expect string
		rows.Scan(&r.ID, &r.Name, &r.URL, &r.Method, &headers, &r.Body, &expect)
		r.Headers = make(map[string]string)
		json.Unmarshal([]byte(headers), &r.Headers)
		json.Unmarshal([]byte(expect), &r.Expect)
		list = append(list, r)
	}
	return list, nil
}

func (db *DB) DeleteRequest(name string) error {
	res, err := db.conn.Exec(`DELETE FROM requests WHERE name=?`, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("request '%s' não encontrada", name)
	}
	return nil
}

func (db *DB) SaveHistory(h *HistoryEntry) error {
	_, err := db.conn.Exec(`
		INSERT INTO history (request_id, name, url, method, status_code, body, headers, duration_ms, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, h.RequestID, h.Name, h.URL, h.Method, h.StatusCode, h.Body, h.Headers, h.Duration, h.Error)
	return err
}

func (db *DB) GetHistory(limit int) ([]HistoryEntry, error) {
	rows, err := db.conn.Query(`
		SELECT id, request_id, name, url, method, status_code, body, headers, duration_ms, timestamp, error
		FROM history ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []HistoryEntry
	for rows.Next() {
		var h HistoryEntry
		var ts string
		rows.Scan(&h.ID, &h.RequestID, &h.Name, &h.URL, &h.Method,
			&h.StatusCode, &h.Body, &h.Headers, &h.Duration, &ts, &h.Error)
		h.Timestamp, _ = time.Parse("2006-01-02 15:04:05", ts)
		list = append(list, h)
	}
	return list, nil
}

func (db *DB) SaveFlow(f *Flow) error {
	chain, _ := json.Marshal(f.Chain)
	_, err := db.conn.Exec(`
		INSERT INTO flows (name, chain, delay) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET chain=excluded.chain, delay=excluded.delay
	`, f.Name, string(chain), f.Delay)
	return err
}

func (db *DB) GetFlow(name string) (*Flow, error) {
	row := db.conn.QueryRow(`SELECT id, name, chain, delay FROM flows WHERE name=?`, name)
	var f Flow
	var chain string
	err := row.Scan(&f.ID, &f.Name, &chain, &f.Delay)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("flow '%s' não encontrado", name)
	}
	json.Unmarshal([]byte(chain), &f.Chain)
	return &f, nil
}

func (db *DB) ListFlows() ([]Flow, error) {
	rows, err := db.conn.Query(`SELECT id, name, chain, delay FROM flows ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Flow
	for rows.Next() {
		var f Flow
		var chain string
		rows.Scan(&f.ID, &f.Name, &chain, &f.Delay)
		json.Unmarshal([]byte(chain), &f.Chain)
		list = append(list, f)
	}
	return list, nil
}

func (db *DB) Close() {
	db.conn.Close()
}
