package session

import (
	"path/filepath"
	"strings"
)

type Repository struct {
	ID           string
	GitCommonDir string
}

func DetectRepository(workspaceRoot string) Repository {
	commonDir, err := gitOutput("rev-parse", "--git-common-dir")
	if err != nil || strings.TrimSpace(commonDir) == "" {
		return Repository{}
	}
	commonDir = strings.TrimSpace(commonDir)
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(workspaceRoot, commonDir)
	}
	commonDir, err = filepath.Abs(commonDir)
	if err != nil {
		return Repository{}
	}
	commonDir = filepath.Clean(commonDir)
	return Repository{ID: filepath.ToSlash(commonDir), GitCommonDir: commonDir}
}
