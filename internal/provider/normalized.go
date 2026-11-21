package provider

import "fmt"

func Normalize(input ProtocolProvider) (Provider, error) {
	if _, err := ValidateProtocol(input); err != nil {
		return Provider{}, err
	}
	caps := append([]string(nil), input.Capabilities...)
	return Provider{Name: input.Name, CLI: append([]string(nil), input.CLI...), Compose: append([]string(nil), input.Compose...), Capabilities: caps, Settings: input.Settings, Source: input.Source}, nil
}

func Require(p Provider, capabilities ...string) error {
	for _, wanted := range capabilities {
		found := false
		for _, capability := range p.Capabilities {
			if capability == wanted {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("provider %q lacks required capability: %s", p.Name, wanted)
		}
	}
	return nil
}
