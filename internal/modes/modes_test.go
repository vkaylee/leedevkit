package modes

import "testing"

func TestResolveMapsLanguagesAndDefaults(t *testing.T) {
	got := Resolve([]Service{
		{Name: "backend", Language: "go"},
		{Name: "frontend", Language: "typescript"},
		{Name: "api-server", Language: "rust"},
	})
	for name, want := range map[string]string{
		"backend":      "go",
		"frontend":     "web",
		"api-server":   "api",
		"apiserver":    "api",
		"webdashboard": "web",
		"all":          "all",
	} {
		if got[name] != want {
			t.Fatalf("mode[%q] = %q, want %q", name, got[name], want)
		}
	}
}

func TestResolveConfiguredServiceOverridesFallback(t *testing.T) {
	got := Resolve([]Service{{Name: "go", Language: "rust"}})
	if got["go"] != "api" {
		t.Fatalf("mode[go] = %q, want api", got["go"])
	}
}
