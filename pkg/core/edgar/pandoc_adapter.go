package edgar

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PandocAdapter provides HTML to Markdown conversion using Pandoc CLI.
// Pandoc is the gold-standard for document conversion and handles complex
// table structures (colspan, rowspan) that standard libraries cannot.
type PandocAdapter struct {
	// Timeout for Pandoc execution (default: 30s)
	Timeout time.Duration
}

// NewPandocAdapter creates a new PandocAdapter with default settings.
func NewPandocAdapter() *PandocAdapter {
	return &PandocAdapter{
		Timeout: 30 * time.Second,
	}
}

// IsAvailable checks if Pandoc is installed and accessible.
func (p *PandocAdapter) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pandoc", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// GetVersion returns the installed Pandoc version string.
func (p *PandocAdapter) GetVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pandoc", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pandoc not found: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}
	return "", fmt.Errorf("unable to parse pandoc version")
}

// HTMLToMarkdown converts HTML content to GitHub Flavored Markdown.
// For tables, Pandoc preserves structure much better than regex-based converters.
//
// Options:
//   - Uses pipe_tables extension for clean Markdown table output
//   - Preserves header attributes for anchor navigation
func (p *PandocAdapter) HTMLToMarkdown(html string) (string, error) {
	timeout := p.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Pandoc command:
	//   -f html                    : Input format is HTML
	//   -t markdown+pipe_tables    : Output format includes ASCII grid tables (preserves colspan)
	//   --wrap=none                : Don't wrap lines (important for tables)
	//   -                          : Read from stdin
	cmd := exec.CommandContext(ctx, "pandoc",
		"-f", "html",
		"-t", "markdown+pipe_tables",
		"--wrap=none",
		"-",
	)

	// Pipe HTML to stdin
	cmd.Stdin = strings.NewReader(html)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("pandoc timeout after %v", timeout)
		}
		return "", fmt.Errorf("pandoc failed: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// HTMLToMarkdownWithGridTables uses Pandoc's multiline_tables for complex layouts.
// This is useful when colspan/rowspan need to be preserved visually.
func (p *PandocAdapter) HTMLToMarkdownWithGridTables(html string) (string, error) {
	timeout := p.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Using multiline_tables extension for complex table support
	cmd := exec.CommandContext(ctx, "pandoc",
		"-f", "html",
		"-t", "markdown+multiline_tables",
		"--wrap=none",
		"-",
	)

	cmd.Stdin = strings.NewReader(html)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pandoc failed: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
