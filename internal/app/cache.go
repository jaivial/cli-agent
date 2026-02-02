package app

import (
	"container/list"
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
	Response   string    `json:"response"`
	InputHash  string    `json:"input_hash"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	TokensUsed int       `json:"tokens_used"`
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
		Dir:       dir,
		MaxAge:    maxAge,
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
		Response:   response,
		InputHash:  key,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(c.MaxAge),
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
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Mode      string        `json:"mode"`
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
	"ls":    "list_dir .",
	"cat":   "read_file",
	"run":   "exec",
	"find":  "search_files",
	"grep":  "grep",
	"show":  "read_file",
	"make":  "exec make",
	"build": "exec go build",
	"test":  "exec go test",
}

// CacheStats holds cache performance statistics
type CacheStats struct {
	Size          int
	HitCount      int64
	MissCount     int64
	EvictionCount int64
	HitRate       float64
}

// LRUCache implements a Least Recently Used cache with max entries
type LRUCache struct {
	maxEntries int
	entries    map[string]*list.Element
	order      *list.List
	mu         sync.RWMutex
}

// lruEntry represents a single entry in the LRU cache
type lruEntry struct {
	key   string
	value *ToolCacheEntry
}

// NewLRUCache creates a new LRU cache with the specified maximum entries
func NewLRUCache(maxEntries int) *LRUCache {
	return &LRUCache{
		maxEntries: maxEntries,
		entries:    make(map[string]*list.Element),
		order:      list.New(),
	}
}

// Get retrieves an entry from the LRU cache
func (c *LRUCache) Get(key string) (*ToolCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Move to front (most recently used)
	c.order.MoveToFront(elem)
	return elem.Value.(*lruEntry).value, true
}

// Put adds or updates an entry in the LRU cache
func (c *LRUCache) Put(key string, value *ToolCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key exists, update and move to front
	if elem, exists := c.entries[key]; exists {
		c.order.MoveToFront(elem)
		elem.Value.(*lruEntry).value = value
		return
	}

	// Add new entry
	elem := c.order.PushFront(&lruEntry{key: key, value: value})
	c.entries[key] = elem

	// Evict oldest if over capacity
	if c.maxEntries > 0 && c.order.Len() > c.maxEntries {
		c.evictOldest()
	}
}

// evictOldest removes the least recently used entry
func (c *LRUCache) evictOldest() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// removeElement removes a specific element from the cache
func (c *LRUCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*lruEntry)
	delete(c.entries, entry.key)
	c.order.Remove(elem)
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.entries[key]; exists {
		c.removeElement(elem)
	}
}

// Len returns the number of entries in the cache
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// ToolResultCache provides in-memory caching for tool execution results.
type ToolResultCache struct {
	mu            sync.RWMutex
	entries       map[string]*ToolCacheEntry
	maxAge        time.Duration
	lru           *LRUCache
	hitCount      int64
	missCount     int64
	evictionCount int64
}

// ToolCacheEntry represents a cached tool result
type ToolCacheEntry struct {
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	Success   bool      `json:"success"`
	CachedAt  time.Time `json:"cached_at"`
	FileMtime time.Time `json:"file_mtime,omitempty"` // For file-based cache invalidation
}

// NewToolResultCache creates a new cache with the specified maximum age.
func NewToolResultCache(maxAge time.Duration) *ToolResultCache {
	return &ToolResultCache{
		entries: make(map[string]*ToolCacheEntry),
		maxAge:  maxAge,
		lru:     NewLRUCache(1000), // Max 1000 entries for LRU eviction
	}
}

// getMaxAge returns the appropriate max age based on tool type
func (c *ToolResultCache) getMaxAge(toolName string) time.Duration {
	switch toolName {
	case "read_file":
		return 10 * time.Minute // Longer for files
	case "list_dir":
		return 1 * time.Minute // Shorter for directories (they change frequently)
	case "grep":
		return 2 * time.Minute
	case "search_files":
		return 3 * time.Minute
	default:
		return c.maxAge
	}
}

// WarmCache pre-loads common files into the cache
func (c *ToolResultCache) WarmCache(paths []string) {
	for _, path := range paths {
		// Check if file exists
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		// Skip directories
		if info.IsDir() {
			continue
		}

		// Read file content
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Pre-populate cache
		c.mu.Lock()
		key := c.GenerateReadFileKey(path)
		c.entries[key] = &ToolCacheEntry{
			Output:    string(data),
			Success:   true,
			CachedAt:  time.Now(),
			FileMtime: info.ModTime(),
		}
		c.lru.Put(key, c.entries[key])
		c.mu.Unlock()
	}
}

// WarmCommonPaths pre-loads common configuration files
func (c *ToolResultCache) WarmCommonPaths(baseDir string) {
	commonPaths := []string{
		filepath.Join(baseDir, "go.mod"),
		filepath.Join(baseDir, "go.sum"),
		filepath.Join(baseDir, "package.json"),
		filepath.Join(baseDir, "package-lock.json"),
		filepath.Join(baseDir, "requirements.txt"),
		filepath.Join(baseDir, "pyproject.toml"),
		filepath.Join(baseDir, "Cargo.toml"),
		filepath.Join(baseDir, "Makefile"),
		filepath.Join(baseDir, "README.md"),
		filepath.Join(baseDir, ".gitignore"),
	}
	c.WarmCache(commonPaths)
}

// Stats returns cache statistics
func (c *ToolResultCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hitCount + c.missCount
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hitCount) / float64(total) * 100
	}

	return CacheStats{
		Size:          len(c.entries),
		HitCount:      c.hitCount,
		MissCount:     c.missCount,
		EvictionCount: c.evictionCount,
		HitRate:       hitRate,
	}
}

// RecordHit records a cache hit
func (c *ToolResultCache) RecordHit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hitCount++
}

// RecordMiss records a cache miss
func (c *ToolResultCache) RecordMiss() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.missCount++
}

// GenerateReadFileKey creates a cache key for read_file operations
func (c *ToolResultCache) GenerateReadFileKey(path string) string {
	return "read_file:" + path
}

// GenerateListDirKey creates a cache key for list_dir operations
func (c *ToolResultCache) GenerateListDirKey(path string) string {
	return "list_dir:" + path
}

// GenerateGrepKey creates a cache key for grep operations
func (c *ToolResultCache) GenerateGrepKey(pattern, path string, recursive bool) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("grep:%s:%s:%t", pattern, path, recursive)))
	return "grep:" + hex.EncodeToString(hash[:16])
}

// GetReadFile retrieves a cached read_file result if valid.
func (c *ToolResultCache) GetReadFile(path string) (*ToolCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.GenerateReadFileKey(path)
	entry, exists := c.entries[key]
	if !exists {
		c.missCount++
		return nil, false
	}

	// Check if cache has expired (using tool-specific max age)
	maxAge := c.getMaxAge("read_file")
	if time.Since(entry.CachedAt) > maxAge {
		c.missCount++
		return nil, false
	}

	// Validate file mtime - if file was modified, invalidate cache
	info, err := os.Stat(path)
	if err != nil {
		c.missCount++
		return nil, false
	}
	if info.ModTime().After(entry.FileMtime) {
		c.missCount++
		return nil, false
	}

	// Update LRU order
	c.lru.Get(key)
	c.hitCount++
	return entry, true
}

// SetReadFile stores a read_file result in the cache.
func (c *ToolResultCache) SetReadFile(path string, output string, success bool, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.GenerateReadFileKey(path)

	var mtime time.Time
	if info, err := os.Stat(path); err == nil {
		mtime = info.ModTime()
	}

	entry := &ToolCacheEntry{
		Output:    output,
		Error:     errMsg,
		Success:   success,
		CachedAt:  time.Now(),
		FileMtime: mtime,
	}

	c.entries[key] = entry
	c.lru.Put(key, entry)
}

// GetListDir retrieves a cached list_dir result
func (c *ToolResultCache) GetListDir(path string) (*ToolCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.GenerateListDirKey(path)
	entry, exists := c.entries[key]
	if !exists {
		c.missCount++
		return nil, false
	}

	// Check if cache has expired (using tool-specific max age)
	maxAge := c.getMaxAge("list_dir")
	if time.Since(entry.CachedAt) > maxAge {
		c.missCount++
		return nil, false
	}

	// Check if directory was modified
	info, err := os.Stat(path)
	if err != nil {
		c.missCount++
		return nil, false
	}
	if info.ModTime().After(entry.FileMtime) {
		c.missCount++
		return nil, false
	}

	// Update LRU order
	c.lru.Get(key)
	c.hitCount++
	return entry, true
}

// SetListDir stores a list_dir result in cache
func (c *ToolResultCache) SetListDir(path string, output string, success bool, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.GenerateListDirKey(path)

	var mtime time.Time
	if info, err := os.Stat(path); err == nil {
		mtime = info.ModTime()
	}

	entry := &ToolCacheEntry{
		Output:    output,
		Error:     errMsg,
		Success:   success,
		CachedAt:  time.Now(),
		FileMtime: mtime,
	}

	c.entries[key] = entry
	c.lru.Put(key, entry)
}

// GetGrep retrieves a cached grep result
func (c *ToolResultCache) GetGrep(pattern, path string, recursive bool) (*ToolCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.GenerateGrepKey(pattern, path, recursive)
	entry, exists := c.entries[key]
	if !exists {
		c.missCount++
		return nil, false
	}

	// Check if cache has expired (using tool-specific max age)
	maxAge := c.getMaxAge("grep")
	if time.Since(entry.CachedAt) > maxAge {
		c.missCount++
		return nil, false
	}

	// Update LRU order
	c.lru.Get(key)
	c.hitCount++
	return entry, true
}

// SetGrep stores a grep result in cache
func (c *ToolResultCache) SetGrep(pattern, path string, recursive bool, output string, success bool, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.GenerateGrepKey(pattern, path, recursive)

	entry := &ToolCacheEntry{
		Output:   output,
		Error:    errMsg,
		Success:  success,
		CachedAt: time.Now(),
	}

	c.entries[key] = entry
	c.lru.Put(key, entry)
}

// InvalidateFile removes cache entries for a specific file.
func (c *ToolResultCache) InvalidateFile(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Invalidate read_file cache for this path
	readKey := c.GenerateReadFileKey(path)
	delete(c.entries, readKey)
	c.lru.Delete(readKey)

	// Invalidate list_dir cache for parent directory
	dir := filepath.Dir(path)
	listKey := c.GenerateListDirKey(dir)
	delete(c.entries, listKey)
	c.lru.Delete(listKey)

	// Also invalidate grep caches that might reference this path
	for key := range c.entries {
		if strings.Contains(key, path) {
			delete(c.entries, key)
			c.lru.Delete(key)
		}
	}
}

// InvalidateDir invalidates cache entries related to a specific directory
// This performs batch invalidation for all entries under the directory
func (c *ToolResultCache) InvalidateDir(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Normalize path
	path = strings.TrimSuffix(path, string(filepath.Separator))

	// Remove list_dir cache for this directory
	listKey := c.GenerateListDirKey(path)
	delete(c.entries, listKey)
	c.lru.Delete(listKey)

	// Remove all file caches under this directory
	for key := range c.entries {
		// Check if this key is for a file in the invalidated directory
		if strings.HasPrefix(key, "read_file:") {
			filePath := strings.TrimPrefix(key, "read_file:")
			if strings.HasPrefix(filePath, path+string(filepath.Separator)) {
				delete(c.entries, key)
				c.lru.Delete(key)
			}
		}
	}
}

// Clear removes all entries from the cache.
func (c *ToolResultCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*ToolCacheEntry)
	c.lru = NewLRUCache(1000)
	c.hitCount = 0
	c.missCount = 0
	c.evictionCount = 0
}

// Stats returns cache statistics
func (c *ToolResultCache) ToolCacheStats() (size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// Cleanup removes expired entries from the cache
func (c *ToolResultCache) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		maxAge := c.getMaxAgeFromKey(key)
		if now.Sub(entry.CachedAt) > maxAge {
			delete(c.entries, key)
			c.lru.Delete(key)
			c.evictionCount++
		}
	}
}

// getMaxAgeFromKey determines the max age based on the cache key
func (c *ToolResultCache) getMaxAgeFromKey(key string) time.Duration {
	if strings.HasPrefix(key, "read_file:") {
		return c.getMaxAge("read_file")
	}
	if strings.HasPrefix(key, "list_dir:") {
		return c.getMaxAge("list_dir")
	}
	if strings.HasPrefix(key, "grep:") {
		return c.getMaxAge("grep")
	}
	return c.maxAge
}
