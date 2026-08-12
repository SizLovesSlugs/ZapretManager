//go:build !windows

package dns

import "fmt"

func readAdapters() ([]adapterBackup, error) {
	return nil, fmt.Errorf("dns: только Windows")
}

func setStaticDNS(servers []string) error {
	return fmt.Errorf("dns: только Windows")
}

func restoreBackup(bak backupFile) error {
	return fmt.Errorf("dns: только Windows")
}

func flushDNS() {}
