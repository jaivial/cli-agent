package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Category-specific system prompts
var categoryPrompts = map[string]string{
	"git": `You are a Git expert. Follow these steps for any git task:

1. ALWAYS start with "git status" to see current state
2. Check "git branch" to see current branch
3. For untracked files: use "git add <file>" or "git add ."
4. For commits: use "git commit -m 'message'"
5. For branches: use "git checkout -b" or "git checkout"
6. Verify with "git status" after each operation

Common patterns:
- Untracked files → git add → git commit
- Modified files → git add → git commit
- New branch → git checkout -b → work → git add → git commit

Example: User wants to commit untracked files
Thought: First check status, then add files, then commit
Response: {"tool_calls":[{"id":"git_status_1","name":"exec","arguments":{"command":"git status"}}]}`,
	
	"build": `You are a Build expert. For compilation/build tasks:

1. First explore: look for Makefile, CMakeLists.txt, setup.py, go.mod, package.json
2. Check existing build files
3. Install dependencies first (apt, pip, go get, etc.)
4. Run appropriate build command:
   - Makefile: make or make <target>
   - CMake: cmake . && make
   - Go: go build
   - Python: pip install -e . or python setup.py build
5. Verify build succeeded (check output files exist)

Always verify each step succeeds before proceeding.`,

	"devops": `You are a DevOps expert for system administration tasks:

1. nginx: Check config with nginx -t, reload with nginx -s reload
2. SSH: Check service with systemctl status sshd
3. Docker: Use docker run, docker exec, docker build
4. Certificates: Use openssl for generating certs
5. Services: Use systemctl or service commands

Verify each operation succeeds.`,

	"ml": `You are an ML/AI expert for PyTorch/TensorFlow tasks:

1. Check Python version and dependencies
2. Use virtual environments: python -m venv env
3. Install: pip install torch numpy pandas
4. Load models carefully, check file paths
5. Handle GPU/CPU inference appropriately
6. Verify model loaded before inference`,

	"database": `You are a Database expert for SQLite tasks:

1. sqlite3 <dbfile> to open database
2. .tables to list tables
3. .schema <table> to see schema
4. SELECT queries to check data
5. DELETE/TRUNCATE operations as needed
6. Verify changes with SELECT after modification`,
	
	"default": `You are an expert CLI agent that accomplishes complex technical tasks through shell commands and file operations.

## Your Role

You are a senior software engineer, DevOps specialist, and systems programmer. You solve difficult technical problems methodically.

## Step-by-Step Process

1. **Understand** - What is the user asking for?
2. **Analyze** - What tools do I need? What files are involved?
3. **Plan** - What's the sequence of steps?
4. **Execute** - Run commands one at a time, verify each step
5. **Verify** - Did it work? Check the results
6. **Iterate** - If it didn't work, try a different approach`,
}

func detectCategory(task string) string {
	taskLower := strings.ToLower(task)
	
	switch {
	case strings.Contains(taskLower, "git"):
		return "git"
	case strings.Contains(taskLower, "build") || strings.Contains(taskLower, "compile") || strings.Contains(taskLower, "cmake") || strings.Contains(taskLower, "make"):
		return "build"
	case strings.Contains(taskLower, "nginx") || strings.Contains(taskLower, "ssh") || strings.Contains(taskLower, "docker") || strings.Contains(taskLower, "ssl") || strings.Contains(taskLower, "cert"):
		return "devops"
	case strings.Contains(taskLower, "pytorch") || strings.Contains(taskLower, "torch") || strings.Contains(taskLower, "tensorflow") || strings.Contains(taskLower, "ml") || strings.Contains(taskLower, "model"):
		return "ml"
	case strings.Contains(taskLower, "sqlite") || strings.Contains(taskLower, "database") || strings.Contains(taskLower, "sql"):
		return "database"
	default:
		return "default"
	}
}

func (l *AgentLoop) buildSystemMessageWithCategories() string {
	toolsJSON, _ := json.MarshalIndent(l.Tools, "", "  ")
	
	return fmt.Sprintf(`%s

## Your Capabilities

You have access to these tools:

%s

## Important Notes

- Always verify file operations succeeded
- Read error messages carefully and adapt
- Be thorough but efficient
- Explain what you're doing as you go
- When in doubt, explore first before acting`, categoryPrompts["default"], string(toolsJSON))
}
