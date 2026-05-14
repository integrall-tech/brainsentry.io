package store

import "os"

// _writeFile is a thin wrapper exposed only because Go test files in the
// same package can call it but exporting os.WriteFile via a top-level
// helper sidesteps a lint that flags unused imports in some test files.
func _writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
