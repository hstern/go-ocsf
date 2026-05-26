// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// readJSON decodes the file at name under fsys into out. Returns
// the wrapped fs.PathError unmodified if the file is missing; that
// lets callers distinguish "missing version.json" from "malformed
// version.json".
func readJSON(fsys fs.FS, name string, out any) error {
	f, err := fsys.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	dec := json.NewDecoder(f)
	dec.UseNumber() // preserve integer enum keys verbatim
	if err := dec.Decode(out); err != nil {
		return err
	}
	return nil
}

// walkJSON walks dir under fsys and invokes fn for every *.json
// file (in slash-separated, sorted order). Subdirectories are
// walked recursively. Missing dir is treated as empty so callers
// don't have to special-case optional subtrees.
func walkJSON(fsys fs.FS, dir string, fn func(p string) error) error {
	var paths []string
	walkErr := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, fs.ErrNotExist) {
		return walkErr
	}
	sort.Strings(paths)
	for _, p := range paths {
		if err := fn(p); err != nil {
			return err
		}
	}
	return nil
}

// openHostFile opens a file on the host filesystem at the given
// path. Factored out so the loader's host-filesystem dependency
// surfaces in one place; everything else in the package speaks
// fs.FS.
func openHostFile(p string) (fs.File, error) {
	return os.Open(p)
}
