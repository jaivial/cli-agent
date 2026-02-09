# EAI CLI Agent - Terminal-Bench 2.0 Improvement Plan

## Current Performance Analysis

| Benchmark Run | Success Rate | Passed | Failed |
|--------------|--------------|--------|--------|
| Official (enhanced prompts) | 68.54% | 61/89 | 28 |
| Final v3 (specialized prompts) | 70.79% | 63/89 | 26 |

**Target: Stabilize at 70%+ and push toward 75-80%**

---

## Consistently Failing Tasks

These tasks failed in multiple benchmark runs:

| Task | Category | Root Cause |
|------|----------|------------|
| `caffe-cifar-10` | ML/Build | Complex dependency chain, GPU requirements |
| `crack-7z-hash` | Security | Requires hashcat/john, specific password patterns |
| `distribution-search` | Algorithm | Statistical algorithm implementation |
| `git-leak-recovery` | Git | Advanced reflog/fsck recovery |
| `git-multibranch` | Git | Complex branch operations, merge conflicts |
| `install-windows-3.11` | VM | QEMU with DOS emulation, interactive |
| `make-mips-interpreter` | Systems | Complex interpreter implementation |
| `merge-diff-arc-agi-task` | AI/Pattern | ARC-AGI pattern recognition |
| `model-extraction-relu-logits` | ML | Neural network reverse engineering |
| `polyglot-rust-c` | Build | Dual-language compilation |
| `pytorch-model-recovery` | ML | Corrupted model state recovery |
| `qemu-alpine-ssh` | VM | QEMU boot + SSH tunnel setup |
| `query-optimize` | Database | SQL query optimization |
| `sqlite-db-truncate` | Database | Proper truncation without corruption |

---

## Improvement Tasks

### Priority 1: System Prompt Enhancement

#### Task 1.1: Expand Core System Prompt
**File:** `internal/app/system_prompt_enhanced.go`
**Impact:** High (affects all tasks)

Current prompt is only ~15 lines. Expand to include:
- Detailed tool usage examples with edge cases
- Error recovery patterns (what to do when a command fails)
- Multi-step task decomposition guidance
- Output verification instructions
- File path conventions (always use absolute paths)

```go
// Example structure:
// 1. Role definition (expert CLI agent)
// 2. Tool reference with examples
// 3. Error handling patterns
// 4. Verification steps
// 5. Common pitfalls to avoid
```

#### Task 1.2: Add Task-Specific Micro-Prompts
**File:** `internal/app/category_prompts.go`
**Impact:** Medium-High

Add specialized prompts for failing categories:
- **VM/QEMU tasks**: Timeout handling, expect scripts, non-interactive modes
- **Advanced Git**: reflog, fsck, cherry-pick, rebase strategies
- **ML Recovery**: Model state inspection, tensor debugging
- **Polyglot builds**: Multi-toolchain coordination
- **Security tasks**: Hash formats, wordlist usage

#### Task 1.3: Implement Dynamic Prompt Selection
**File:** `internal/app/prompt.go`
**Impact:** Medium

Improve category detection beyond simple keyword matching:
- Parse task instructions for semantic meaning
- Detect compound tasks (e.g., "build AND test")
- Allow prompt composition for multi-category tasks

---

### Priority 2: Tool System Expansion

#### Task 2.1: Add Patch/Diff Tool
**File:** `internal/app/agent.go` (DefaultTools function)
**Impact:** High

Add a `patch_file` tool for applying unified diffs:
```json
{
  "name": "patch_file",
  "description": "Apply a unified diff patch to a file",
  "parameters": {
    "path": "file to patch",
    "patch": "unified diff content"
  }
}
```

This would help with:
- `large-scale-text-editing`
- `merge-diff-arc-agi-task`
- Complex multi-edit tasks

#### Task 2.2: Add Append File Tool
**File:** `internal/app/agent.go`
**Impact:** Medium

Add `append_file` for log files and incremental writes:
```json
{
  "name": "append_file",
  "description": "Append content to end of file",
  "parameters": {
    "path": "file path",
    "content": "content to append"
  }
}
```

#### Task 2.3: Add Process Management Tools
**File:** `internal/app/agent.go`
**Impact:** High (VM tasks)

Add tools for background processes:
```json
{
  "name": "exec_background",
  "description": "Start a background process, returns PID"
},
{
  "name": "wait_for_output",
  "description": "Wait for specific output pattern from a process"
},
{
  "name": "send_input",
  "description": "Send input to a running process"
}
```

Critical for:
- `qemu-alpine-ssh`
- `qemu-startup`
- `install-windows-3.11`

#### Task 2.4: Add HTTP/Network Tool
**File:** `internal/app/agent.go`
**Impact:** Medium

Add basic HTTP capability:
```json
{
  "name": "http_request",
  "description": "Make HTTP request (GET/POST)",
  "parameters": {
    "method": "GET|POST",
    "url": "target URL",
    "body": "optional request body"
  }
}
```

Useful for:
- `pypi-server` verification
- `nginx-request-logging` testing
- API testing tasks

---

### Priority 3: Agent Loop Improvements

#### Task 3.1: Increase Max Iterations
**File:** `internal/app/agent.go`
**Impact:** Medium

Current: 10 iterations max
Proposed: 15-20 iterations for complex tasks

Add dynamic iteration limits based on task complexity:
```go
func (l *AgentLoop) determineMaxLoops(task string) int {
    // Simple tasks: 10
    // Build tasks: 15
    // VM/multi-step: 20
}
```

#### Task 3.2: Implement Retry with Strategy Change
**File:** `internal/app/agent.go`
**Impact:** High

When a tool fails repeatedly, inject a "strategy change" prompt:
```
"Previous approach failed 3 times. Try a completely different method:
- If install failed, try from source
- If build failed, check dependencies
- If file not found, search for it"
```

#### Task 3.3: Add Planning Phase
**File:** `internal/app/agent.go`
**Impact:** High

Before execution, add a planning step:
1. Parse task into sub-goals
2. Identify required tools
3. Estimate complexity
4. Create execution checklist

This reduces wasted iterations on complex tasks.

#### Task 3.4: Improve Convergence Detection
**File:** `internal/app/agent.go` (detectConvergence function)
**Impact:** Medium

Current: Terminates after 4 identical patterns
Issues: May terminate valid retry loops

Improvements:
- Track output changes, not just tool patterns
- Allow "retry with modification" (same tool, different args)
- Increase tolerance for write/read cycles

#### Task 3.5: Add Context Summarization
**File:** `internal/app/agent.go`
**Impact:** Medium

For long conversations (>5000 tokens), summarize earlier context:
- Keep system prompt intact
- Summarize tool results to key outcomes
- Preserve error messages and fixes

---

### Priority 4: Task-Specific Fixes

#### Task 4.1: QEMU/VM Task Handler
**Files:** `internal/app/agent.go`, `internal/app/runner.go`
**Impact:** High (3+ failing tasks)

Create specialized handling for VM tasks:
- Extended timeout (300s+)
- Output streaming
- Port forwarding detection
- Boot completion detection

#### Task 4.2: Git Advanced Operations Prompt
**File:** `internal/app/category_prompts.go`
**Impact:** Medium (2 failing tasks)

Add comprehensive git recovery prompt:
```
Git Recovery Expert:
- reflog: git reflog to find lost commits
- fsck: git fsck --lost-found for dangling objects
- cherry-pick: git cherry-pick <sha> to recover commits
- Reset: git reset --hard vs --soft
- Stash: git stash list/pop/apply
```

#### Task 4.3: SQLite Operations Prompt
**File:** `internal/app/category_prompts.go`
**Impact:** Medium (2 failing tasks)

```
SQLite Expert:
- TRUNCATE: DELETE FROM table; (no TRUNCATE in SQLite)
- Vacuum: VACUUM to reclaim space
- WAL mode: PRAGMA journal_mode for WAL operations
- Backup: .backup command or sqlite3_backup API
```

#### Task 4.4: ML/PyTorch Recovery Prompt
**File:** `internal/app/category_prompts.go`
**Impact:** Medium

```
PyTorch Recovery:
- torch.load with map_location='cpu' for GPU models
- Inspect state_dict keys before loading
- Handle version mismatches
- Recover partial checkpoints
```

---

### Priority 5: Testing and Validation

#### Task 5.1: Add Unit Tests for Tool Parsing
**File:** `internal/app/agent_test.go` (create)
**Impact:** Medium

Test cases for:
- All JSON formats (3 supported formats)
- Malformed JSON recovery
- Tool name normalization
- Argument validation

#### Task 5.2: Add Integration Tests for Failing Tasks
**File:** `tests/failing_tasks_test.go` (create)
**Impact:** High

Create focused tests for consistently failing tasks:
- Mock the expected interaction patterns
- Verify correct tool sequences
- Test error recovery paths

#### Task 5.3: Add Benchmark Regression Tests
**File:** `tbench2_regression.py` (create)
**Impact:** Medium

Track benchmark results over time:
- Store historical results
- Alert on regressions
- Track per-task success rates

---

### Priority 6: Performance Optimizations

#### Task 6.1: Parallel Tool Execution Tuning
**File:** `internal/app/agent.go` (executeToolsParallel)
**Impact:** Low-Medium

Current: 5 max workers
Consider: Dynamic worker count based on tool types
- File reads: high parallelism (10+)
- Exec commands: conservative (3-5)
- Write operations: sequential

#### Task 6.2: Cache Optimization
**File:** `internal/app/cache.go`
**Impact:** Low

- Increase cache duration for stable files
- Add cache warming for common paths
- Implement LRU eviction for large caches

---

## Implementation Roadmap

### Phase 1 (Quick Wins) - 2-3% improvement expected
1. Expand system prompt (Task 1.1)
2. Add VM/QEMU-specific prompts (Task 1.2)
3. Increase max iterations (Task 3.1)

### Phase 2 (Tool Expansion) - 3-5% improvement expected
1. Add patch_file tool (Task 2.1)
2. Add background process tools (Task 2.3)
3. Implement retry with strategy change (Task 3.2)

### Phase 3 (Agent Intelligence) - 2-4% improvement expected
1. Add planning phase (Task 3.3)
2. Improve convergence detection (Task 3.4)
3. Add context summarization (Task 3.5)

### Phase 4 (Task-Specific) - 2-3% improvement expected
1. Git recovery prompt (Task 4.2)
2. SQLite operations prompt (Task 4.3)
3. ML recovery prompt (Task 4.4)

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Overall success rate | 68-70% | 75%+ |
| VM tasks (qemu-*) | 0-50% | 75%+ |
| Git advanced tasks | 50% | 80%+ |
| Build tasks | 70% | 85%+ |
| Database tasks | 60% | 80%+ |

---

## Code Change Summary

| File | Changes |
|------|---------|
| `internal/app/system_prompt_enhanced.go` | Expand from ~15 to ~100 lines |
| `internal/app/category_prompts.go` | Add 5+ new category prompts |
| `internal/app/agent.go` | Add 3 new tools, improve loop logic |
| `internal/app/runner.go` | Add background process support |
| `internal/app/cache.go` | Optimize caching strategy |
| `internal/app/prompt.go` | Dynamic prompt composition |
