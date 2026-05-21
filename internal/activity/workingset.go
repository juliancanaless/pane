package activity

import (
	"path/filepath"
	"sort"
)

func RecentFiles(items []FileActivity, limit int) []string {
	seen := make(map[string]bool)
	files := make([]string, 0, limit)
	for _, item := range items {
		if item.Path == "" || seen[item.Path] {
			continue
		}
		seen[item.Path] = true
		files = append(files, item.Path)
		if len(files) == limit {
			break
		}
	}
	return files
}

// Overlap represents file-level overlap between two sessions
type Overlap struct {
	SessionA    string
	SessionB    string
	SharedFiles []string
}

// ComputeOverlap takes a map of path -> []sessionIDs (from store query) and returns
// per-pair overlaps between sessions.
func ComputeOverlap(pathSessions map[string][]string) []Overlap {
	// Build pair -> shared files map
	type pair struct{ a, b string }
	pairFiles := make(map[pair][]string)

	for path, sessions := range pathSessions {
		for i := 0; i < len(sessions); i++ {
			for j := i + 1; j < len(sessions); j++ {
				a, b := sessions[i], sessions[j]
				if a > b {
					a, b = b, a
				}
				p := pair{a, b}
				pairFiles[p] = append(pairFiles[p], path)
			}
		}
	}

	overlaps := make([]Overlap, 0, len(pairFiles))
	for p, files := range pairFiles {
		sort.Strings(files)
		overlaps = append(overlaps, Overlap{
			SessionA:    p.a,
			SessionB:    p.b,
			SharedFiles: files,
		})
	}
	sort.Slice(overlaps, func(i, j int) bool {
		if len(overlaps[i].SharedFiles) != len(overlaps[j].SharedFiles) {
			return len(overlaps[i].SharedFiles) > len(overlaps[j].SharedFiles)
		}
		return overlaps[i].SessionA < overlaps[j].SessionA
	})
	return overlaps
}

// HotDirectories derives directories with 2+ recently active files.
// Returns directory paths sorted by file count descending, limited to maxDirs.
func HotDirectories(files []string, maxDirs int) []string {
	dirCount := make(map[string]int)
	for _, f := range files {
		dir := filepath.Dir(f)
		dirCount[dir]++
	}
	type dirEntry struct {
		dir   string
		count int
	}
	var entries []dirEntry
	for dir, count := range dirCount {
		if count >= 2 {
			entries = append(entries, dirEntry{dir, count})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})
	result := make([]string, 0, maxDirs)
	for _, e := range entries {
		result = append(result, e.dir)
		if len(result) == maxDirs {
			break
		}
	}
	return result
}
