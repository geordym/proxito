package jvm

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// JDKInfo describes a detected Java installation.
type JDKInfo struct {
	Path    string `json:"path"`    // full path to java/java.exe
	Name    string `json:"name"`    // display name (dir name)
	Major   int    `json:"major"`   // major version (8, 11, 17 …)
	HasTools bool  `json:"has_tools"` // tools.jar present (Java 8 only)
}

// DetectJDKs scans well-known locations and returns all usable JDKs,
// sorted by major version descending (highest first).
func DetectJDKs() []JDKInfo {
	var roots []string
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		roots = []string{
			filepath.Join(home, ".jdks"),
			`C:\Program Files\Java`,
			`C:\Program Files\Eclipse Adoptium`,
			`C:\Program Files\Microsoft`,
			`C:\Program Files\Amazon Corretto`,
			`C:\Program Files (x86)\Java`,
		}
	case "darwin":
		roots = []string{
			filepath.Join(home, ".jdks"),
			"/Library/Java/JavaVirtualMachines",
			filepath.Join(home, "Library/Java/JavaVirtualMachines"),
		}
	default:
		roots = []string{
			filepath.Join(home, ".jdks"),
			"/usr/lib/jvm",
			"/usr/local/lib/jvm",
		}
	}

	seen := map[string]bool{}
	var jdks []JDKInfo

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			javaBin := javaExe(filepath.Join(root, e.Name()))
			if javaBin == "" {
				// macOS: Contents/Home/bin/java
				javaBin = javaExe(filepath.Join(root, e.Name(), "Contents", "Home"))
			}
			if javaBin == "" || seen[javaBin] {
				continue
			}
			seen[javaBin] = true

			major := parseMajor(e.Name())
			info := JDKInfo{
				Path:  javaBin,
				Name:  e.Name(),
				Major: major,
			}
			if major == 8 || major == 0 {
				toolsPath := findToolsJar(javaBin)
				info.HasTools = toolsPath != ""
			}
			jdks = append(jdks, info)
		}
	}

	sort.Slice(jdks, func(i, j int) bool {
		return jdks[i].Major > jdks[j].Major
	})
	return jdks
}

// BestJava returns the path to the best available java binary:
// prefers a JDK >= 11 from ~/.jdks, falls back to system java.
func BestJava(jdks []JDKInfo) string {
	// Prefer 11-21 from ~/.jdks (most stable for the agent)
	home, _ := os.UserHomeDir()
	jdksDir := filepath.Join(home, ".jdks")
	for _, j := range jdks {
		if j.Major >= 11 && strings.HasPrefix(j.Path, jdksDir) {
			return j.Path
		}
	}
	// Any 11+ JDK
	for _, j := range jdks {
		if j.Major >= 11 {
			return j.Path
		}
	}
	// Java 8 JDK with tools.jar
	for _, j := range jdks {
		if j.Major == 8 && j.HasTools {
			return j.Path
		}
	}
	// Fall back to system java
	return "java"
}

// javaExe returns the full path to java/java.exe inside a JDK root dir,
// or "" if not found.
func javaExe(jdkRoot string) string {
	name := "java"
	if runtime.GOOS == "windows" {
		name = "java.exe"
	}
	p := filepath.Join(jdkRoot, "bin", name)
	if isFile(p) {
		return p
	}
	return ""
}

var reMajor = regexp.MustCompile(`(?:^|[-._])1\.(\d+)|(?:^|[-._])(\d{2,3})(?:$|[-._])`)

// parseMajor extracts the Java major version from a directory name.
// Examples: "ms-11.0.27" → 11, "corretto-1.8.0_492" → 8, "openjdk-25" → 25.
func parseMajor(name string) int {
	m := reMajor.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	if m[1] != "" { // matched 1.X → Java X
		n, _ := strconv.Atoi(m[1])
		return n
	}
	n, _ := strconv.Atoi(m[2])
	return n
}
