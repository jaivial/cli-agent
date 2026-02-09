# EAI CLI Agent - Refactoring Review

## Summary of Changes Made

The improvements from `improvement-1.md` have been implemented comprehensively:

| Area | Status | Quality |
|------|--------|---------|
| System Prompt Expansion | ✅ Done | Good - Expanded from ~15 to ~180 lines |
| New Tools Added | ✅ Done | Good - 6 new tools (patch, append, background, http) |
| Dynamic Max Iterations | ✅ Done | Partial - Function exists but not fully integrated |
| Planning Phase | ✅ Done | Good - Added before main execution loop |
| Context Summarization | ✅ Done | Good - Reduces token usage on long tasks |
| Strategy Change Prompts | ✅ Done | Good - Injects guidance after repeated failures |
| Improved Convergence | ✅ Done | Good - Better tolerance for legitimate retries |
| Category Prompts | ✅ Done | Excellent - 12 specialized categories |
| Background Process Tools | ✅ Done | Good - Full manager implementation |
| VM/QEMU Handling | ✅ Done | Good - Extended timeouts and boot detection |

---

## Issues to Refactor

### Priority 1: Dead Code & Duplication

#### 1.1 Remove Unused `detectConvergence` Function
**File:** `internal/app/agent.go:876-896`
**Issue:** Old `detectConvergence` function exists but `detectConvergenceImproved` is used instead.
**Fix:** Delete the old function.

```go
// DELETE THIS - lines 876-896
func (l *AgentLoop) detectConvergence(lastPatterns []string, currentPattern string) bool {
    // ... old code
}
```

#### 1.2 Remove Unused `executeVMCommand` Method
**File:** `internal/app/agent.go:2194-2225`
**Issue:** Method defined but never called anywhere.
**Fix:** Either integrate it into the `exec` case or delete it.

#### 1.3 Consolidate Category Detection Functions
**File:** `internal/app/agent.go` and `internal/app/category_prompts.go`
**Issue:** Two similar functions exist:
- `GetTaskCategory()` in agent.go (lines 1029-1061)
- `detectCategory()` in category_prompts.go (lines 412-543)

**Fix:** Delete `GetTaskCategory` and use `detectCategory` everywhere, or make one call the other.

```go
// In agent.go, replace GetTaskCategory with:
func GetTaskCategory(task string) string {
    return detectCategory(task) // From category_prompts.go
}
```

#### 1.4 Remove Duplicate VM Detection Logic
**File:** `internal/app/agent.go`
**Issue:** `isVMCommand()` function exists (line 2186) but similar logic is also inline in `determineMaxLoops()` (lines 111-116).

**Fix:** Use `isVMCommand()` in `determineMaxLoops()`:
```go
func (l *AgentLoop) determineMaxLoops(task string) int {
    category := GetTaskCategory(task)
    switch category {
    // ...
    default:
        if isVMCommand(task) {
            return 20
        }
        return 12
    }
}
```

---

### Priority 2: Bug Fixes

#### 2.1 Fix Path Handling in `write_file`
**File:** `internal/app/agent.go:1691-1702`
**Issue:** Using `strings.LastIndex` without checking for -1 can cause issues.

**Current (buggy):**
```go
baseName := args.Path[strings.LastIndex(args.Path, "/")+1:]
// ...
dir := args.Path[:strings.LastIndex(args.Path, "/")]
```

**Fixed:**
```go
dir := filepath.Dir(args.Path)
baseName := filepath.Base(args.Path)

if err := os.MkdirAll(dir, 0755); err != nil {
    // handle error
}
```

#### 2.2 Fix Dynamic Max Loops Not Being Applied
**File:** `internal/app/agent.go:79-90`
**Issue:** `determineMaxLoops` is called inside `Execute()` but `l.MaxLoops` is already set in constructor.

**Current:**
```go
func NewAgentLoop(...) *AgentLoop {
    determinedMaxLoops := maxLoops  // This ignores determineMaxLoops
    return &AgentLoop{
        MaxLoops: determinedMaxLoops,
        // ...
    }
}
```

**Fixed:** Either:
1. Don't set MaxLoops in constructor, let Execute determine it
2. Or use 0 as a sentinel value meaning "auto-determine"

```go
func NewAgentLoop(client *MinimaxClient, maxLoops int, ...) *AgentLoop {
    return &AgentLoop{
        Client:    client,
        Tools:     DefaultTools(),
        MaxLoops:  maxLoops, // 0 means auto-determine
        // ...
    }
}

func (l *AgentLoop) Execute(ctx context.Context, task string) (*AgentState, error) {
    maxLoops := l.MaxLoops
    if maxLoops <= 0 {
        maxLoops = l.determineMaxLoops(task)
    }
    // ...
}
```

#### 2.3 Add HTTP Response Size Limit
**File:** `internal/app/agent.go:2066`
**Issue:** Reading full response body without size limit can cause OOM.

**Fix:**
```go
// Add size limit (e.g., 1MB)
const maxResponseSize = 1024 * 1024
body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
```

#### 2.4 Background Process Memory Leak
**File:** `internal/app/runner.go:82-88`
**Issue:** `globalProcessManager` never removes completed processes.

**Fix:** Add cleanup when process exits:
```go
// In ExecBackground, after process completes:
go func() {
    err := cmd.Wait()
    // ... existing code ...
    close(proc.Done)

    // Cleanup after a delay to allow final output reads
    time.AfterFunc(5*time.Minute, func() {
        globalProcessManager.Remove(proc.PID)
    })
}()
```

---

### Priority 3: Consistency Issues

#### 3.1 Inconsistent Path Handling
**Issue:** Some tools use `filepath.Dir()`, others use manual string slicing.

**Files affected:**
- `write_file` uses `strings.LastIndex`
- `append_file` uses `filepath.Dir`

**Fix:** Standardize on `filepath.Dir()` and `filepath.Base()` everywhere.

#### 3.2 Inconsistent Error Return Pattern
**Issue:** Some tool handlers use early returns, others use if-else chains.

**Example inconsistency:**
```go
// Pattern A (early return):
if err != nil {
    result.Error = ...
    return result
}

// Pattern B (if-else):
if err != nil {
    result.Error = ...
} else {
    result.Output = ...
}
```

**Fix:** Standardize on early returns for cleaner code.

#### 3.3 Missing Tool Documentation in System Prompt
**File:** `internal/app/system_prompt_enhanced.go`
**Issue:** New tools (patch_file, append_file, exec_background, wait_for_output, send_input, http_request) are not documented in the system prompt.

**Fix:** Add documentation for all new tools:
```go
### 8. patch_file - Apply unified diff patches
Format: {"tool": "patch_file", "args": {"path": "/file", "patch": "..."}}

### 9. append_file - Append to existing file
Format: {"tool": "append_file", "args": {"path": "/file", "content": "..."}}

### 10. exec_background - Start background process
Format: {"tool": "exec_background", "args": {"command": "..."}}
Returns PID for use with wait_for_output and send_input.

### 11. wait_for_output - Wait for pattern in process output
Format: {"tool": "wait_for_output", "args": {"pid": 123, "pattern": "regex"}}

### 12. send_input - Send input to background process
Format: {"tool": "send_input", "args": {"pid": 123, "input": "text"}}

### 13. http_request - Make HTTP requests
Format: {"tool": "http_request", "args": {"method": "GET", "url": "..."}}
```

---

### Priority 4: Architecture Improvements

#### 4.1 Unify Prompt Building Systems
**Issue:** Two separate systems exist:
1. `system_prompt_enhanced.go` - Returns a static enhanced prompt
2. `prompt.go` - Has dynamic PromptBuilder with category detection

These don't talk to each other.

**Fix:** Make `buildSystemMessageEnhanced()` use `PromptBuilder`:
```go
func (l *AgentLoop) buildSystemMessage() string {
    builder := NewPromptBuilder()
    // Get task from context or state
    return builder.SystemPromptWithTask(ModeDo, l.currentTask)
}
```

Or merge the enhanced prompt content into the PromptBuilder.

#### 4.2 Make Planning Phase Optional
**File:** `internal/app/agent.go:411-438`
**Issue:** Planning phase runs for ALL tasks, even simple ones.

**Fix:** Skip planning for simple tasks:
```go
// Only plan for complex tasks
_, complexity := (&PromptBuilder{}).ParseTaskForHints(task)
if complexity == "high" || complexity == "medium" {
    // Run planning phase
    planningPrompt := l.buildPlanningPrompt(task)
    // ...
}
```

#### 4.3 Add Tool Category to Validation
**Issue:** No validation that background tools (wait_for_output, send_input) receive valid PIDs.

**Fix:** Add PID validation:
```go
case "wait_for_output", "send_input":
    // Validate PID exists in process manager
    if _, ok := globalProcessManager.Get(args.PID); !ok {
        result.Error = fmt.Sprintf("No background process with PID %d", args.PID)
        return result
    }
```

---

### Priority 5: Performance Optimizations

#### 5.1 Optimize Context Summarization Check
**File:** `internal/app/agent.go:457-459`
**Issue:** `shouldSummarizeContext()` is called every iteration.

**Fix:** Cache the last known length or only check every N iterations:
```go
// Only check every 3 iterations
if state.Iteration % 3 == 0 && l.shouldSummarizeContext(state.Messages) {
    l.summarizeContext(state)
}
```

#### 5.2 Reuse HTTP Client
**File:** `internal/app/agent.go:2056`
**Issue:** Creates new `http.Client{}` for every request.

**Fix:** Create a package-level client with proper settings:
```go
var httpClient = &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        10,
        IdleConnTimeout:     30 * time.Second,
        DisableCompression:  false,
    },
}
```

#### 5.3 Add Caching for New File Tools
**Issue:** `append_file` and `patch_file` don't invalidate cache, but they modify files.

**Fix:** Already partially done for patch_file, but verify cache invalidation is called consistently:
```go
case "append_file":
    // ... write logic ...
    if l.ToolCache != nil {
        l.ToolCache.InvalidateFile(args.Path)
    }
```

---

### Priority 6: Testing Requirements

#### 6.1 Add Unit Tests for Patch Application
**File:** Create `internal/app/agent_test.go`

```go
func TestApplyUnifiedPatch(t *testing.T) {
    tests := []struct {
        name     string
        content  string
        patch    string
        expected string
        wantErr  bool
    }{
        {
            name:    "simple add",
            content: "line1\nline2\n",
            patch:   "@@ -1,2 +1,3 @@\n line1\n+line1.5\n line2\n",
            expected: "line1\nline1.5\nline2\n",
        },
        // Add more test cases...
    }
    // ...
}
```

#### 6.2 Add Tests for Background Process Manager
```go
func TestBackgroundProcessManager(t *testing.T) {
    manager := NewBackgroundProcessManager()

    // Test Add/Get/Remove cycle
    proc := &BackgroundProcess{PID: 12345}
    manager.Add(proc)

    got, ok := manager.Get(12345)
    if !ok || got.PID != 12345 {
        t.Error("Get failed")
    }

    manager.Remove(12345)
    _, ok = manager.Get(12345)
    if ok {
        t.Error("Remove failed")
    }
}
```

#### 6.3 Add Integration Tests for New Tools
```go
func TestExecBackground(t *testing.T) {
    runner := NewRunner(nil, "")
    proc, err := runner.ExecBackground("sleep 2 && echo done")
    if err != nil {
        t.Fatal(err)
    }

    // Wait for output
    matched, output, err := runner.WaitForOutput(proc.PID, "done", 5)
    if !matched {
        t.Errorf("Expected pattern match, got: %s", output)
    }
}
```

---

## Refactoring Checklist

### Must Fix (Blocking Issues)
- [ ] Fix `write_file` path handling panic potential
- [ ] Remove dead `detectConvergence` function
- [ ] Add HTTP response size limit
- [ ] Document new tools in system prompt

### Should Fix (Quality Issues)
- [ ] Consolidate category detection functions
- [ ] Fix dynamic max loops integration
- [ ] Add background process cleanup
- [ ] Standardize path handling with `filepath` package
- [ ] Make planning phase conditional

### Nice to Have (Improvements)
- [ ] Unify prompt building systems
- [ ] Add HTTP client connection pooling
- [ ] Optimize context summarization frequency
- [ ] Add comprehensive test coverage

---

## Code Quality Metrics

| Metric | Before | After | Target |
|--------|--------|-------|--------|
| Lines of code (agent.go) | ~1,270 | ~2,226 | - |
| Tool count | 7 | 13 | 13 |
| Category prompts | 6 | 12 | 12 |
| Dead code functions | 0 | 2 | 0 |
| Duplicated functions | 0 | 2 | 0 |
| Test coverage | ~0% | ~0% | >50% |

---

## Estimated Effort

| Task | Effort | Impact |
|------|--------|--------|
| Remove dead code | 30 min | Low |
| Fix path handling bugs | 1 hr | High |
| Add tool documentation | 1 hr | Medium |
| Consolidate category detection | 1 hr | Medium |
| Add basic tests | 2-3 hr | High |
| Unify prompt systems | 2-3 hr | Medium |
| Performance optimizations | 1-2 hr | Low |

**Total estimated effort: 8-12 hours**
