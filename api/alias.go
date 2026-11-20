package api

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
)

func aliasLine(projectDir string) string {
	return fmt.Sprintf("alias proxito='(cd %s && go run .)'", projectDir)
}

func shellConfigFile() (string, bool) {
	if runtime.GOOS == "windows" {
		return "", false
	}
	home, _ := os.UserHomeDir()
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return home + "/.zshrc", true
	}
	rc := home + "/.bashrc"
	if _, err := os.Stat(rc); err == nil {
		return rc, true
	}
	return home + "/.bash_profile", true
}

func aliasAlreadyExists(cfgFile string) bool {
	f, err := os.Open(cfgFile)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "alias proxito=") {
			return true
		}
	}
	return false
}

func (s *Server) handleCreateAlias(w http.ResponseWriter, r *http.Request) {
	cfgFile, ok := shellConfigFile()
	if !ok {
		jsonErr(w, "alias creation not supported on this OS", http.StatusNotImplemented)
		return
	}
	if aliasAlreadyExists(cfgFile) {
		jsonOK(w, map[string]any{"status": "already_exists", "alias_file": cfgFile})
		return
	}
	f, err := os.OpenFile(cfgFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	if _, err = fmt.Fprintf(f, "\n# Proxito alias\n%s\n", aliasLine(s.projectDir)); err != nil {
		jsonErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"status": "ok", "alias_file": cfgFile})
}
