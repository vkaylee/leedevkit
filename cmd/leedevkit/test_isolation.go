package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vkaylee/leedevkit/internal/config"
	"github.com/vkaylee/leedevkit/internal/ownership"
	"github.com/vkaylee/leedevkit/internal/process"
	"github.com/vkaylee/leedevkit/internal/provider"
)

func validateDisposableTestProject(project string, env map[string]string, projectConfig config.Project) error {
	trimmed := strings.TrimSpace(project)
	if trimmed == "" {
		return fmt.Errorf("test Compose project name must be non-empty")
	}
	disallowed := []string{"leedevkit-dev"}
	if projectConfig.Namespace != "" {
		disallowed = append(disallowed, projectConfig.Namespace+"-dev")
	}
	if devProj := strings.TrimSpace(env["LEEDEVKIT_DEV_PROJECT"]); devProj != "" {
		disallowed = append(disallowed, devProj)
	}
	for _, devName := range disallowed {
		if trimmed == devName {
			return fmt.Errorf("refusing to run tests against dev Compose project %q; test runs must target a disposable test stack", trimmed)
		}
	}
	return nil
}

func resolveTestRunProject(env map[string]string, projectConfig config.Project) (string, bool, error) {
	if explicit := strings.TrimSpace(env["LEEDEVKIT_TEST_PROJECT"]); explicit != "" {
		if err := validateDisposableTestProject(explicit, env, projectConfig); err != nil {
			return "", true, err
		}
		return explicit, true, nil
	}
	prefix := "leedevkit-test"
	if projectConfig.Namespace != "" {
		prefix = projectConfig.Namespace + "-test"
	}
	generated, err := ownership.NewProjectName(prefix)
	if err != nil {
		return "", false, err
	}
	return generated, false, nil
}

func openExclusiveTestSession(project string, pinned bool) (*ownership.Session, error) {
	session, err := ownership.Open(project, true)
	if err != nil {
		if pinned {
			return nil, fmt.Errorf("test project %q is locked by another active test run; wait for it to finish or unset LEEDEVKIT_TEST_PROJECT for per-run isolation", project)
		}
		return nil, fmt.Errorf("failed to lock test project %q: %w", project, err)
	}
	return session, nil
}

func composeFileArgs(compose, files []string) []string {
	args := append([]string(nil), compose...)
	for _, file := range files {
		if file != "" {
			args = append(args, "-f", file)
		}
	}
	return args
}

type composeExternal bool

func (e *composeExternal) UnmarshalJSON(data []byte) error {
	var flag bool
	if err := json.Unmarshal(data, &flag); err == nil {
		*e = composeExternal(flag)
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		*e = composeExternal(len(obj) > 0)
		return nil
	}
	*e = false
	return nil
}

type composeConfigSnapshot struct {
	Services map[string]struct {
		Environment map[string]*string `json:"environment"`
	} `json:"services"`
	Volumes map[string]struct {
		Name       string            `json:"name"`
		External   composeExternal   `json:"external"`
		Driver     string            `json:"driver"`
		Labels     map[string]string `json:"labels"`
		DriverOpts map[string]string `json:"driver_opts"`
	} `json:"volumes"`
}

type composeTestRuntime struct {
	OverrideFile string
	ServiceEnv   map[string]map[string]string
	volumeLocks  []*ownership.Session
}

func (r *composeTestRuntime) Close() {
	if r == nil {
		return
	}
	if r.OverrideFile != "" {
		_ = os.Remove(r.OverrideFile)
		r.OverrideFile = ""
	}
	for _, lock := range r.volumeLocks {
		_ = lock.Close(nil)
	}
	r.volumeLocks = nil
}

func (r *composeTestRuntime) ComposeFiles(base []string) []string {
	files := append([]string(nil), base...)
	if r != nil && r.OverrideFile != "" {
		files = append(files, r.OverrideFile)
	}
	return files
}

func (r *composeTestRuntime) LookupEnvFor(service string, hostEnv map[string]string) func(string) (bool, bool) {
	return func(name string) (bool, bool) {
		if val, ok := hostEnv[name]; ok && val != "" {
			return true, true
		}
		if r != nil {
			if svcEnv := r.ServiceEnv[service]; svcEnv != nil {
				if val, ok := svcEnv[name]; ok && val != "" {
					return true, false
				}
			}
		}
		return false, false
	}
}

type foreignVolumeOverride struct {
	Key              string
	Name             string
	NeedsOverrideTag bool
}

func fixedVolumeLockProject(volumeName string) string {
	return fmt.Sprintf("leedevkit-vol-%08x", fnv32a(volumeName))
}

// prepareComposeTestRuntime inspects resolved Compose services and volumes to:
//  1. collect container-level service env vars for required_env verification,
//  2. lock any non-external fixed-name volumes so concurrent test runs cannot collide, and
//  3. generate a non-destructive external override for already-existing volumes created by a
//     different Compose project so ownership warnings disappear without deleting user data.
func prepareComposeTestRuntime(
	ctx context.Context,
	providerConfig provider.Provider,
	project string,
	files []string,
	dir string,
	env []string,
	run func(context.Context, process.Request) process.Result,
) (*composeTestRuntime, error) {
	rt := &composeTestRuntime{ServiceEnv: map[string]map[string]string{}}
	if len(providerConfig.Compose) == 0 || !hasCapability(providerConfig.Capabilities, "config") || len(files) == 0 {
		return rt, nil
	}
	if run == nil {
		run = process.Run
	}
	args := append(composeBaseArgs(providerConfig.Compose, project, files), "--profile", "*", "config", "--format", "json")
	var stdout, stderr bytes.Buffer
	res := run(ctx, process.Request{Args: args, Dir: dir, Env: append([]string(nil), env...), Stdout: &stdout, Stderr: &stderr, Timeout: 15 * time.Second})
	if res.ExitCode != 0 {
		return rt, nil
	}
	raw := res.Stdout
	if len(raw) == 0 {
		raw = stdout.Bytes()
	}
	var snapshot composeConfigSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return rt, nil
	}
	for svcName, svc := range snapshot.Services {
		if len(svc.Environment) == 0 {
			continue
		}
		svcMap := make(map[string]string, len(svc.Environment))
		for k, v := range svc.Environment {
			if v != nil && *v != "" {
				svcMap[k] = *v
			}
		}
		if len(svcMap) > 0 {
			rt.ServiceEnv[svcName] = svcMap
		}
	}
	volKeys := make([]string, 0, len(snapshot.Volumes))
	for key := range snapshot.Volumes {
		volKeys = append(volKeys, key)
	}
	sort.Strings(volKeys)

	var foreign []foreignVolumeOverride
	for _, volKey := range volKeys {
		vol := snapshot.Volumes[volKey]
		if bool(vol.External) || vol.Name == "" {
			continue
		}
		if vol.Name == project+"_"+volKey || strings.HasPrefix(vol.Name, project+"_") {
			continue
		}
		// Non-external fixed-name volume cannot be isolated by Compose project alone; lock it explicitly.
		lock, err := ownership.Open(fixedVolumeLockProject(vol.Name), true)
		if err != nil {
			rt.Close()
			return nil, fmt.Errorf("shared Compose volume %q is in use by another active test run; remove fixed volume name for per-run isolation or mark shared cache volumes external: true", vol.Name)
		}
		rt.volumeLocks = append(rt.volumeLocks, lock)

		if len(providerConfig.CLI) == 0 {
			continue
		}
		inspectArgs := append(append([]string(nil), providerConfig.CLI...), "volume", "inspect", "--format", `{{index .Labels "com.docker.compose.project"}}`, vol.Name)
		var inspectOut, inspectErr bytes.Buffer
		inspectRes := run(ctx, process.Request{Args: inspectArgs, Dir: dir, Env: append([]string(nil), env...), Stdout: &inspectOut, Stderr: &inspectErr, Timeout: 10 * time.Second})
		if inspectRes.ExitCode != 0 {
			continue
		}
		ownerRaw := inspectRes.Stdout
		if len(ownerRaw) == 0 {
			ownerRaw = inspectOut.Bytes()
		}
		owner := strings.TrimSpace(string(ownerRaw))
		if owner != project {
			foreign = append(foreign, foreignVolumeOverride{
				Key:              volKey,
				Name:             vol.Name,
				NeedsOverrideTag: vol.Driver != "" || len(vol.Labels) > 0 || len(vol.DriverOpts) > 0,
			})
		}
	}
	if len(foreign) > 0 {
		overridePath, err := writeVolumeExternalOverride(foreign)
		if err != nil {
			rt.Close()
			return nil, err
		}
		rt.OverrideFile = overridePath
	}
	return rt, nil
}

func writeVolumeExternalOverride(entries []foreignVolumeOverride) (string, error) {
	file, err := os.CreateTemp("", "leedevkit-vol-override-*.yml")
	if err != nil {
		return "", fmt.Errorf("create compose volume override: %w", err)
	}
	var buf strings.Builder
	buf.WriteString("volumes:\n")
	for _, entry := range entries {
		key := strconv.Quote(entry.Key)
		if entry.NeedsOverrideTag {
			fmt.Fprintf(&buf, "  %s: !override\n", key)
		} else {
			fmt.Fprintf(&buf, "  %s:\n", key)
		}
		fmt.Fprintf(&buf, "    name: %s\n", strconv.Quote(entry.Name))
		buf.WriteString("    external: true\n")
	}
	if _, err := file.WriteString(buf.String()); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("write compose volume override: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("close compose volume override: %w", err)
	}
	return file.Name(), nil
}
