//go:build mage
// +build mage

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	// mage:import
	build "github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
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

// getBuildInfoFromEnvironment reads the
func getBuildInfoFromEnvironment() Info {
	v := Info{
		Time: time.Now().UnixNano() / int64(time.Millisecond),
	}

	v.Repo = getEnvironment(
		"DRONE_REPO_LINK",
		"CIRCLE_PROJECT_REPONAME",
		"CI_REPONAME",
		"> git remote get-url origin")
	v.Branch = getEnvironment(
		"DRONE_BRANCH",
		"CIRCLE_BRANCH",
		"CI_BRANCH",
		"> git branch --show-current")
	v.Hash = getEnvironment(
		"DRONE_COMMIT_SHA",
		"CIRCLE_SHA1",
		"CI_COMMIT_SHA",
		"> git rev-parse HEAD")
	val, err := strconv.ParseInt(getEnvironment(
		"DRONE_BUILD_NUMBER",
		"CIRCLE_BUILD_NUM",
		"CI_BUILD_NUM"), 10, 64)
	if err == nil {
		v.Build = val
	}
	val, err = strconv.ParseInt(getEnvironment(
		"DRONE_PULL_REQUEST",
		"CI_PULL_REQUEST"), 10, 64)
	if err == nil {
		v.PR = val
	}
	return v
}

var exname string

// Callbacks give you a way to run custom behavior when things happen
var beforeBuild = func(cfg build.Config) (build.Config, error) {
	return cfg, nil
}

func getExecutableName(os string, arch string, pluginJSONPath string) (string, error) {
	if exname == "" {
		exename, err := GetExecutableFromPluginJSON(pluginJSONPath)
		if err != nil {
			return "", err
		}

		exname = exename
	}

	exeName := fmt.Sprintf("%s_%s_%s", exname, os, arch)
	if os == "windows" {
		exeName = fmt.Sprintf("%s.exe", exeName)
	}
	return exeName, nil
}

// Info See also PluginBuildInfo in https://github.com/grafana/grafana/blob/master/pkg/plugins/models.go
type Info struct {
	Time     int64  `json:"time,omitempty"`
	PluginID string `json:"pluginID,omitempty"`
	Version  string `json:"version,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Hash     string `json:"hash,omitempty"`
	Build    int64  `json:"build,omitempty"`
	PR       int64  `json:"pr,omitempty"`
}

// this will append build flags -- the keys are picked to match existing
// grafana build flags from bra
func (v Info) appendFlags(flags map[string]string) {
	if v.PluginID != "" {
		flags["main.pluginID"] = v.PluginID
	}
	if v.Version != "" {
		flags["main.version"] = v.Version
	}
	if v.Branch != "" {
		flags["main.branch"] = v.Branch
	}
	if v.Hash != "" {
		flags["main.commit"] = v.Hash
	}

	out, err := json.Marshal(v)
	if err == nil {
		flags["github.com/grafana/grafana-plugin-sdk-go/build.buildInfoJSON"] = string(out)
	}
}

func buildBackend(cfg build.Config) error {
	cfg, err := beforeBuild(cfg)
	if err != nil {
		return err
	}

	pluginJSONPath := defaultPluginJSONPath
	if cfg.PluginJSONPath != "" {
		pluginJSONPath = cfg.PluginJSONPath
	}
	exeName, err := getExecutableName(cfg.OS, cfg.Arch, pluginJSONPath)
	if err != nil {
		return err
	}

	ldFlags := ""
	if !cfg.EnableCGo {
		// Link statically
		ldFlags = `-extldflags "-static"`
	}

	if !cfg.EnableDebug {
		// Add linker flags to drop debug information
		prefix := ""
		if ldFlags != "" {
			prefix = " "
		}
		ldFlags = fmt.Sprintf("-w -s%s%s", prefix, ldFlags)
	}

	outputPath := cfg.OutputBinaryPath
	if outputPath == "" {
		outputPath = defaultOutputBinaryPath
	}
	args := []string{
		"build", "-o", filepath.Join(outputPath, exeName),
	}

	info := getBuildInfoFromEnvironment()
	pluginID, err := GetStringValueFromJSON(filepath.Join(pluginJSONPath, "plugin.json"), "id")
	if err == nil && len(pluginID) > 0 {
		info.PluginID = pluginID
	}
	version, err := GetStringValueFromJSON("package.json", "version")
	if err == nil && len(version) > 0 {
		info.Version = version
	}

	flags := make(map[string]string, 10)
	info.appendFlags(flags)

	if cfg.CustomVars != nil {
		for k, v := range cfg.CustomVars {
			flags[k] = v
		}
	}

	for k, v := range flags {
		ldFlags = fmt.Sprintf("%s -X '%s=%s'", ldFlags, k, v)
	}
	// args = append(args, "-tags=duckdb_use_lib")
	args = append(args, "-extldflags", "-static-libstdc++", "-ldflags", ldFlags)

	if cfg.EnableDebug {
		args = append(args, "-gcflags=all=-N -l")
	}
	rootPackage := "./pkg"
	if cfg.RootPackagePath != "" {
		rootPackage = cfg.RootPackagePath
	}
	args = append(args, rootPackage)

	cfg.Env["GOARCH"] = cfg.Arch
	cfg.Env["GOOS"] = cfg.OS
	if !cfg.EnableCGo {
		cfg.Env["CGO_ENABLED"] = "0"
	}
	// cfg.Env["CGO_LDFLAGS"] = "-L/Users/louisa/duckdbs/0.9.2/libduckdb-linux-amd64/ -L"

	// TODO: Change to sh.RunWithV once available.
	return sh.RunWith(cfg.Env, "go", args...)
}

// // GenerateManifestFile generates a manifest file for plugin submissions
// func (Build) GenerateManifestFile() error {
// 	config := build.Config{}
// 	config, err := beforeBuild(config)
// 	if err != nil {
// 		return err
// 	}
// 	outputPath := config.OutputBinaryPath
// 	if outputPath == "" {
// 		outputPath = defaultOutputBinaryPath
// 	}
// 	manifestContent, err := utils.GenerateManifest()
// 	if err != nil {
// 		return err
// 	}

// 	manifestFilePath := filepath.Join(outputPath, "go_plugin_build_manifest")
// 	err = os.MkdirAll(outputPath, 0755)
// 	if err != nil {
// 		return err
// 	}
// 	// #nosec G306 - we need reading permissions for this file
// 	err = os.WriteFile(manifestFilePath, []byte(manifestContent), 0755)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func newBuildConfig(os string, arch string, enableCGo bool) build.Config {
	return build.Config{
		OS:          os,
		Arch:        arch,
		EnableDebug: false,
		Env:         map[string]string{},
		EnableCGo:   enableCGo,
	}
}

// Linux builds the back-end plugin for Linux.
func (Build) LinuxS() error {
	return buildBackend(newBuildConfig("linux", "amd64", true))
}

// LinuxARM builds the back-end plugin for Linux on ARM.
func (Build) LinuxARMS() error {
	return buildBackend(newBuildConfig("linux", "arm", false))
}

// LinuxARM64 builds the back-end plugin for Linux on ARM64.
func (Build) LinuxARM64S() error {
	return buildBackend(newBuildConfig("linux", "arm64", false))
}

// Windows builds the back-end plugin for Windows.
func (Build) WindowsS() error {
	return buildBackend(newBuildConfig("windows", "amd64", false))
}

// Darwin builds the back-end plugin for OSX on AMD64.
func (Build) DarwinS() error {
	return buildBackend(newBuildConfig("darwin", "amd64", false))
}

// DarwinARM64 builds the back-end plugin for OSX on ARM (M1/M2).
func (Build) DarwinARM64S() error {
	return buildBackend(newBuildConfig("darwin", "arm64", false))
}

// BuildAll builds production executables for all supported platforms.
func BuildAllS() { //revive:disable-line
	b := Build{}
	mg.Deps(b.LinuxS, b.WindowsS, b.DarwinS, b.DarwinARM64S, b.LinuxARM64S, b.LinuxARMS, build.Build.GenerateManifestFile)
}

// Default configures the default target.
var Default = BuildAllS

// func (Build) MacLocal() error {
// 	return buildBackend(build.Config{
// 		OS:          "darwin",
// 		Arch:        "arm64",
// 		EnableDebug: true,
// 		Env:         map[string]string{},
// 		EnableCGo:   true,
// 	})
// }
