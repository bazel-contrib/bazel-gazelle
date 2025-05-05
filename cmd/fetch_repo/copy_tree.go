/* Copyright 2019 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"io"
	"os"
	"path/filepath"
)

func copyTree(destRoot, srcRoot string) error {
	return filepath.Walk(srcRoot, func(src string, info os.FileInfo, e error) (err error) {
		if e != nil {
			return e
		}
		rel, err := filepath.Rel(srcRoot, src)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dest := filepath.Join(destRoot, rel)

		if info.IsDir() {
			return os.Mkdir(dest, 0o777)
		}

		// Check if the current file is a symlink and if it's a directory
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := filepath.EvalSymlinks(src)
			if err != nil {
				return err
			}
			linkInfo, err := os.Lstat(linkTarget)
			if err != nil {
				return err
			}
			if linkInfo.IsDir() {
				// Rather than copying the directory symlink we create the dir and continue
				// This resolves an issue where the walk attempts to copy files into the symlinked directory
				// copy_file_range: is a directory
				return os.Mkdir(dest, 0o777)
			}
		}

		r, err := os.Open(src)
		if err != nil {
			return err
		}
		defer r.Close()
		w, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := w.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
		_, err = io.Copy(w, r)
		return err
	})
}
