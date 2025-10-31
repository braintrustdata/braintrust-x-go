package braintrust

import (
	_ "embed"
	"os"
	"os/exec"
	"strings"
	"testing"
)

//go:embed README.md
var readme string

func TestReadmeSnippets(t *testing.T) {
	lines := strings.Split(readme, "\n")
	var snippet []string
	snippetCount := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "```go") {
			snippet = []string{}
			continue
		}
		if strings.HasPrefix(line, "```") && snippet != nil {
			snippetCount++
			code := strings.Join(snippet, "\n")
			if err := tryCompile(code); err != nil {
				t.Errorf("README snippet %d failed to compile: %v\n%s", snippetCount, err, code)
			} else {
				t.Logf("README snippet %d compiled successfully", snippetCount)
			}
			snippet = nil
			continue
		}
		if snippet != nil {
			snippet = append(snippet, line)
		}
	}

	if snippetCount == 0 {
		t.Error("No Go code snippets found in README.md")
	} else {
		t.Logf("Tested %d Go code snippets from README.md", snippetCount)
	}
}

func tryCompile(code string) error {
	tmp := "snippet.go"
	defer func() {
		_ = os.Remove(tmp)
		_ = os.Remove("snippet") // Remove binary if created
	}()

	// Don't add "package main" if it's already there
	if !strings.HasPrefix(strings.TrimSpace(code), "package main") {
		code = "package main\n\n" + code
	}

	if err := os.WriteFile(tmp, []byte(code), 0644); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", tmp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &compileError{err: err, output: string(output)}
	}
	return nil
}

type compileError struct {
	err    error
	output string
}

func (e *compileError) Error() string {
	return e.err.Error() + "\nCompilation output:\n" + e.output
}
