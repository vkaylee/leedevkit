package lifecycle

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/vkaylee/leedevkit/internal/provider"
)

type Plan struct {
	Provider     provider.Provider `json:"provider"`
	Project      string            `json:"project"`
	Files        []string          `json:"files"`
	Profiles     []string          `json:"profiles"`
	Services     []string          `json:"services"`
	Dependencies []string          `json:"dependencies"`
	Build        bool              `json:"build"`
	SkipBuild    bool              `json:"skip_build"`
	NativeWait   bool              `json:"native_wait"`
	WaitTimeout  int               `json:"wait_timeout"`
	OwnsProject  bool              `json:"owns_project"`
	NoDeps       bool              `json:"no_deps"`
	OverrideFile string            `json:"override_file"`
	BuildPolicy  BuildPolicy       `json:"build_policy"`
}

func (p Plan) Up() ([]string, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	args := p.base()
	args = append(args, p.Profiles...)
	args = append(args, "up", "-d")
	if buildFlag := (BuildPolicy{
		Build:     p.Build || p.BuildPolicy.Build,
		SkipBuild: p.SkipBuild || p.BuildPolicy.SkipBuild,
	}).Flag(); buildFlag != "" {
		args = append(args, buildFlag)
	}
	if p.NativeWait {
		if !slices.Contains(p.Provider.Capabilities, "wait") {
			return nil, fmt.Errorf("provider %q lacks required capability: wait", p.Provider.Name)
		}
		args = append(args, "--wait", "--wait-timeout", strconv.Itoa(p.WaitTimeout))
	}
	if p.NoDeps {
		args = append(args, "--no-deps")
	}
	services := append([]string{}, p.Services...)
	if !p.NoDeps {
		services = append(services, p.Dependencies...)
	}
	return append(args, unique(services)...), nil
}

func (p Plan) Down() ([]string, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	if !p.OwnsProject {
		return nil, nil
	}
	args := p.base()
	args = append(args, p.Profiles...)
	if !hasProfile(p.Profiles, "*") {
		args = append(args, "--profile", "*")
	}
	return append(args, "down", "--remove-orphans", "--timeout", "1"), nil
}

func hasProfile(profiles []string, name string) bool {
	for index := 0; index+1 < len(profiles); index++ {
		if profiles[index] == "--profile" && profiles[index+1] == name {
			return true
		}
	}
	return false
}

func (p Plan) Args() ([]string, error) {
	return p.Up()
}

func (p Plan) base() []string {
	args := append([]string{}, p.Provider.Compose...)
	args = append(args, "-p", p.Project)
	for _, file := range p.Files {
		args = append(args, "-f", file)
	}
	if p.OverrideFile != "" {
		args = append(args, "-f", p.OverrideFile)
	}
	return args
}

func (p Plan) validate() error {
	for _, capability := range []string{"compose", "config", "up", "down", "ps"} {
		if !slices.Contains(p.Provider.Capabilities, capability) {
			return fmt.Errorf("provider %q lacks required capability: %s", p.Provider.Name, capability)
		}
	}
	if p.Project == "" {
		return fmt.Errorf("compose project must be non-empty")
	}
	if p.NativeWait && p.WaitTimeout <= 0 {
		return fmt.Errorf("wait timeout must be positive")
	}
	if !p.OwnsProject && p.NativeWait {
		return fmt.Errorf("external project cannot run lifecycle startup")
	}
	return nil
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
