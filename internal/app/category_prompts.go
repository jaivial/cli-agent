package app

import (
	"strings"
)

// categoryPrompts maps category names to specialized system prompts.
var categoryPrompts = map[string]string{
	// Basic categories
	"git": `You are a Git expert. Follow these steps for any git task:

1. ALWAYS start with "git status" to see current state
2. Check "git branch" (or "git branch --show-current") to see current branch
3. For untracked/modified files: use "git add <file>" or "git add ."
4. For commits: use "git commit -m 'message'"
5. For branches: use "git checkout -b" or "git checkout"
6. Verify with "git status" after each operation

Common patterns:
- Untracked files -> git add -> git commit
- Modified files -> git add -> git commit
- New branch -> git checkout -b -> work -> git add -> git commit

Tool call format reminder (one tool call per response):
- {"tool":"exec","args":{"command":"git status"}}
- {"tool":"exec","args":{"command":"git add ."}}
- {"tool":"exec","args":{"command":"git commit -m \"your message\""}}`,

	"git_advanced": `You are a Git Recovery and Advanced Operations Expert.

## Advanced Git Recovery Techniques

### Recovering Lost Commits
1. Use git reflog to find lost commits:
   {"tool": "exec", "args": {"command": "git reflog --all"}}

2. Find dangling commits with fsck:
   {"tool": "exec", "args": {"command": "git fsck --lost-found --dangling"}}

3. Recover a commit by SHA:
   {"tool": "exec", "args": {"command": "git cherry-pick <commit-sha>"}}

4. Or checkout to a new branch:
   {"tool": "exec", "args": {"command": "git checkout -b recovery <commit-sha>"}}

### Cherry-pick Strategies
- Single commit: git cherry-pick <sha>
- Range: git cherry-pick <sha1>^..<sha2>
- With merge: git cherry-pick -m 1 <merge-commit-sha>
- Abort conflicts: git cherry-pick --abort
- Skip commit: git cherry-pick --skip

### Rebase Strategies
- Interactive rebase: git rebase -i HEAD~N
- Continue after fix: git rebase --continue
- Abort: git rebase --abort
- Skip: git rebase --skip
- Onto another branch: git rebase --onto <newbase> <oldbase>

### Merge Conflict Resolution
1. Check conflict status: git status
2. See conflict markers in files: grep -n "<<<<<<<" <file>
3. Resolve conflicts manually or use:
   - git checkout --ours <file>
   - git checkout --theirs <file>
4. Mark resolved: git add <file>
5. Complete merge: git commit (or git rebase --continue)

### Reflog Recovery Patterns
- git reflog show HEAD@{N} - view Nth previous state
- git reset --hard HEAD@{1} - undo last operation
- git reset --hard <reflog-entry> - go to specific state

### Best Practices
- ALWAYS run git reflog before panic operations
- Check .git/lost-found/ for recovered objects
- Use git show <sha> to inspect commits before recovery
- Create backup branches before destructive operations`,

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

	"polyglot_build": `You are a Polyglot Build Expert - coordinating multiple toolchains.

## Multi-Language Build Coordination

### Rust + C Integration
1. Check for build.rs file in Rust project
2. Verify C compiler availability: gcc --version
3. For FFI bindings:
   - Build C library first: gcc -shared -fPIC -o libfoo.so foo.c
   - Set LIBRARY_PATH for Rust: export LIBRARY_PATH=/path:$LIBRARY_PATH
   - Build Rust: cargo build
4. Common patterns:
   - Use cc crate in build.rs for C compilation
   - Link with #[link(name = "foo")]
   - Declare extern "C" functions

### Go + C Integration
1. Check for CGO directives: // #cgo LDFLAGS: -L/path -lfoo
2. Set CGO_ENABLED=1 explicitly
3. Build C dependencies first
4. Use go build with -v for verbose output

### Python + C/C++ Extensions
1. Check setup.py or pyproject.toml
2. Verify Python headers: python3-config --includes
3. Build with: python setup.py build_ext --inplace
4. Or use: pip install -e .

### General Multi-Toolchain Steps
1. Identify all languages involved (check file extensions)
2. Determine build order (dependencies first)
3. Set up environment variables (PATH, LIBRARY_PATH, etc.)
4. Build each component separately, verify each step
5. Link final artifacts together

### Common Issues
- "undefined reference" -> Check library linking order
- "cannot find -lfoo" -> Set LD_LIBRARY_PATH or use -L flag
- "fatal error: foo.h" -> Set CFLAGS with -I/path
- Version mismatches -> Check compiler/interpreter versions`,

	"devops": `You are a DevOps expert for system administration tasks:

1. nginx: Check config with nginx -t, reload with nginx -s reload
2. SSH: Check service with systemctl status sshd
3. Docker: Use docker run, docker exec, docker build
4. Certificates: Use openssl for generating certs
5. Services: Use systemctl or service commands

Verify each operation succeeds.`,

	"vm": `You are a VM/QEMU Expert for virtualization tasks.

## QEMU Operations

### Basic QEMU Commands
1. Start VM: qemu-system-x86_64 -hda disk.img -m 512
2. With KVM: qemu-system-x86_64 -enable-kvm -hda disk.img
3. Boot from ISO: qemu-system-x86_64 -cdrom iso -boot d

### Non-Interactive Modes
- Use -nographic for headless operation
- Redirect serial: -serial stdio
- Use -daemonize to run in background
- Disable graphics: -vga none

### Timeout Handling
1. Set reasonable timeouts (60-300s for boot)
2. Use timeout command: timeout 120 qemu-system-x86_64 ...
3. For expect scripts:
   - Send "expect" patterns to match output
   - Use send to provide input
   - Set timeout in expect script

### QEMU Boot Detection
1. Check for login prompt patterns: "login:", "~ #", "$")
2. Monitor serial output for boot messages
3. Use nc (netcat) for monitoring TCP serial ports
4. Check process: ps aux | grep qemu

### SSH Tunnel Setup with QEMU
1. Forward port: -net user,hostfwd=tcp::2222-:22
2. Wait for SSH ready: sleep 30 or check with nc -zv localhost 2222
3. Connect: ssh -p 2222 user@localhost

### Alpine/QEMU Specific
- Alpine uses serial console by default in VMs
- Login as root (no password on fresh installs)
- Use setup-alpine for configuration
- SSH may need to be enabled: rc-service sshd start`,

	"qemu": `You are a QEMU/VM Expert for virtualization tasks.

## QEMU Advanced Operations

### Background Process Management
1. Start QEMU in background:
   qemu-system-x86_64 ... -daemonize
   OR: qemu-system-x86_64 ... &

2. Track PID:
   echo $! > /tmp/qemu.pid

3. Monitor output:
   - Serial to file: -serial file:/tmp/qemu.log
   - Monitor via telnet: -monitor telnet::4444,server,nowait

### Expect Script Patterns
For interactive automation:
  expect "login:"
  send "root\r"
  expect "~ #"
  send "command\r"

### Port Forwarding
- SSH forwarding: -netdev user,id=net0,hostfwd=tcp::2222-:22 -device e1000,netdev=net0
- Multiple ports: hostfwd=tcp::8080-:80,hostfwd=tcp::2222-:22

### Boot Detection Strategies
1. Wait for specific output: grep -q "login:" /tmp/serial.log
2. Check port availability: nc -zv localhost 2222
3. Process check: pgrep qemu-system

### Timeout Best Practices
- Fresh VM boot: 60-120 seconds
- With package installation: 180-300 seconds
- Windows/DOS VMs: 120-180 seconds
- Always have cleanup: kill $(cat /tmp/qemu.pid) on failure`,

	"ml": `You are an ML/AI expert for PyTorch/TensorFlow tasks:

1. Check Python version and dependencies
2. Use virtual environments: python -m venv env
3. Install: pip install torch numpy pandas
4. Load models carefully, check file paths
5. Handle GPU/CPU inference appropriately
6. Verify model loaded before inference`,

	"ml_recovery": `You are a PyTorch Model Recovery Expert.

## PyTorch Model State Inspection

### Loading Corrupted Checkpoints
1. Try loading with weights_only=False:
   checkpoint = torch.load('model.pt', weights_only=False)

2. Load to CPU for inspection:
   checkpoint = torch.load('model.pt', map_location='cpu')

3. Inspect checkpoint structure:
   print(checkpoint.keys()) if dict
   print(type(checkpoint)) if single object

### Tensor Debugging
1. Check tensor shapes:
   for k, v in state_dict.items():
       print(f"{k}: {v.shape}")

2. Check for NaN/Inf:
   torch.isnan(tensor).any()
   torch.isinf(tensor).any()

3. Verify device placement:
   tensor.device

### Checkpoint Recovery
1. Extract partial state_dict:
   state_dict = checkpoint['model_state_dict'] if 'model_state_dict' in checkpoint else checkpoint

2. Handle missing keys:
   model.load_state_dict(state_dict, strict=False)

3. Version mismatch handling:
   - PyTorch 1.x to 2.x: use weights_only=False
   - CUDA to CPU: use map_location='cpu'

4. Partial checkpoint recovery:
   - Load what you can with strict=False
   - Initialize missing layers manually
   - Check for key name mismatches (model. prefix)

### Recovery Steps
1. First attempt: torch.load(path, map_location='cpu')
2. If fails, try pickle loading: pickle.load(open(path, 'rb'))
3. For zip errors: torch.load with mmap=False
4. Inspect with: python -c "import torch; print(torch.load('x.pt').keys())"`,

	"database": `You are a Database expert for SQLite tasks:

1. sqlite3 <dbfile> to open database
2. .tables to list tables
3. .schema <table> to see schema
4. SELECT queries to check data
5. DELETE/TRUNCATE operations as needed
6. Verify changes with SELECT after modification`,

	"sqlite_advanced": `You are a SQLite Advanced Operations Expert.

## SQLite Specific Operations

### DELETE FROM vs TRUNCATE
SQLite does NOT have TRUNCATE TABLE. Use:
- DELETE FROM table_name; (preserves table structure, slower on large tables)
- DELETE FROM table_name WHERE condition; (selective deletion)

Important: After DELETE, run VACUUM to reclaim space.

### VACUUM Operations
1. Full vacuum: VACUUM;
2. Into new file: VACUUM INTO 'backup.db';
3. Check size before/after: ls -la *.db

Benefits:
- Reclaims free space
- Defragments database file
- Can reduce file size significantly

### WAL Mode Operations
1. Check current mode: PRAGMA journal_mode;
2. Enable WAL: PRAGMA journal_mode=WAL;
3. Disable WAL: PRAGMA journal_mode=DELETE;
4. Checkpoint: PRAGMA wal_checkpoint;

WAL mode benefits:
- Better concurrent read performance
- Faster write operations
- Auto-checkpointing

### Backup Commands
1. SQLite CLI backup: .backup backup.db
2. SQL dump: .dump > backup.sql
3. VACUUM INTO: VACUUM INTO 'backup.db';
4. File-level: cp original.db backup.db

### Recovery Operations
1. Check integrity: PRAGMA integrity_check;
2. Recover from dump: sqlite3 new.db < dump.sql
3. Fix corruption: .recover command (sqlite3 3.29+)

### Common Pitfalls
- WAL files (*.db-wal, *.db-shm) must be kept with main db
- DELETE FROM without WHERE deletes all rows but keeps table
- Always verify with SELECT count(*) FROM table after operations`,

	"security": `You are a Security Expert for hash cracking and vulnerability analysis.

## Hash Cracking Operations

### Hash Format Identification
1. Identify hash type by length and pattern:
   - MD5: 32 hex chars
   - SHA1: 40 hex chars
   - SHA256: 64 hex chars
   - bcrypt: $2a$10$... (starts with $2)
   - 7z: very long, contains salts

2. Use hash-identifier: hash-identifier <hash>

### Wordlist Usage
1. Common wordlists locations:
   - /usr/share/wordlists/
   - /usr/share/wordlists/rockyou.txt
   - /opt/wordlists/

2. Custom wordlists:
   - Generate with crunch: crunch 4 6 abcdefghijklmnopqrstuvwxyz
   - Use cewl for target-specific: cewl -d 2 -m 5 http://target.com

### Hashcat Patterns
1. Basic usage: hashcat -m <mode> -a <attack> hashfile wordlist
2. Common modes:
   - 0: MD5
   - 100: SHA1
   - 1400: SHA256
   - 11600: 7-Zip

3. Example: hashcat -m 0 hash.txt /usr/share/wordlists/rockyou.txt

### John the Ripper Patterns
1. Basic: john --wordlist=/path/wordlist hashfile
2. Show cracked: john --show hashfile
3. Specific format: john --format=raw-md5 hashfile

### Password Patterns
1. Common patterns to try:
   - Common passwords: password, admin, 123456
   - Leet speak: p@ssw0rd, 4dm1n
   - Years: password2023, company2024
   - Simple variations: Password1, admin123

2. Rules for mutation:
   john --rules --wordlist=wordlist hashfile

### Best Practices
- Always try common passwords first
- Use smaller wordlists before massive ones
- Check if hash needs preparation (remove usernames, etc.)
- For 7z: use 7z2john.pl first to extract hash`,

	"default": `You are an expert CLI agent that accomplishes complex technical tasks through shell commands and file operations.

## Your Role

You are a senior software engineer, DevOps specialist, and systems programmer. You solve difficult technical problems methodically.

## Step-by-Step Process

1. **Understand** - What is the user asking for?
2. **Analyze** - What tools do I need? What files are involved?
3. **Plan** - What is the sequence of steps?
4. **Execute** - Run commands one at a time, verify each step
5. **Verify** - Did it work? Check the results
6. **Iterate** - If it did not work, try a different approach`,

	"terminal_bench": `You are running Terminal-Bench 2.0 via Harbor (host-side orchestration + Docker).

## Fast, correct run path
1. Preflight (Docker + KVM):
   {"tool":"exec","args":{"command":"PATH=\\\"$PWD/.venv/bin:$PATH\\\" ./harbor_preflight.sh --require-kvm","timeout":300}}

2. Build Harbor-compatible binary:
   {"tool":"exec","args":{"command":"./harbor_build_eai.sh","timeout":900}}

3. Run + track (recommended):
   {"tool":"exec","args":{"command":"PATH=\\\"$PWD/.venv/bin:$PATH\\\" ./harbor_run_and_track.sh tbench2_all_glm47_coding_v22.harbor.yaml","timeout":28800}}

Notes:
- If docker info fails with /var/run/docker.sock permission denied, run docker/harbor commands via: sg docker -c '...'
  (or use ./harbor_run.sh which already does this).
- Do NOT try to fix Docker group membership with sudo usermod ... unless the user explicitly approves
  (it requires re-login to take effect).`,
}

// detectCategory analyzes a task description and returns the most appropriate category.
// Uses semantic keyword matching and can detect compound tasks.
func detectCategory(task string) string {
	taskLower := strings.ToLower(task)

	// Compound task detection - check for multiple categories
	categories := []string{}

	// Terminal-Bench / Harbor benchmark detection.
	if strings.Contains(taskLower, "terminal-bench") ||
		strings.Contains(taskLower, "terminal bench") ||
		strings.Contains(taskLower, "tbench") ||
		strings.Contains(taskLower, "harbor") {
		categories = append(categories, "terminal_bench")
	}

	// Git detection - distinguish between basic and advanced
	if strings.Contains(taskLower, "git") {
		if strings.Contains(taskLower, "reflog") ||
			strings.Contains(taskLower, "fsck") ||
			strings.Contains(taskLower, "cherry-pick") ||
			strings.Contains(taskLower, "rebase") ||
			strings.Contains(taskLower, "merge conflict") ||
			strings.Contains(taskLower, "recover") ||
			strings.Contains(taskLower, "lost commit") {
			categories = append(categories, "git_advanced")
		} else {
			categories = append(categories, "git")
		}
	}

	// VM/QEMU detection
	if strings.Contains(taskLower, "qemu") ||
		strings.Contains(taskLower, "vm ") ||
		strings.Contains(taskLower, "virtual machine") ||
		(strings.Contains(taskLower, "alpine") && strings.Contains(taskLower, "ssh")) ||
		(strings.Contains(taskLower, "install") && strings.Contains(taskLower, "windows")) {
		categories = append(categories, "vm")
		if strings.Contains(taskLower, "qemu") {
			categories = append(categories, "qemu")
		}
	}

	// Security/Hash cracking detection
	if strings.Contains(taskLower, "crack") ||
		strings.Contains(taskLower, "hash") ||
		(strings.Contains(taskLower, "password") && strings.Contains(taskLower, "recover")) ||
		strings.Contains(taskLower, "hashcat") ||
		strings.Contains(taskLower, "john") ||
		(strings.Contains(taskLower, "7z") && strings.Contains(taskLower, "hash")) {
		categories = append(categories, "security")
	}

	// ML Recovery detection
	if (strings.Contains(taskLower, "pytorch") && strings.Contains(taskLower, "recover")) ||
		(strings.Contains(taskLower, "model") && strings.Contains(taskLower, "corrupt")) ||
		(strings.Contains(taskLower, "checkpoint") && strings.Contains(taskLower, "load")) ||
		strings.Contains(taskLower, "torch.load") ||
		strings.Contains(taskLower, "state_dict") {
		categories = append(categories, "ml_recovery")
	}

	// Build detection - distinguish polyglot
	if strings.Contains(taskLower, "build") ||
		strings.Contains(taskLower, "compile") ||
		strings.Contains(taskLower, "cmake") ||
		strings.Contains(taskLower, "make") {
		if (strings.Contains(taskLower, "rust") && strings.Contains(taskLower, "c")) ||
			strings.Contains(taskLower, "polyglot") ||
			strings.Contains(taskLower, "ffi") ||
			(strings.Contains(taskLower, "bind") && strings.Contains(taskLower, "gen")) ||
			strings.Contains(taskLower, "cgo") {
			categories = append(categories, "polyglot_build")
		} else {
			categories = append(categories, "build")
		}
	}

	// Database detection - distinguish advanced SQLite
	if strings.Contains(taskLower, "sqlite") ||
		strings.Contains(taskLower, "database") ||
		strings.Contains(taskLower, "sql") {
		if strings.Contains(taskLower, "truncate") ||
			strings.Contains(taskLower, "vacuum") ||
			strings.Contains(taskLower, "wal") ||
			strings.Contains(taskLower, "backup") ||
			strings.Contains(taskLower, "corrupt") ||
			strings.Contains(taskLower, "recover") {
			categories = append(categories, "sqlite_advanced")
		} else {
			categories = append(categories, "database")
		}
	}

	// DevOps detection
	if strings.Contains(taskLower, "nginx") ||
		strings.Contains(taskLower, "ssh") ||
		strings.Contains(taskLower, "docker") ||
		strings.Contains(taskLower, "ssl") ||
		strings.Contains(taskLower, "cert") {
		categories = append(categories, "devops")
	}

	// Basic ML detection (not recovery)
	if strings.Contains(taskLower, "pytorch") ||
		strings.Contains(taskLower, "torch") ||
		strings.Contains(taskLower, "tensorflow") ||
		strings.Contains(taskLower, "ml") ||
		strings.Contains(taskLower, "model") {
		// Only add ml if not already added as ml_recovery
		found := false
		for _, c := range categories {
			if c == "ml_recovery" {
				found = true
				break
			}
		}
		if !found {
			categories = append(categories, "ml")
		}
	}

	// Return the most specific category, or default
	if len(categories) == 0 {
		return "default"
	}

	// Return first match from priority order
	priority := []string{
		"terminal_bench",
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

// detectCompoundTasks identifies if a task involves multiple operations
// Returns true if task appears to have multiple distinct sub-tasks
func detectCompoundTasks(task string) bool {
	taskLower := strings.ToLower(task)

	// Check for explicit conjunctions indicating multiple steps
	conjunctions := []string{
		" and ", " then ", " after ", " before ",
		"build and test", "compile and run",
		"install and configure", "setup and verify",
	}

	for _, conj := range conjunctions {
		if strings.Contains(taskLower, conj) {
			return true
		}
	}

	// Check for multiple category indicators
	categoryCount := 0
	if strings.Contains(taskLower, "git") {
		categoryCount++
	}
	if strings.Contains(taskLower, "build") || strings.Contains(taskLower, "compile") {
		categoryCount++
	}
	if strings.Contains(taskLower, "test") {
		categoryCount++
	}
	if strings.Contains(taskLower, "deploy") || strings.Contains(taskLower, "install") {
		categoryCount++
	}

	return categoryCount >= 2
}

// getRelatedCategories returns additional categories that may be relevant
func getRelatedCategories(primaryCategory string) []string {
	switch primaryCategory {
	case "git_advanced":
		return []string{"git"}
	case "sqlite_advanced":
		return []string{"database"}
	case "ml_recovery":
		return []string{"ml"}
	case "polyglot_build":
		return []string{"build"}
	case "qemu":
		return []string{"vm"}
	default:
		return []string{}
	}
}
