package exrex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0x464e/traefik-opnsense-sync/internal/config"
)

type Exrex struct {
	ExrexPath    string
	MaxGenerated int
	Cache        map[string][]string

	once              sync.Once
	resolvedExrexPath string
	resolveErr        error
}

func NewExrexRunner(cfg *config.Config) *Exrex {
	return &Exrex{
		ExrexPath:    cfg.Regex.ExrexPath,
		MaxGenerated: cfg.Regex.MaxGenerated,
		Cache:        make(map[string][]string),
	}
}

func (e *Exrex) Generate(pattern string) ([]string, error) {
	if v, ok := e.Cache[pattern]; ok {
		return v, nil
	}
	if strings.TrimSpace(pattern) == "" {
		return nil, errors.New("empty regex pattern")
	}

	// resolve exrex path once, on demand.
	e.once.Do(func() {
		e.resolvedExrexPath, e.resolveErr = resolveExecutable(e.ExrexPath)
		if e.resolveErr != nil {
			e.resolveErr = fmt.Errorf("%w: %v", errors.New("exrex not available"), e.resolveErr)
		}
	})
	if e.resolveErr != nil {
		return nil, e.resolveErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"--max-number", strconv.Itoa(e.MaxGenerated), pattern}
	out, err := exec.CommandContext(ctx, e.resolvedExrexPath, args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("exrex failed for %q: %s", pattern, msg)
	}

	lines := splitNonEmptyLines(string(out))
	e.Cache[pattern] = lines
	return lines, nil
}

func resolveExecutable(name string) (string, error) {
	// if name contains a path separator, treat as literal (absolute or relative).
	if strings.ContainsRune(name, os.PathSeparator) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		return "", fmt.Errorf("not executable or not found: %q", name)
	}

	// search PATH.
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	// try working directory
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("executable %q not found in PATH or working directory", name)
}

func splitNonEmptyLines(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimRight(v, "\r")
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
