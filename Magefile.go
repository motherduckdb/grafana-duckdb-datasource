//go:build mage
// +build mage

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	// mage:import
	build "github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/magefile/mage/mg"
)

var defaultOutputBinaryPath = "dist"
var defaultPluginJSONPath = "src"

// Build is a namespace.
type Build mg.Namespace

func GetStringValueFromJSON(fpath string, key string) (string, error) {
	byteValue, err := os.ReadFile(fpath)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	err = json.Unmarshal(byteValue, &result)
	if err != nil {
		return "", err
	}
	executable := result[key]
	name, ok := executable.(string)
	if !ok || name == "" {
		return "", fmt.Errorf("plugin.json is missing: %s", key)
	}
	return name, nil
}

func GetExecutableFromPluginJSON(dir string) (string, error) {
	exe, err := GetStringValueFromJSON(path.Join(dir, "plugin.json"), "executable")
	if err != nil {
		// In app plugins, the exe may be nested
		exe, err2 := GetStringValueFromJSON(path.Join(dir, "datasource", "plugin.json"), "executable")
		if err2 == nil {
			if !strings.HasPrefix(exe, "../") {
				return "", fmt.Errorf("datasource should reference executable in root folder")
			}
			return exe[3:], nil
		}
	}
	return exe, err
}

func getEnvironment(check ...string) string {
	for _, key := range check {
		if strings.HasPrefix(key, "> ") {
			parts := strings.Split(key, " ")
			cmd := exec.Command(parts[1], parts[2:]...) // #nosec G204
			out, err := cmd.CombinedOutput()
			if err == nil && len(out) > 0 {
				str := strings.TrimSpace(string(out))
				if strings.Index(str, " ") > 0 {
					continue // skip any output that has spaces
				}
				return str
			}
			continue
		}

		val := os.Getenv(key)
		if val != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

var Default = build.BuildAll
