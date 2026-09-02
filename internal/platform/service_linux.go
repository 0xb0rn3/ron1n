//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const serviceName = "ron1n.service"

func InstallService(paths Paths, executable, configFile string) (string, error) {
	executable, err := filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	configFile, err = filepath.Abs(configFile)
	if err != nil {
		return "", err
	}
	systemdDir := filepath.Join(filepath.Dir(paths.ConfigDir), "systemd", "user")
	if err := os.MkdirAll(systemdDir, 0o700); err != nil {
		return "", err
	}
	unitPath := filepath.Join(systemdDir, serviceName)
	unit := `[Unit]
Description=ron1n verified PS4 content host
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=` + systemdQuote(executable) + ` serve --config ` + systemdQuote(configFile) + `
Restart=on-failure
RestartSec=2
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=` + systemdQuote(paths.StateDir) + `

[Install]
WantedBy=default.target
`
	if err := atomicServiceWrite(unitPath, []byte(unit)); err != nil {
		return "", err
	}
	if output, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return unitPath, fmt.Errorf("systemctl daemon-reload: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("systemctl", "--user", "enable", "--now", serviceName).CombinedOutput(); err != nil {
		return unitPath, fmt.Errorf("systemctl enable: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return unitPath, nil
}

func RestartService() error {
	output, err := exec.Command("systemctl", "--user", "restart", serviceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart %s: %w: %s", serviceName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ServiceStatus() string {
	if exec.Command("systemctl", "--user", "is-active", "--quiet", serviceName).Run() == nil {
		return "running"
	}
	return "stopped-or-unavailable"
}

func RemoveService(paths Paths) error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", serviceName).Run()
	unitPath := filepath.Join(filepath.Dir(paths.ConfigDir), "systemd", "user", serviceName)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func systemdQuote(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func atomicServiceWrite(name string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), ".service-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, name)
}
