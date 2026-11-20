package targets

// Alias describes one configured logical target and its services.
type Alias struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

// Request contains normalized target configuration. Python owns TOML loading.
type Request struct {
	Target      string   `json:"target"`
	AllServices []string `json:"all_services"`
	Aliases     []Alias  `json:"aliases"`
}

// Resolve expands one logical target using stable configuration order.
func Resolve(request Request) []string {
	if request.Target != "all" {
		return []string{request.Target}
	}

	selected := make([]string, 0, len(request.Aliases)+len(request.AllServices))
	covered := make(map[string]bool, len(request.AllServices))
	allServices := make([]string, 0, len(request.AllServices))
	for _, service := range request.AllServices {
		service = string(service)
		allServices = append(allServices, service)
	}
	for _, alias := range request.Aliases {
		services := make(map[string]bool, len(alias.Services))
		for _, service := range alias.Services {
			services[service] = true
		}
		matched := false
		for _, service := range allServices {
			if services[service] {
				matched = true
				covered[service] = true
			}
		}
		if matched {
			selected = append(selected, alias.Name)
		}
	}
	for _, service := range allServices {
		if !covered[service] && !contains(selected, service) {
			selected = append(selected, service)
			covered[service] = true
		}
	}
	if len(selected) == 0 {
		return []string{"all"}
	}
	return selected
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
