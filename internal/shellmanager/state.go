package shellmanager

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pelletier/go-toml/v2"
)

const stateSchemaVersion = 1

// State is what shell-manager knows about this machine from previous runs.
type State struct {
	SchemaVersion int `toml:"schema_version"`
	// Files maps each managed file to the hash of what shell-manager wrote.
	Files map[string]string `toml:"files"`
	// Versions records versions backends know but checks cannot report.
	Versions map[string]string `toml:"versions"`
}

// LoadState reads the state file; a missing file is an empty state.
func LoadState(sys System, path string) (*State, error) {
	st := &State{}
	data, err := sys.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := toml.Unmarshal(data, st); err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
	}
	if st.SchemaVersion > stateSchemaVersion {
		return nil, fmt.Errorf("%s was written by a newer shell-manager (schema %d); run shm self-update", path, st.SchemaVersion)
	}
	// Schema 1 is the first version: nothing older to migrate yet.
	st.SchemaVersion = stateSchemaVersion
	if st.Files == nil {
		st.Files = map[string]string{}
	}
	if st.Versions == nil {
		st.Versions = map[string]string{}
	}
	return st, nil
}

// Save writes the state file.
func (s *State) Save(sys System, path string) error {
	data, err := toml.Marshal(s)
	if err != nil {
		return err
	}
	return sys.WriteFile(path, data, 0o644)
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
