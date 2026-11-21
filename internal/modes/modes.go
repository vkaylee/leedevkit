package modes

// Service describes normalized service configuration loaded by Go TOML parsing.
type Service struct {
	Name     string `json:"name"`
	Language string `json:"language"`
}

// Resolve maps configured service languages to test modes and adds compatibility defaults.
func Resolve(services []Service) map[string]string {
	result := map[string]string{"all": "all"}
	for _, service := range services {
		switch service.Language {
		case "rust":
			result[service.Name] = "api"
		case "typescript", "javascript":
			result[service.Name] = "web"
		case "go":
			result[service.Name] = "go"
		}
	}
	for name, mode := range map[string]string{
		"apiserver":    "api",
		"agent-main":   "api",
		"webdashboard": "web",
		"api":          "api",
		"web":          "web",
		"go":           "go",
	} {
		if _, exists := result[name]; !exists {
			result[name] = mode
		}
	}
	return result
}
