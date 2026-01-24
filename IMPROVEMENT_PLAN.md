# CLI AGENT IMPROVEMENT PLAN
**Generated:** 2026-01-24 19:25 GMT+1  
**Goal:** Achieve terminal-first CLI agent with MiniMax 2.1 API, benchmark ~70%

---

## PHASE 1: CORE FUNCTIONALITY (Make it work)

### 1.1 Tool Calling System
**Priority:** HIGH  
**Status:** ✅ IMPLEMENTED

**Implemented:**
- Tool schema definitions (exec, read_file, write_file, list_dir, search_files, grep, web_search, web_fetch)
- Tool dispatcher with JSON argument parsing
- Tool result formatting for model consumption
- Tool calling integration with MiniMax API

**Files modified:**
- `internal/app/agent.go` - Complete tool system implementation
- `cmd/eai/main.go` - Agent command integration

### 1.2 Agent Loop (Ralph Loop)
**Priority:** HIGH  
**Status:** ✅ IMPLEMENTED

**Implemented:**
- Iterative improvement loop with max iterations safeguard
- Self-reflection between iterations (tool results fed back to model)
- Checkpointing for recovery (state saved to disk)
- Progress tracking (iteration count, tool execution count)
- State persistence across sessions

**Code structure:**
```go
type AgentLoop struct {
    MaxIterations int
    Tools         []Tool
    Client        *MinimaxClient
    State         AgentState
}

func (l *AgentLoop) Run(ctx context.Context, task string) error {
    for iteration := 0; iteration < l.MaxIterations; iteration++ {
        response, err := l.plan(ctx, task)
        if response.Done {
            return nil
        }
        if err := l.executeTools(ctx, response.Tools); err != nil {
            return err
        }
    }
    return errors.New("max iterations reached")
}
```

### 1.3 MiniMax API Streaming
**Priority:** MEDIUM  
**Status:** NOT IMPLEMENTED

**Needed:**
- [ ] Add SSE (Server-Sent Events) support
- [ ] Implement streaming response handler
- [ ] Update TUI for real-time display
- [ ] Add streaming toggle in config

---

## PHASE 2: TERMINAL INTEGRATION

### 2.1 Interactive Command Execution
**Priority:** HIGH

**Needed:**
- [ ] PTY support for interactive commands (ssh, vim, etc.)
- [ ] Capture both stdout and stderr
- [ ] Handle terminal control sequences
- [ ] Add timeout and cancellation support

**Code:**
```go
type PTYRunner struct {
    timeout time.Duration
}

func (r *PTYRunner) Run(ctx context.Context, cmd string) (string, error) {
    // Use github.com/creack/pty for full terminal emulation
}
```

### 2.2 Shell Completion Detection
**Priority:** MEDIUM

**Needed:**
- [ ] Detect shell completion signals
- [ ] Parse shell command outputs
- [ ] Handle multi-line command responses

### 2.3 Process Lifecycle Management
**Priority:** MEDIUM

**Needed:**
- [ ] Background process support (already in JobStore)
- [ ] Process tree killing
- [ ] Resource cleanup

---

## PHASE 3: PERSISTENCE & STATE

### 3.1 Chat History Storage
**Priority:** MEDIUM

**Needed:**
- [ ] Save conversation to file
- [ ] Load history on startup
- [ ] Support multiple sessions/workspaces
- [ ] Export/import functionality

### 3.2 Configuration Management
**Priority:** LOW

**Needed:**
- [ ] YAML config file (already exists)
- [ ] Environment variable overrides
- [ ] Command-line flags
- [ ] Interactive config wizard

---

## PHASE 4: BENCHMARK OPTIMIZATION

### 4.1 Performance Metrics
**Priority:** LOW

**Needed:**
- [ ] Track token usage
- [ ] Measure response latency
- [ ] Log success/failure rates
- [ ] Generate performance reports

### 4.2 Prompt Optimization
**Priority:** MEDIUM

**Needed:**
- [ ] Optimize system prompts for MiniMax
- [ ] Add few-shot examples
- [ ] Implement prompt caching
- [ ] Fine-tune for different modes

### 4.3 Error Handling & Retry
**Priority:** MEDIUM

**Needed:**
- [ ] Exponential backoff for API calls
- [ ] Graceful degradation on failures
- [ ] User-friendly error messages
- [ ] Recovery mechanisms

---

## PRIORITY ORDER (70% Benchmark Target)

### Must Have (80% of value):
1. ✅ Basic TUI and API client (already done)
2. **Tool calling system** ← NEXT
3. **Agent loop with iterations**
4. PTY/interactive command support
5. Error handling and retry logic

### Should Have (15% of value):
6. Streaming support
7. Chat history persistence
8. Performance metrics

### Nice to Have (5% of value):
9. Configuration wizard
10. Export/import
11. Advanced prompt optimization

---

## IMPLEMENTATION ROADMAP

### Step 1: Tool Definitions (`internal/app/tools.go`)
```go
type Tool struct {
    Name        string
    Description string
    Parameters  []Parameter
}

var AvailableTools = []Tool{
    {
        Name: "exec",
        Description: "Execute a shell command",
        Parameters: []Parameter{
            {Name: "command", Type: "string", Required: true},
        },
    },
    {
        Name: "read_file",
        Description: "Read file contents",
        Parameters: []Parameter{
            {Name: "path", Type: "string", Required: true},
        },
    },
    // ... more tools
}
```

### Step 2: Tool Calling Loop (`internal/app/agent.go`)
```go
type Agent struct {
    Client     *MinimaxClient
    Tools      []Tool
    MaxLoops   int
    StateDir   string
}

func (a *Agent) Execute(ctx context.Context, task string) error {
    messages := []minimaxMessage{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: task},
    }
    
    for loop := 0; loop < a.MaxLoops; loop++ {
        response := a.sendWithTools(ctx, messages)
        
        if len(response.ToolCalls) == 0 {
            // No more tools, we're done
            break
        }
        
        for _, call := range response.ToolCalls {
            result := a.executeTool(call)
            messages = append(messages, minimaxMessage{
                Role: "user",
                Content: fmt.Sprintf("Tool result: %s", result),
            })
        }
    }
    return nil
}
```

### Step 3: PTY Support (`internal/app/pty.go`)
```go
func RunInteractive(ctx context.Context, cmd string) (string, error) {
    proc, err := pty.StartWithSize(cmd, ...)
    // Handle full terminal interaction
}
```

---

## TESTING PLAN

### Unit Tests
- [ ] Tool dispatcher tests
- [ ] Agent loop tests
- [ ] API client tests
- [ ] PTY runner tests

### Integration Tests
- [ ] Full agent workflow test
- [ ] Tool execution sequence test
- [ ] Error recovery test
- [ ] Benchmark test (70% target)

### Manual Tests
- [ ] Interactive command execution
- [ ] Multi-step task completion
- [ ] Performance under load

---

## ESTIMATED EFFORT

- **Phase 1 (Core):** 4-6 hours
- **Phase 2 (Terminal):** 2-4 hours
- **Phase 3 (Persistence):** 1-2 hours
- **Phase 4 (Benchmark):** 2-3 hours

**Total:** 9-15 hours

---

*This plan will be executed iteratively. Each step will be implemented and tested before moving to the next.*
