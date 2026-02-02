package app

import (
	"bytes"
	"testing"
)

// BenchmarkDetectCategory benchmarks category detection for various tasks
func BenchmarkDetectCategory(b *testing.B) {
	tasks := []string{
		"git commit -m 'test'",
		"build rust c ffi",
		"sqlite3 truncate table",
		"qemu-system-x86_64 boot",
		"unknown task description",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, task := range tasks {
			GetTaskCategory(task)
		}
	}
}

// BenchmarkParseToolCalls benchmarks parsing tool calls from responses
func BenchmarkParseToolCalls(b *testing.B) {
	l := &AgentLoop{Tools: DefaultTools(), Logger: NewLogger(&bytes.Buffer{})}
	responses := []string{
		`{"tool": "exec", "args": {"command": "ls -la"}}`,
		`{"tool_calls": [{"id": "1", "name": "read_file", "arguments": {"path": "/tmp/test"}}]}`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, resp := range responses {
			l.parseToolCalls(resp)
		}
	}
}

// BenchmarkDefaultTools benchmarks tool definition creation
func BenchmarkDefaultTools(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DefaultTools()
	}
}
