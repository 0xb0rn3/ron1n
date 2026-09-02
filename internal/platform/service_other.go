//go:build !linux && !windows && !darwin

package platform

import "errors"

func InstallService(_ Paths, _, _ string) (string, error) {
	return "", errors.New("background service installation is supported on Linux and Windows")
}

func RestartService() error { return errors.New("background service is unsupported on this platform") }
func ServiceStatus() string { return "unsupported" }
func RemoveService(_ Paths) error {
	return errors.New("background service is unsupported on this platform")
}
