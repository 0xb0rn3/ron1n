//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const serviceName = "io.stingraylabs.ron1n"

func InstallService(paths Paths, executable, configFile string) (string, error) {
	executable, err := filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	configFile, err = filepath.Abs(configFile)
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	launchDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchDir, 0o700); err != nil {
		return "", err
	}
	plistPath := filepath.Join(launchDir, serviceName+".plist")
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>` + serviceName + `</string>
  <key>ProgramArguments</key><array>
    <string>` + xmlEscape(executable) + `</string><string>serve</string>
    <string>--config</string><string>` + xmlEscape(configFile) + `</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>StandardOutPath</key><string>` + xmlEscape(filepath.Join(paths.StateDir, "service.stdout.log")) + `</string>
  <key>StandardErrorPath</key><string>` + xmlEscape(filepath.Join(paths.StateDir, "service.stderr.log")) + `</string>
</dict></plist>
`
	if err := os.WriteFile(plistPath, []byte(plist), 0o600); err != nil {
		return "", err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain, plistPath).Run()
	if output, err := exec.Command("launchctl", "bootstrap", domain, plistPath).CombinedOutput(); err != nil {
		return plistPath, fmt.Errorf("launchctl bootstrap: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return plistPath, nil
}

func RestartService() error {
	domain := "gui/" + strconv.Itoa(os.Getuid())
	output, err := exec.Command("launchctl", "kickstart", "-k", domain+"/"+serviceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl kickstart: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ServiceStatus() string {
	domain := "gui/" + strconv.Itoa(os.Getuid())
	if exec.Command("launchctl", "print", domain+"/"+serviceName).Run() == nil {
		return "running"
	}
	return "stopped-or-unavailable"
}

func RemoveService(_ Paths) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", serviceName+".plist")
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain, plistPath).Run()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func xmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	return strings.ReplaceAll(value, "'", "&apos;")
}
