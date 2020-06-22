// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build windows

// Package reboot uses native syscalls to reboot a machine.
package reboot

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	tokenQuery                         = 0x0008
	tokenAdjustPrivileges              = 0x0020
	seShutdownName                     = "SeShutdownPrivilege"
	sePrivilegeEnabled          uint32 = 0x00000002
	ewxForceIfHung                     = 0x00000010
	ewxReboot                          = 0x00000002
	shutdownReasonMajorSoftware        = 0x00030000
)

var (
	user32   *windows.LazyDLL = windows.NewLazySystemDLL("user32.dll")
	kernel32 *windows.LazyDLL = windows.NewLazySystemDLL("kernel32.dll")
	advapi32 *windows.LazyDLL = windows.NewLazySystemDLL("advapi32.dll")

	exitWindowsEx         *windows.LazyProc = user32.NewProc("ExitWindowsEx")
	getCurrentProcess     *windows.LazyProc = kernel32.NewProc("GetCurrentProcess")
	closeHandle           *windows.LazyProc = kernel32.NewProc("CloseHandle")
	openProcessToken      *windows.LazyProc = advapi32.NewProc("OpenProcessToken")
	lookupPrivilegeValue  *windows.LazyProc = advapi32.NewProc("LookupPrivilegeValueW")
	adjustTokenPrivileges *windows.LazyProc = advapi32.NewProc("AdjustTokenPrivileges")
)

type luid struct {
	lowPart  uint32 // DWORD
	highPart int32  // long
}

type luidAndAttributes struct {
	luid       luid   // LUID
	attributes uint32 // DWORD
}

type tokenPrivileges struct {
	privilegeCount uint32 // DWORD
	privileges     [1]luidAndAttributes
}

// Now attempts to obtain the proper tokens then initiates a system reboot.
func Now() error {
	currentProcess, _, _ := getCurrentProcess.Call()
	var hToken uintptr

	if returnCode, _, err := openProcessToken.Call(currentProcess, tokenAdjustPrivileges|tokenQuery, uintptr(unsafe.Pointer(&hToken))); returnCode == 0 {
		return fmt.Errorf("OpenProcessToken() exited with %q and error:%v", returnCode, err)
	}
	defer close(currentProcess)

	var tkp tokenPrivileges
	if returnCode, _, err := lookupPrivilegeValue.Call(uintptr(0), uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(seShutdownName))), uintptr(unsafe.Pointer(&(tkp.privileges[0].luid)))); returnCode == 0 {
		return fmt.Errorf("LookupPrivilegeValue() exited with %q and error:%v", returnCode, err)
	}

	tkp.privilegeCount = 1
	tkp.privileges[0].attributes = sePrivilegeEnabled

	if returnCode, _, err := adjustTokenPrivileges.Call(hToken, 0, uintptr(unsafe.Pointer(&tkp)), 0, uintptr(0), 0); returnCode == 0 {
		return fmt.Errorf("AdjustTokenPrivileges() exited with %q and error:%v", returnCode, err)
	}

	if returnCode, _, err := exitWindowsEx.Call(ewxReboot|ewxForceIfHung, shutdownReasonMajorSoftware); returnCode == 0 {
		return fmt.Errorf("failed to initiate reboot: %v", err)
	}

	return nil
}

func close(handle uintptr) error {
	if returnCode, _, err := closeHandle.Call(handle); returnCode == 0 {
		return fmt.Errorf("failed to close handle: %v", err)
	}
	return nil
}
