package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Treat non-numeric components as 0 — good enough for semver comparison.
func versLE(a, b string) bool {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < len(ap) || i < len(bp); i++ {
		var ai, bi int
		if i < len(ap) {
			ai, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bi, _ = strconv.Atoi(bp[i])
		}
		if ai > bi {
			return false
		}
		if ai < bi {
			return true
		}
	}
	return true
}

const (
	repoOwner = "KabosuNeko"
	repoName  = "Futon"
)

const installScript = "curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh -o /tmp/futon_install.sh && bash /tmp/futon_install.sh && rm /tmp/futon_install.sh"

// InstallScriptCommand returns the install.sh invocation shared by the CLI
// updater and the TUI update flow. Callers wire stdin/stdout/stderr.
func InstallScriptCommand() *exec.Cmd {
	return exec.Command("bash", "-c", installScript)
}

var apiURL = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"

type releaseInfo struct {
	TagName string `json:"tag_name"`
}

// CheckForUpdate reports whether the latest GitHub release is newer than
// currentVersion, returning the release tag when one is available.
func CheckForUpdate(currentVersion string) (bool, string, error) {
	if currentVersion == "dev" {
		return false, "", nil
	}

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("check update failed, HTTP status: %d", resp.StatusCode)
	}

	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return false, "", fmt.Errorf("failed to parse release info: %w", err)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	if versLE(latest, current) {
		return false, "", nil
	}

	return true, rel.TagName, nil
}
