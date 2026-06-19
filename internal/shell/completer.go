package shell

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// PATH command completion cache, lazily built and refreshed when $PATH changes.
var (
	pathCache    []string
	pathCacheVal string
	pathCacheMu  sync.Mutex
)

// buildPathCache scans $PATH and caches all executable names for tab completion.
func buildPathCache() {
	dirs := filepath.SplitList(os.Getenv("PATH"))
	seen := make(map[string]bool)
	pathCache = nil
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !seen[name] {
				seen[name] = true
				pathCache = append(pathCache, name)
			}
		}
	}
	sort.Strings(pathCache)
}

// ensurePathCache rebuilds the path cache if PATH has changed since last build.
func ensurePathCache() {
	cur := os.Getenv("PATH")
	pathCacheMu.Lock()
	defer pathCacheMu.Unlock()
	if cur != pathCacheVal {
		pathCacheVal = cur
		buildPathCache()
	}
}

// Do implements readline.AutoCompleter. It completes:
//   - Command names (first word) from the PATH cache
//   - File paths (subsequent words) by reading the filesystem
// Directories get a trailing "/".
func (s *Shell) Do(line []rune, pos int) ([][]rune, int) {
	ensurePathCache()

	input := string(line[:pos])
	words := strings.Fields(input)
	var prefix string
	if len(words) > 0 && !strings.HasSuffix(input, " ") {
		prefix = words[len(words)-1]
	}

	isFirstWord := len(words) == 0 || (len(words) == 1 && !strings.HasSuffix(input, " "))
	var candidates []string

	if isFirstWord {
		for _, cmd := range pathCache {
			if strings.HasPrefix(cmd, prefix) {
				candidates = append(candidates, cmd)
			}
		}
	}

	fileDir := filepath.Dir(prefix)
	if fileDir == "." {
		fileDir = ""
	}
	filePrefix := filepath.Base(prefix)

	searchDir := fileDir
	if strings.HasPrefix(searchDir, "~") {
		searchDir = s.home + searchDir[1:]
	}
	if searchDir == "" {
		searchDir = "."
	}

	entries, err := os.ReadDir(searchDir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, filePrefix) {
				continue
			}
			full := name
			if fileDir != "" {
				full = fileDir + "/" + name
			}
			if e.IsDir() {
				full += "/"
			}
			candidates = append(candidates, full)
		}
	}

	if len(candidates) == 0 {
		return nil, 0
	}

	sort.Strings(candidates)
	preRunes := []rune(prefix)
	completions := make([][]rune, len(candidates))
	for i, c := range candidates {
		cRunes := []rune(c)
		completions[i] = cRunes[len(preRunes):]
	}
	return completions, len(preRunes)
}
