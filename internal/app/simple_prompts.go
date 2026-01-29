package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Simplified prompts for simple tools
var simpleToolPrompts = map[string]string{
	"regex-log": `Task: Extract IP addresses from a log file.
Steps:
1. Read the log file
2. Write a regex pattern to match IPs (e.g., \d+\.\d+\.\d+\.\d+)
3. Extract and print all IPs

Execute now.`,
	
	"sqlite-db-truncate": `Task: Truncate all data from an SQLite database.
Steps:
1. Open the database with sqlite3
2. Run DELETE FROM or VACUUM commands
3. Verify table is empty

Execute now.`,
	
	"sqlite-with-gcov": `Task: Compile a C program with coverage.
Steps:
1. Compile with: gcc -fprofile-arcs -ftest-coverage -o program source.c
2. Run the program to generate .gcda files
3. Use gcov to generate coverage report

Execute now.`,
	
	"crack-7z-hash": `Task: Crack a 7z password hash.
Steps:
1. Use 7z hash mode or John the Ripper
2. Provide wordlist or bruteforce
3. Report the password

Execute now.`,
	
	"extract-elf": `Task: Extract information from an ELF binary.
Steps:
1. Use readelf -h to get ELF header
2. Use objdump or readelf for sections
3. Print summary

Execute now.`,
	
	"extract-moves-from-video": `Task: Extract moves from a video file.
Steps:
1. Read the video file or frames
2. Use computer vision to detect game pieces
3. Output the sequence of moves

Execute now.`,
	
	"distribution-search": `Task: Search for statistical distributions.
Steps:
1. Use scipy.stats or numpy
2. Search for best-fit distribution
3. Print distribution name and parameters

Execute now.`,
	
	"model-extraction-relu-logits": `Task: Extract ReLU logits from a model.
Steps:
1. Load the neural network model
2. Extract activations from ReLU layers
3. Print the logits

Execute now.`,
	
	"largest-eigenval": `Task: Find the largest eigenvalue.
Steps:
1. Use numpy.linalg.eigvals
2. Find the eigenvalue with largest magnitude
3. Print the value

Execute now.`,
	
	"query-optimize": `Task: Optimize SQL queries.
Steps:
1. Read the query
2. Use EXPLAIN to analyze
3. Suggest and apply optimizations

Execute now.`,
	
	"install-windows-3.11": `Task: Install Windows 3.11 in DOSBox.
Steps:
1. Mount Windows 3.11 installer ISO
2. Run setup.exe
3. Complete installation

Execute now.`,
	
	"pypi-server": `Task: Set up a PyPI server.
Steps:
1. Install pypiserver: pip install pypiserver
2. Run: pypi-server run -p 8080 ./packages
3. Verify server is running

Execute now.`,
	
	"adaptive-rejection-sampler": `Task: Implement adaptive rejection sampling.
Steps:
1. Read the algorithm description
2. Write Python/SciPy code
3. Test with sample distribution

Execute now.`,
}

// Simplified tool-specific system prompt
func (l *AgentLoop) buildSimpleToolsPrompt() string {
	toolsJSON, _ := json.MarshalIndent(l.Tools, "", "  ")
	
	return fmt.Sprintf(`You are a simple task execution agent. Follow these rules:

1. Read the task carefully
2. Execute the exact steps listed
3. Use the appropriate tool for each step
4. Verify each step succeeded
5. Report the final result

## Tools Available:
%s

## Important:
- Use exec for command-line tasks
- Use read_file to read inputs
- Use write_file to create outputs
- Report success or failure clearly

Let's execute the task now.`, string(toolsJSON))
}

// Detect if task needs simplified prompt
func needsSimplePrompt(taskName string) bool {
	simpleTasks := []string{
		"regex-log", "sqlite", "gcov", "crack", "extract",
		"distribution", "eigenval", "query-optimize",
		"install-windows", "pypi-server", "adaptive-rejection",
	}
	
	for _, t := range simpleTasks {
		if strings.Contains(strings.ToLower(taskName), t) {
			return true
		}
	}
	return false
}
