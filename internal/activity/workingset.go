package activity

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
