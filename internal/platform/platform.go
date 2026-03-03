package platform

import "runtime"

func Detect() string {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "mac-arm64"
		}
		return "mac-x64"
	case "linux":
		return "linux64"
	case "windows":
		if runtime.GOARCH == "386" {
			return "win32"
		}
		return "win64"
	default:
		return "linux64"
	}
}
