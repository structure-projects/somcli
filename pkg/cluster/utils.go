package cluster

import (
	"fmt"
	"os"
	"path/filepath"
)

// ensureWorkDir creates the work directory if it doesn't exist
func ensureWorkDir(subdir string) (string, error) {
	dir := filepath.Join("work", subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}
	return dir, nil
}
