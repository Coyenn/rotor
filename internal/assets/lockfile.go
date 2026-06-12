package assets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// LockfileName is the committed lockfile recording uploaded asset ids by
// project-relative path.
const LockfileName = "rotor-lock.json"

// LockEntry records one uploaded asset: the content hash that produced it and
// the stable Roblox asset id.
type LockEntry struct {
	Hash    string `json:"hash"`
	AssetID int64  `json:"assetId"`
}

// Lockfile is the on-disk shape of rotor-lock.json. Keys of Assets are
// project-relative forward-slash paths; encoding/json marshals map keys in
// sorted order, so the file is deterministic.
type Lockfile struct {
	Version int                  `json:"version"`
	Assets  map[string]LockEntry `json:"assets"`
}

// NewLockfile returns an empty version-1 lockfile.
func NewLockfile() *Lockfile {
	return &Lockfile{Version: 1, Assets: map[string]LockEntry{}}
}

// LoadLockfile reads rotor-lock.json from projectDir. A missing file is not
// an error (returns an empty lockfile); a corrupt file is, so asset ids are
// never silently discarded and re-uploaded.
func LoadLockfile(projectDir string) (*Lockfile, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, LockfileName))
	if errors.Is(err, fs.ErrNotExist) {
		return NewLockfile(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("assets: reading %s: %w", LockfileName, err)
	}
	l := &Lockfile{}
	if err := json.Unmarshal(data, l); err != nil {
		return nil, fmt.Errorf("assets: %s is not valid JSON: %w", LockfileName, err)
	}
	if l.Version == 0 {
		l.Version = 1
	}
	if l.Assets == nil {
		l.Assets = map[string]LockEntry{}
	}
	return l, nil
}

// Save writes the lockfile atomically (temp file + rename) so a crash never
// leaves a half-written rotor-lock.json.
func (l *Lockfile) Save(projectDir string) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("assets: encoding %s: %w", LockfileName, err)
	}
	data = append(data, '\n')
	if err := writeFileAtomic(filepath.Join(projectDir, LockfileName), data); err != nil {
		return fmt.Errorf("assets: writing %s: %w", LockfileName, err)
	}
	return nil
}

// writeFileAtomic writes data to path via a temp file in the same directory
// followed by a rename (os.Rename replaces the destination on Windows too).
// Parent directories are created as needed.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".rotor-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmpName)
		if werr != nil {
			return werr
		}
		return cerr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
