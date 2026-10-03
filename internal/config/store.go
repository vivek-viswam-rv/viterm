package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store reads and writes viterm's JSON configuration files in a single
// directory.
type Store struct {
	Dir string
}

// Open resolves the user's config directory, creates it if needed, and
// returns a Store over it.
func Open() (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	if err := EnsureDir(dir); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

// OpenAt returns a Store over an explicit directory. Intended for tests.
func OpenAt(dir string) *Store {
	return &Store{Dir: dir}
}

// readJSON loads a JSON file into v. A missing file leaves v untouched.
func (s *Store) readJSON(filename string, v any) error {
	data, err := os.ReadFile(filepath.Join(s.Dir, filename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parsing %s: %w", filename, err)
	}
	return nil
}

// writeJSON atomically replaces a JSON file with the indented encoding of v.
func (s *Store) writeJSON(filename string, v any) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(s.Dir, filename)
	// Dotfile setups often symlink config files into a repository; writing
	// through the link keeps that arrangement intact.
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), filename+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// randomID returns an 8-character hexadecimal identifier.
func randomID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%08x", os.Getpid())
	}
	return hex.EncodeToString(b[:])
}
