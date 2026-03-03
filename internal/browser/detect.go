package browser

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type Installed struct {
	Name    string // "Google Chrome" or "Chromium"
	Version string // e.g. "131.0.6778.204"
	Major   int
}

// DetectInstalled finds installed Chrome and Chromium and returns their versions.
func DetectInstalled() []Installed {
	var browsers []Installed

	type candidate struct {
		name string
		cmds [][]string
	}

	var candidates []candidate

	switch runtime.GOOS {
	case "darwin":
		candidates = []candidate{
			{
				name: "Google Chrome",
				cmds: [][]string{
					{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "--version"},
					{"google-chrome", "--version"},
				},
			},
			{
				name: "Chromium",
				cmds: [][]string{
					{"/Applications/Chromium.app/Contents/MacOS/Chromium", "--version"},
					{"chromium", "--version"},
				},
			},
		}
	case "linux":
		candidates = []candidate{
			{
				name: "Google Chrome",
				cmds: [][]string{
					{"google-chrome", "--version"},
					{"google-chrome-stable", "--version"},
				},
			},
			{
				name: "Chromium",
				cmds: [][]string{
					{"chromium", "--version"},
					{"chromium-browser", "--version"},
				},
			},
		}
	case "windows":
		candidates = []candidate{
			{
				name: "Google Chrome",
				cmds: [][]string{
					{"reg", "query", `HKLM\SOFTWARE\Google\Chrome\BLBeacon`, "/v", "version"},
				},
			},
		}
	}

	for _, c := range candidates {
		for _, cmd := range c.cmds {
			out, err := exec.Command(cmd[0], cmd[1:]...).Output()
			if err != nil {
				continue
			}
			ver := parseVersionOutput(string(out))
			if ver == "" {
				continue
			}
			parts := strings.SplitN(ver, ".", 2)
			major, _ := strconv.Atoi(parts[0])
			browsers = append(browsers, Installed{
				Name:    c.name,
				Version: ver,
				Major:   major,
			})
			break
		}
	}

	return browsers
}

func parseVersionOutput(output string) string {
	output = strings.TrimSpace(output)
	fields := strings.Fields(output)
	for _, f := range fields {
		if len(f) > 0 && f[0] >= '0' && f[0] <= '9' && strings.Contains(f, ".") {
			return f
		}
	}
	return ""
}
