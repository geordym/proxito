package jvm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// JVM represents a running Java process.
type JVM struct {
	PID      string `json:"pid"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Attached bool   `json:"attached"`
	Watched  bool   `json:"watched"`
}

// Manager handles JVM discovery and proxy attachment.
type Manager struct {
	agentJar string
	mu       sync.Mutex
	javaBin  string
	attached map[string]bool
	inflight map[string]bool
	watch    watchList
	// tools.jar cache: empty string = not yet resolved
	toolsJar string
	toolsDone bool
}

func New(agentJar, javaBin string) *Manager {
	return &Manager{
		agentJar: agentJar,
		javaBin:  javaBin,
		attached: make(map[string]bool),
		inflight: make(map[string]bool),
	}
}

// SetJavaBin updates the java binary at runtime (settings change).
func (m *Manager) SetJavaBin(bin string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.javaBin = bin
	m.toolsDone = false // invalidate tools.jar cache
	m.toolsJar = ""
}

// GetJavaBin returns the current java binary path.
func (m *Manager) GetJavaBin() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.javaBin
}

// List returns running JVMs via the agent JAR's list-targets command.
func (m *Manager) List() ([]JVM, error) {
	javaBin, flags := m.javaAndFlags()
	args := append(flags, "-jar", m.agentJar, "list-targets")
	out, err := exec.Command(javaBin, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w — %s", err, strings.TrimSpace(string(out)))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var jvms []JVM
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		pid := line[:colon]
		if _, err := strconv.Atoi(pid); err != nil {
			continue
		}
		desc := strings.TrimSpace(line[colon+1:])
		name := ProcessName(desc)
		if len(desc) > 90 {
			desc = desc[:87] + "..."
		}
		jvms = append(jvms, JVM{
			PID:      pid,
			Name:     name,
			Desc:     desc,
			Attached: m.attached[pid],
			Watched:  m.watch.contains(name),
		})
	}
	return jvms, nil
}

// ErrAttachInProgress is returned when another attach to the same pid is running.
var ErrAttachInProgress = errors.New("attach en curso para ese PID")

// Attach injects the proxy into the JVM with the given pid. It is idempotent:
// attaching an already attached pid is a no-op.
func (m *Manager) Attach(pid, host, port, certPath string) error {
	m.mu.Lock()
	if m.attached[pid] {
		m.mu.Unlock()
		return nil
	}
	if m.inflight[pid] {
		m.mu.Unlock()
		return ErrAttachInProgress
	}
	m.inflight[pid] = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.inflight, pid)
		m.mu.Unlock()
	}()

	javaBin, flags := m.javaAndFlags()
	args := append(flags, "-jar", m.agentJar, pid, host, port, certPath)
	cmd := exec.Command(javaBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	m.mu.Lock()
	m.attached[pid] = true
	m.mu.Unlock()
	return nil
}

// javaAndFlags returns the java binary and any extra flags (tools.jar for Java 8).
func (m *Manager) javaAndFlags() (string, []string) {
	m.mu.Lock()
	bin := m.javaBin
	if !m.toolsDone {
		m.toolsJar = findToolsJar(bin)
		m.toolsDone = true
	}
	jar := m.toolsJar
	m.mu.Unlock()

	var flags []string
	if jar != "" {
		flags = []string{"-Xbootclasspath/a:" + jar}
	}
	return bin, flags
}

// findToolsJar looks for tools.jar using three strategies.
func findToolsJar(javaBin string) string {
	// Strategy 1: derive from java executable location
	if javaExe, err := exec.LookPath(javaBin); err == nil {
		binDir := filepath.Dir(javaExe)
		for _, rel := range []string{
			"../lib/tools.jar",
			"../../lib/tools.jar",
		} {
			abs, _ := filepath.Abs(filepath.Join(binDir, rel))
			if isFile(abs) {
				return abs
			}
		}
	}

	// Strategy 2: JAVA_HOME env
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		for _, rel := range []string{"lib/tools.jar", "../lib/tools.jar"} {
			abs, _ := filepath.Abs(filepath.Join(javaHome, rel))
			if isFile(abs) {
				return abs
			}
		}
	}

	// Strategy 3: parse -XshowSettings:property output
	out, err := exec.Command(javaBin, "-XshowSettings:property", "-version").CombinedOutput()
	if err != nil {
		return ""
	}
	var javaHome string
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.Contains(ln, "java.home") {
			if parts := strings.SplitN(ln, "=", 2); len(parts) == 2 {
				javaHome = strings.TrimSpace(parts[1])
				break
			}
		}
	}
	for _, rel := range []string{"lib/tools.jar", "../lib/tools.jar"} {
		abs, _ := filepath.Abs(filepath.Join(javaHome, rel))
		if isFile(abs) {
			return abs
		}
	}
	return ""
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
