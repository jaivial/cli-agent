# EAI CLI Agent - Refactoring Review (Round 2)

## Summary of Fixes Applied from refactor-1.md

| Issue | Status | Notes |
|-------|--------|-------|
| Remove dead `detectConvergence` | ✅ Fixed | Function removed |
| Remove dead `executeVMCommand` | ✅ Fixed | Function removed |
| Consolidate `GetTaskCategory` | ✅ Fixed | Now calls `detectCategory()` |
| Fix path handling in `write_file` | ✅ Fixed | Uses `filepath.Dir/Base` |
| Add HTTP response size limit | ✅ Fixed | 1MB limit added |
| Reuse HTTP client | ✅ Fixed | Package-level client |
| Document new tools in system prompt | ✅ Fixed | All 13 tools documented |
| Make planning phase conditional | ✅ Fixed | Only for medium/high complexity |
| Optimize context summarization check | ✅ Fixed | Every 3 iterations |
| Add PID validation | ✅ Fixed | In wait_for_output and send_input |
| Add background process cleanup | ✅ Fixed | 5-minute delayed cleanup |
| Standardize error handling | ✅ Partially | Most tools use early returns |

---

## Remaining Issues to Refactor

### Priority 1: Orphaned Code & Integration Issues

#### 1.1 `buildSystemMessageEnhanced` is Orphaned
**File:** `internal/app/system_prompt_enhanced.go`
**Issue:** The comprehensive 230-line system prompt is never used. `buildSystemMessage(task)` now calls `PromptBuilder.SystemPromptWithTask()` which returns a much simpler ~20-line prompt.

**Current flow:**
```
buildSystemMessage(task) → PromptBuilder.SystemPromptWithTask() → simple prompt
buildSystemMessageEnhanced() → never called → 230 lines wasted
```

**Impact:** High - The detailed tool documentation, error handling patterns, and best practices are not being sent to the model.

**Fix options:**
1. **Option A (Recommended):** Make `PromptBuilder.SystemPrompt()` return the enhanced prompt:
```go
func (p *PromptBuilder) SystemPrompt(mode Mode, categoryHints ...string) string {
    // Use the enhanced base prompt instead of the simple one
    basePrompt := buildSystemMessageEnhancedContent()
    // Add mode-specific and category guidance
    // ...
}
```

2. **Option B:** Have `buildSystemMessage` call `buildSystemMessageEnhanced` instead:
```go
func (l *AgentLoop) buildSystemMessage(task string) string {
    base := l.buildSystemMessageEnhanced()
    category := detectCategory(task)
    if prompt, ok := categoryPrompts[category]; ok {
        return base + "\n\n## TASK-SPECIFIC GUIDANCE\n" + prompt
    }
    return base
}
```

#### 1.2 Unused Functions in `category_prompts.go`
**File:** `internal/app/category_prompts.go`

Functions defined but never called:
- `composePrompts()` (line 582)
- `buildSystemMessageWithCategories()` (line 614)

**Fix:** Either integrate these into the prompt building flow or delete them.

#### 1.3 Inconsistent Tool Argument Names in Documentation
**File:** `internal/app/system_prompt_enhanced.go:86-91`

**Current (incorrect):**
```go
### 7. edit_file - Modify existing files
Format: {"tool": "edit_file", "args": {"path": "/path", "old": "text to replace", "new": "replacement"}}
```

**Actual tool expects:** `old_text` and `new_text`

**Fix:**
```go
Format: {"tool": "edit_file", "args": {"path": "/path", "old_text": "text to replace", "new_text": "replacement"}}
```

---

### Priority 2: Code Quality Issues

#### 2.1 `append_file` Still Uses Nested If-Else
**File:** `internal/app/agent.go:1881-1919`

Should be converted to early returns for consistency with other tools.

**Current:**
```go
case "append_file":
    // ... parse args ...
    if err := os.MkdirAll(dir, 0755); err != nil {
        result.Error = ...
    } else {
        f, err := os.OpenFile(...)
        if err != nil {
            result.Error = ...
        } else {
            defer f.Close()
            if _, err := f.WriteString(...); err != nil {
                result.Error = ...
            } else {
                // success
            }
        }
    }
```

**Fix (early returns):**
```go
case "append_file":
    // ... parse args ...
    dir := filepath.Dir(args.Path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        result.Error = fmt.Sprintf("Failed to create directory: %v", err)
        result.DurationMs = time.Since(start).Milliseconds()
        return result
    }

    f, err := os.OpenFile(args.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        result.Error = fmt.Sprintf("Failed to open file: %v", err)
        result.DurationMs = time.Since(start).Milliseconds()
        return result
    }
    defer f.Close()

    if _, err := f.WriteString(args.Content); err != nil {
        result.Error = fmt.Sprintf("Failed to append content: %v", err)
        result.DurationMs = time.Since(start).Milliseconds()
        return result
    }

    stat, _ := f.Stat()
    if stat != nil {
        result.Output = fmt.Sprintf("Content appended to: %s (new size: %d bytes)", args.Path, stat.Size())
    } else {
        result.Output = fmt.Sprintf("Content appended to: %s", args.Path)
    }
    result.Success = true
    if l.ToolCache != nil {
        l.ToolCache.InvalidateFile(args.Path)
    }
```

#### 2.2 Race Condition in `RunVMWithStreaming`
**File:** `internal/app/runner.go:251-282`

**Issue:** `outputBuilder` is accessed from multiple goroutines without synchronization.

**Current:**
```go
var outputBuilder strings.Builder
done := make(chan bool)

go func() {
    for scanner.Scan() {
        outputBuilder.WriteString(line + "\n")  // Race!
        // ...
    }
}()

go func() {
    for scanner.Scan() {
        outputBuilder.WriteString(line + "\n")  // Race!
        // ...
    }
}()
```

**Fix:** Add mutex protection:
```go
var outputBuilder strings.Builder
var outputMu sync.Mutex
done := make(chan bool)

go func() {
    for scanner.Scan() {
        outputMu.Lock()
        outputBuilder.WriteString(line + "\n")
        outputMu.Unlock()
        // ...
    }
}()
```

#### 2.3 Missing DurationMs Assignment
**File:** `internal/app/agent.go:1871-1879`

In `patch_file`, on success path, `result.DurationMs` is set at the end of the switch but the early error returns properly set it. However, this is actually correct as written (line 2087 sets it). No fix needed.

---

### Priority 3: Architecture Improvements

#### 3.1 Consolidate Prompt Building
**Issue:** Three separate prompt-building mechanisms exist:
1. `system_prompt_enhanced.go` - `buildSystemMessageEnhanced()`
2. `category_prompts.go` - `buildSystemMessageWithCategories()`
3. `prompt.go` - `PromptBuilder.SystemPrompt()`

**Recommendation:** Consolidate into a single, well-structured approach:

```go
// In prompt.go, update SystemPrompt to use enhanced content:
func (p *PromptBuilder) SystemPrompt(mode Mode, categoryHints ...string) string {
    // Start with the comprehensive enhanced prompt
    basePrompt := getEnhancedSystemPrompt()

    // Add mode-specific section
    modePrompt := p.getModePrompt(mode)

    // Add category-specific guidance
    categoryGuidance := p.buildCategoryGuidance(categoryHints)

    return basePrompt + modePrompt + categoryGuidance
}
```

#### 3.2 Extract Constants
**File:** `internal/app/agent.go`

Magic numbers should be extracted to named constants:

```go
const (
    DefaultTimeout        = 30 * time.Second
    VMTimeout            = 300 * time.Second
    MaxHTTPResponseSize  = 1024 * 1024  // 1MB
    ContextSummarizeThreshold = 20000   // ~5000 tokens
    MaxStallCount        = 6
    ToolCacheExpiry      = 5 * time.Minute
    ProcessCleanupDelay  = 5 * time.Minute
)
```

#### 3.3 Simplify Category Detection Return
**File:** `internal/app/category_prompts.go:524-543`

The priority loop is overly complex. Simplify:

```go
func detectCategory(task string) string {
    // ... detection logic ...

    if len(categories) == 0 {
        return "default"
    }

    // Return first match from priority order
    priority := []string{
        "git_advanced", "sqlite_advanced", "ml_recovery",
        "polyglot_build", "security", "qemu", "vm",
        "git", "database", "ml", "build", "devops",
    }

    categorySet := make(map[string]bool)
    for _, c := range categories {
        categorySet[c] = true
    }

    for _, p := range priority {
        if categorySet[p] {
            return p
        }
    }
    return categories[0]
}
```

---

### Priority 4: Testing Requirements

#### 4.1 Unit Tests for Tool Execution
**File:** Create `internal/app/agent_test.go`

```go
func TestExecuteTool_WriteFile(t *testing.T) {
    // Test write_file creates file correctly
}

func TestExecuteTool_AppendFile(t *testing.T) {
    // Test append_file appends correctly
}

func TestExecuteTool_PatchFile(t *testing.T) {
    // Test patch application
}

func TestExecuteTool_HTTPRequest(t *testing.T) {
    // Test HTTP requests with mock server
}
```

#### 4.2 Unit Tests for Category Detection
```go
func TestDetectCategory(t *testing.T) {
    tests := []struct {
        task     string
        expected string
    }{
        {"git reflog show HEAD", "git_advanced"},
        {"git commit -m 'test'", "git"},
        {"qemu-system-x86_64 boot alpine", "qemu"},
        {"sqlite3 truncate table", "sqlite_advanced"},
        {"build rust c ffi", "polyglot_build"},
    }

    for _, tt := range tests {
        t.Run(tt.task, func(t *testing.T) {
            got := detectCategory(tt.task)
            if got != tt.expected {
                t.Errorf("detectCategory(%q) = %q, want %q", tt.task, got, tt.expected)
            }
        })
    }
}
```

#### 4.3 Integration Tests for Background Processes
```go
func TestBackgroundProcess_Lifecycle(t *testing.T) {
    runner := NewRunner(nil, "")

    // Start process
    proc, err := runner.ExecBackground("echo hello && sleep 1 && echo done")
    require.NoError(t, err)
    require.NotNil(t, proc)

    // Wait for output
    matched, output, err := runner.WaitForOutput(proc.PID, "done", 5)
    require.NoError(t, err)
    require.True(t, matched)
    require.Contains(t, output, "hello")

    // Verify cleanup happens
    <-proc.Done
    time.Sleep(100 * time.Millisecond)
    // Process should still be accessible
    _, ok := globalProcessManager.Get(proc.PID)
    require.True(t, ok)
}
```

---

### Priority 5: Documentation

#### 5.1 Add GoDoc Comments
Several exported functions lack documentation:

```go
// GetTaskCategory returns the detected category for a task description.
// Categories include: git, git_advanced, build, polyglot_build, devops,
// vm, qemu, ml, ml_recovery, database, sqlite_advanced, security, default.
func GetTaskCategory(task string) string {
    return detectCategory(task)
}

// DefaultTools returns the standard set of tools available to the agent.
// This includes file operations (read, write, edit, append, patch),
// execution tools (exec, exec_background), process management
// (wait_for_output, send_input), and HTTP requests.
func DefaultTools() []Tool {
    // ...
}
```

---

## Refactoring Checklist

### Must Fix (High Impact)
- [ ] Integrate enhanced system prompt into PromptBuilder (1.1)
- [ ] Fix edit_file argument names in documentation (1.3)
- [ ] Fix race condition in RunVMWithStreaming (2.2)

### Should Fix (Medium Impact)
- [ ] Remove unused functions from category_prompts.go (1.2)
- [ ] Convert append_file to early returns (2.1)
- [ ] Extract magic numbers to constants (3.2)

### Nice to Have (Low Impact)
- [ ] Consolidate prompt building systems (3.1)
- [ ] Simplify category detection return (3.3)
- [ ] Add comprehensive test coverage (4.x)
- [ ] Add GoDoc comments (5.1)

---

## Code Metrics Comparison

| Metric | After refactor-1 | Target |
|--------|------------------|--------|
| Dead code functions | 0 | 0 ✓ |
| Unused functions | 2 | 0 |
| Race conditions | 1 | 0 |
| Inconsistent error handling | 1 tool | 0 |
| Test coverage | ~0% | >50% |
| Documentation coverage | ~20% | >80% |

---

## Estimated Effort

| Task | Effort | Impact |
|------|--------|--------|
| Integrate enhanced prompt (1.1) | 1-2 hr | High |
| Fix argument names (1.3) | 15 min | Medium |
| Fix race condition (2.2) | 30 min | High |
| Remove unused functions (1.2) | 15 min | Low |
| Convert append_file (2.1) | 30 min | Low |
| Extract constants (3.2) | 30 min | Low |
| Add tests (4.x) | 3-4 hr | High |

**Total estimated effort: 6-8 hours**

---

## Quick Wins (Can Do Now)

1. **Fix edit_file documentation** - 5 minutes
2. **Remove unused functions** - 10 minutes
3. **Add mutex to RunVMWithStreaming** - 15 minutes

These three changes can be done in under 30 minutes and would resolve 3 of the remaining issues.
