//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const serviceName = "ron1n"

func InstallService(_ Paths, executable, configFile string) (string, error) {
	executable, err := filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	configFile, err = filepath.Abs(configFile)
	if err != nil {
		return "", err
	}
	command := fmt.Sprintf(`\"%s\" serve --config \"%s\"`, executable, configFile)
	args := []string{"/Create", "/SC", "ONLOGON", "/TN", serviceName, "/TR", command, "/F"}
	if output, err := exec.Command("schtasks.exe", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("create Windows scheduled task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("schtasks.exe", "/Run", "/TN", serviceName).CombinedOutput(); err != nil {
		return serviceName, fmt.Errorf("start Windows scheduled task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return serviceName, nil
}

func RestartService() error {
	_ = exec.Command("schtasks.exe", "/End", "/TN", serviceName).Run()
	output, err := exec.Command("schtasks.exe", "/Run", "/TN", serviceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart Windows scheduled task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ServiceStatus() string {
	if exec.Command("schtasks.exe", "/Query", "/TN", serviceName).Run() == nil {
		return "installed"
	}
	return "stopped-or-unavailable"
}

func RemoveService(_ Paths) error {
	output, err := exec.Command("schtasks.exe", "/Delete", "/TN", serviceName, "/F").CombinedOutput()
	if err != nil && !strings.Contains(strings.ToLower(string(output)), "cannot find") {
		return fmt.Errorf("remove Windows scheduled task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func atomicServiceWrite(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	return os.WriteFile(name, data, 0o600)
}
