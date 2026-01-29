package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CacheEntry represents a cached response
type CacheEntry struct {
	Response    string    `json:"response"`
	InputHash   string    `json:"input_hash"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	TokensUsed  int       `json:"tokens_used"`
}

// ResponseCache provides caching for similar queries
type ResponseCache struct {
	Dir       string
	MaxAge    time.Duration
	mu        sync.RWMutex
	CacheSize int
}

func NewResponseCache(dir string, maxAge time.Duration) *ResponseCache {
	os.MkdirAll(dir, 0755)
	return &ResponseCache{
		Dir:     dir,
		MaxAge:  maxAge,
		CacheSize: 0,
	}
}

// GenerateKey creates a unique key for a query
func (c *ResponseCache) GenerateKey(query string, context string) string {
	data := query + "||" + context
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// Get retrieves a cached response
func (c *ResponseCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filePath := filepath.Join(c.Dir, key+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", false
	}

	if time.Now().After(entry.ExpiresAt) {
		os.Remove(filePath)
		return "", false
	}

	return entry.Response, true
}

// Set stores a response in cache
func (c *ResponseCache) Set(key string, response string, tokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	filePath := filepath.Join(c.Dir, key+".json")
	entry := CacheEntry{
		Response:  response,
		InputHash: key,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(c.MaxAge),
		TokensUsed: tokens,
	}

	data, _ := json.MarshalIndent(entry, "", "  ")
	os.WriteFile(filePath, data, 0644)
	c.CacheSize++
}

// Cleanup removes expired entries
func (c *ResponseCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, _ := os.ReadDir(c.Dir)
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(c.Dir, entry.Name())
		data, _ := os.ReadFile(filePath)
		var ce CacheEntry
		if json.Unmarshal(data, &ce) == nil && time.Now().After(ce.ExpiresAt) {
			os.Remove(filePath)
			count++
		}
	}
	c.CacheSize -= count
}

// Stats returns cache statistics
func (c *ResponseCache) Stats() (size int, hits int, misses int) {
	entries, _ := os.ReadDir(c.Dir)
	return len(entries), 0, 0
}

func (c *ResponseCache) PrintStats() {
	size, _, _ := c.Stats()
	fmt.Printf("📦 Cache: %d entries\n", size)
}

// ChatHistory stores persistent chat sessions
type ChatHistory struct {
	Dir         string
	MaxSessions int
	mu          sync.RWMutex
}

type ChatSession struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Messages  []ChatMessage  `json:"messages"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Mode      string         `json:"mode"`
}

type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func NewChatHistory(dir string, maxSessions int) *ChatHistory {
	os.MkdirAll(dir, 0755)
	return &ChatHistory{
		Dir:         dir,
		MaxSessions: maxSessions,
	}
}

// Save saves a chat session
func (h *ChatHistory) Save(session *ChatSession) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	session.UpdatedAt = time.Now()
	data, _ := json.MarshalIndent(session, "", "  ")
	fileName := fmt.Sprintf("chat_%s.json", session.ID)
	return os.WriteFile(filepath.Join(h.Dir, fileName), data, 0644)
}

// Load loads a chat session
func (h *ChatHistory) Load(id string) (*ChatSession, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := os.ReadFile(filepath.Join(h.Dir, fmt.Sprintf("chat_%s.json", id)))
	if err != nil {
		return nil, err
	}

	var session ChatSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// List returns all chat sessions
func (h *ChatHistory) List() ([]*ChatSession, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries, err := os.ReadDir(h.Dir)
	if err != nil {
		return nil, err
	}

	var sessions []*ChatSession
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "chat_") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(h.Dir, entry.Name()))
		var session ChatSession
		if json.Unmarshal(data, &session) == nil {
			sessions = append(sessions, &session)
		}
	}

	return sessions, nil
}

// Delete removes a chat session
func (h *ChatHistory) Delete(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	fileName := fmt.Sprintf("chat_%s.json", id)
	return os.Remove(filepath.Join(h.Dir, fileName))
}

// NewSession creates a new chat session
func (h *ChatHistory) NewSession(title string, mode string) *ChatSession {
	return &ChatSession{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Title:     title,
		Messages:  []ChatMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Mode:      mode,
	}
}

// CommandAliases stores user-defined command shortcuts
type CommandAliases struct {
	Dir  string
	mu   sync.RWMutex
	Data map[string]string
}

func NewCommandAliases(dir string) *CommandAliases {
	os.MkdirAll(dir, 0755)
	
	aliases := &CommandAliases{
		Dir:  dir,
		Data: make(map[string]string),
	}
	
	// Load existing aliases
	filePath := filepath.Join(dir, "aliases.json")
	data, _ := os.ReadFile(filePath)
	json.Unmarshal(data, &aliases.Data)
	
	return aliases
}

// Save persists aliases to disk
func (a *CommandAliases) Save() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	data, _ := json.MarshalIndent(a.Data, "", "  ")
	return os.WriteFile(filepath.Join(a.Dir, "aliases.json"), data, 0644)
}

// Get retrieves an alias
func (a *CommandAliases) Get(key string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	value, ok := a.Data[key]
	return value, ok
}

// Set creates or updates an alias
func (a *CommandAliases) Set(key string, value string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.Data[key] = value
	return a.Save()
}

// Delete removes an alias
func (a *CommandAliases) Delete(key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	delete(a.Data, key)
	return a.Save()
}

// Expand replaces aliases in a command
func (a *CommandAliases) Expand(cmd string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	result := cmd
	for key, value := range a.Data {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}

// List returns all aliases
func (a *CommandAliases) List() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	return a.Data
}

// Default aliases
var DefaultAliases = map[string]string{
	"ls":     "list_dir .",
	"cat":    "read_file",
	"run":    "exec",
	"find":   "search_files",
	"grep":   "grep",
	"show":   "read_file",
	"make":   "exec make",
	"build":  "exec go build",
	"test":   "exec go test",
}
