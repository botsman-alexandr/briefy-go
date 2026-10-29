package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxSessions = 500
	defaultBrief       = "[CAVEMAN] Кратко. Без вступлений, вежливости, хеджирования. " +
		"Код без пояснений. Не пересказывать вопрос. Тех-термины точные."
	refreshEvery = 10 // каждый N-й ход -> полный core
)

type payload struct {
	SessionID string `json:"session_id"`
}

type sessionState struct {
	TurnCount int   `json:"turnCount"`
	LastSeen  int64 `json:"lastSeen"`
}

type brieflyConfig struct {
	Core        string
	Examples    string
	Brief       string
	MaxSessions int
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func main() {
	run() // fail-open: любая ошибка -> тихий выход с кодом 0
}

func run() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(raw) == 0 {
		return
	}

	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}

	base := resolveBaseDir()
	logDir := filepath.Join(base, "briefly_logs")
	dbFile := filepath.Join(logDir, "db.json")
	brieflyFile := filepath.Join(base, "BRIFLY.md")

	_ = os.MkdirAll(logDir, 0o755)

	sessionID := p.SessionID
	if sessionID == "" {
		sessionID = "default_session"
	}

	cfg := loadBriefly(brieflyFile)

	db := loadDB(dbFile)
	state := db[sessionID]
	if state == nil {
		state = &sessionState{}
	}
	state.TurnCount++
	state.LastSeen = time.Now().UnixMilli()
	db[sessionID] = state
	saveDB(dbFile, trimDB(db, cfg.MaxSessions))

	ctx := buildContext(cfg, state.TurnCount)

	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "UserPromptSubmit"
	out.HookSpecificOutput.AdditionalContext = ctx

	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	_, _ = os.Stdout.Write(b)
}

// base dir: рядом с exe (собираем); при go run exe в %TEMP%, там нет BRIFLY.md — fallback на ~/.gigacode/hooks/briefly-go
func resolveBaseDir() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "BRIFLY.md")); err == nil {
			return dir
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".gigacode", "hooks", "briefly-go")
	}
	return "."
}

func buildContext(cfg brieflyConfig, turn int) string {
	switch {
	case cfg.Core != "" && turn == 1:
		ctx := "Следуй правилам CAVEMAN:\n\n" + cfg.Core
		if cfg.Examples != "" {
			ctx += "\n\n" + cfg.Examples
		}
		return ctx
	case cfg.Core != "" && turn%refreshEvery == 0:
		return "Следуй правилам CAVEMAN:\n\n" + cfg.Core
	default:
		return cfg.Brief
	}
}

func loadBriefly(path string) brieflyConfig {
	cfg := brieflyConfig{
		Brief:       defaultBrief,
		MaxSessions: defaultMaxSessions,
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	raw := string(b)

	cfg.Core = extractBetween(raw, "<!-- CORE:START -->", "<!-- CORE:END -->")
	cfg.Examples = extractBetween(raw, "<!-- EXAMPLES:START -->", "<!-- EXAMPLES:END -->")
	if cfg.Core == "" { // маркеры не найдены — fallback на весь файл
		cfg.Core = strings.TrimSpace(raw)
	}

	parseConfig(extractBetween(raw, "<!-- CONFIG:START -->", "<!-- CONFIG:END -->"), &cfg)
	return cfg
}

func parseConfig(block string, cfg *brieflyConfig) {
	if block == "" {
		return
	}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "maxSessions":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.MaxSessions = n
			}
		case "brief":
			if val != "" {
				cfg.Brief = val
			}
		}
	}
}

func extractBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:j])
}

func loadDB(path string) map[string]*sessionState {
	db := make(map[string]*sessionState)
	b, err := os.ReadFile(path)
	if err != nil {
		return db
	}
	_ = json.Unmarshal(b, &db) // битый db -> пустая карта
	return db
}

func saveDB(path string, db map[string]*sessionState) {
	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func trimDB(db map[string]*sessionState, limit int) map[string]*sessionState {
	if limit <= 0 || len(db) <= limit {
		return db
	}
	keys := make([]string, 0, len(db))
	for k := range db {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return db[keys[i]].LastSeen > db[keys[j]].LastSeen
	})
	kept := make(map[string]*sessionState, limit)
	for _, k := range keys[:limit] {
		kept[k] = db[k]
	}
	return kept
}