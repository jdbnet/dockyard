package compose

import (
	"os"
	"path/filepath"
)

// IsEditable reports whether the stack compose file can be modified.
func (s Stack) IsEditable() bool {
	if s.Managed {
		return true
	}
	if s.ComposeFile == "" {
		return false
	}
	return fileWritable(s.ComposeFile)
}

func fileWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.Mode()&0200 == 0 {
		return false
	}
	dir := filepath.Dir(path)
	dinfo, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return dinfo.Mode()&0200 != 0
}
