package shellmanager

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
)

// An shm block is the part of a user's file (~/.zshrc, ~/.zshenv) that shm
// owns: the lines from blockBegin to blockEnd. The rest of the file is the
// user's (or other installers') and is never touched.
const (
	blockBegin = "# >>> shm >>>"
	blockEnd   = "# <<< shm <<<"
)

// ErrBrokenMarkers means a file has an incomplete or repeated shm block.
var ErrBrokenMarkers = errors.New("broken shm block markers (fix them by hand)")

// findBlock returns the byte range of the shm block in content, from the
// start of the begin marker line to the end of the end marker line
// (its newline included). Marker lines may end in spaces or \r. found is
// false when neither marker is present.
func findBlock(content []byte) (start, end int, found bool, err error) {
	var begins, ends []int
	for off := 0; off < len(content); {
		next := len(content)
		if i := bytes.IndexByte(content[off:], '\n'); i >= 0 {
			next = off + i + 1
		}
		switch string(bytes.TrimRight(content[off:next], " \t\r\n")) {
		case blockBegin:
			begins = append(begins, off)
		case blockEnd:
			ends = append(ends, next)
		}
		off = next
	}
	switch {
	case len(begins) == 0 && len(ends) == 0:
		return 0, 0, false, nil
	case len(begins) == 1 && len(ends) == 1 && begins[0] < ends[0]:
		return begins[0], ends[0], true, nil
	default:
		return 0, 0, false, ErrBrokenMarkers
	}
}

// writeBlock keeps block (both marker lines included) in path: inserted at
// the top when missing, replaced in place when shm wrote the current one.
// A block edited by hand is left alone unless forced. Anything replaced that
// shm did not write is backed up first. Symlinks are followed, so dotfile
// managers keep their link; state is keyed by path.
func (w fileWriter) writeBlock(path string, block []byte, force bool) (Result, error) {
	target := resolveLink(w.sys, path)
	current, err := w.sys.ReadFile(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ResultFailed, err
	}
	start, end, found, err := findBlock(current)
	if err != nil {
		return ResultFailed, fmt.Errorf("%s: %w", path, err)
	}
	var next []byte
	switch {
	case found:
		recorded, managed := w.state.Blocks[path]
		edited := managed && hashBytes(current[start:end]) != recorded
		if edited && !force {
			return ResultModified, nil
		}
		if bytes.Equal(current[start:end], block) {
			w.state.Blocks[path] = hashBytes(block)
			return ResultOK, nil
		}
		if !managed || edited {
			if err := w.sys.WriteFile(w.backupPath(path), current, 0o644); err != nil {
				return ResultFailed, err
			}
		}
		next = slices.Concat(current[:start], block, current[end:])
	case len(bytes.TrimSpace(current)) > 0:
		if err := w.sys.WriteFile(w.backupPath(path), current, 0o644); err != nil {
			return ResultFailed, err
		}
		next = slices.Concat(block, []byte("\n"), current)
	default:
		next = block
	}
	if err := w.sys.WriteFile(target, next, 0o644); err != nil {
		return ResultFailed, err
	}
	w.state.Blocks[path] = hashBytes(block)
	return ResultOK, nil
}

// resolveLink follows path's symlinks (at most 40, like the kernel) and
// returns the final target; path itself if it is not a link or is broken.
func resolveLink(sys System, path string) string {
	for range 40 {
		info, err := sys.Lstat(path)
		if err != nil || info.Mode()&fs.ModeSymlink == 0 {
			return path
		}
		dest, err := sys.Readlink(path)
		if err != nil {
			return path
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(path), dest)
		}
		path = dest
	}
	return path
}
