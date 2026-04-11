package heuristic

import (
	"strings"
	"testing"
)

func TestCompactJSON(t *testing.T) {
	compacted, ok := CompactJSON("{\n  \"b\": 1,\n  \"a\": 2\n}")
	if !ok {
		t.Fatalf("expected compact json to report change")
	}
	if compacted != "{\"b\":1,\"a\":2}" {
		t.Fatalf("unexpected compacted json: %s", compacted)
	}
}

func TestDescribeJSONValue(t *testing.T) {
	summary := DescribeJSONValue(map[string]any{
		"z": 1,
		"a": 2,
	}, 12)
	if summary != "JSON Object Keys: a, z" {
		t.Fatalf("unexpected json summary: %s", summary)
	}
}

func TestCodeOutlineLines(t *testing.T) {
	lines := CodeOutlineLines("package main\n\nfunc first() {}\nfunc first() {}\nvar foo = 1", 12)
	if len(lines) != 3 {
		t.Fatalf("expected 3 outline lines, got %d", len(lines))
	}
	if lines[0] != "package main" || lines[1] != "func first() {}" || lines[2] != "var foo = 1" {
		t.Fatalf("unexpected outline lines: %#v", lines)
	}
}

func TestErrorStackFocusLines(t *testing.T) {
	lines := ErrorStackFocusLines("panic: boom\nat main.go:10\n\nat worker.go:20", 10)
	if len(lines) != 3 {
		t.Fatalf("expected 3 selected lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "panic") {
		t.Fatalf("expected panic line first, got %#v", lines)
	}
}

func TestStackTraceLines(t *testing.T) {
	lines := StackTraceLines("Exception: failed\nmain.go:10\nworker.go:20", 10)
	if len(lines) != 3 {
		t.Fatalf("expected stack trace lines, got %#v", lines)
	}
}
