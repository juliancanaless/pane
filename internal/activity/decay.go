package activity

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	FullDetailWindow = 5 * time.Minute
	SummaryWindow    = 2 * time.Hour
	CompressedWindow = 72 * time.Hour
)

type DecayDigest struct {
	FullFiles       []string
	SummaryFiles    int
	SummaryDirs     []string
	CompressedFiles int
	CompressedDirs  []string
}

func (d DecayDigest) Lines() []string {
	var lines []string
	if d.SummaryFiles > 0 {
		line := fmt.Sprintf("%d %s in summary tier (5m–2h)", d.SummaryFiles, plural(d.SummaryFiles, "file", "files"))
		if len(d.SummaryDirs) > 0 {
			line += ": " + strings.Join(d.SummaryDirs, ", ")
		}
		lines = append(lines, line)
	}
	if d.CompressedFiles > 0 {
		line := fmt.Sprintf("%d %s compressed (2h–72h)", d.CompressedFiles, plural(d.CompressedFiles, "file", "files"))
		if len(d.CompressedDirs) > 0 {
			line += ": " + strings.Join(d.CompressedDirs, ", ")
		}
		lines = append(lines, line)
	}
	return lines
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func DecayActivities(items []FileActivity, now time.Time, fullLimit, dirLimit int) DecayDigest {
	fullSeen := make(map[string]bool)
	summarySeen := make(map[string]bool)
	compressedSeen := make(map[string]bool)
	var full []string

	for _, item := range items {
		if item.Path == "" || item.Timestamp <= 0 {
			continue
		}
		age := now.Sub(time.Unix(item.Timestamp, 0))
		if age < 0 {
			age = 0
		}
		switch {
		case age < FullDetailWindow:
			if !fullSeen[item.Path] {
				fullSeen[item.Path] = true
				if len(full) < fullLimit {
					full = append(full, item.Path)
				}
			}
		case age < SummaryWindow:
			summarySeen[item.Path] = true
		case age < CompressedWindow:
			compressedSeen[item.Path] = true
		}
	}

	return DecayDigest{
		FullFiles:       full,
		SummaryFiles:    len(summarySeen),
		SummaryDirs:     topDirs(summarySeen, dirLimit),
		CompressedFiles: len(compressedSeen),
		CompressedDirs:  topDirs(compressedSeen, dirLimit),
	}
}

func topDirs(files map[string]bool, limit int) []string {
	if limit <= 0 || len(files) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for file := range files {
		counts[filepath.ToSlash(filepath.Dir(file))]++
	}
	type dirCount struct {
		dir   string
		count int
	}
	entries := make([]dirCount, 0, len(counts))
	for dir, count := range counts {
		entries = append(entries, dirCount{dir: dir, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].dir < entries[j].dir
	})
	result := make([]string, 0, min(limit, len(entries)))
	for _, entry := range entries {
		result = append(result, entry.dir)
		if len(result) == limit {
			break
		}
	}
	return result
}
