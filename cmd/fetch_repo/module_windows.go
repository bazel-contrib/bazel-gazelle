//go:build windows

/* Copyright 2026 The Bazel Authors. All rights reserved.

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
	"os"
	"syscall"
	"unsafe"
)

const (
	lockfileExclusiveLock = 0x00000002
	allBytes              = ^uint32(0)
)

var (
	lockFileEx   = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")
	unlockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("UnlockFileEx")
)

func lockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	return callFileLockProcedure(
		lockFileEx,
		file.Fd(),
		lockfileExclusiveLock,
		0,
		uintptr(allBytes),
		uintptr(allBytes),
		uintptr(unsafe.Pointer(&overlapped)),
	)
}

func unlockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	return callFileLockProcedure(
		unlockFileEx,
		file.Fd(),
		0,
		uintptr(allBytes),
		uintptr(allBytes),
		uintptr(unsafe.Pointer(&overlapped)),
	)
}

func callFileLockProcedure(proc *syscall.LazyProc, args ...uintptr) error {
	result, _, callErr := proc.Call(args...)
	if result != 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}
