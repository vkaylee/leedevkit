package testplan

import "fmt"

type ProtocolRequest struct {
	SchemaVersion int     `json:"schema_version"`
	Flags         Flags   `json:"flags"`
	Plan          Request `json:"plan"`
}

type ProtocolResponse struct {
	SchemaVersion int       `json:"schema_version"`
	Result        RunResult `json:"result"`
}

func ValidateProtocol(request ProtocolRequest) error {
	if request.SchemaVersion != 1 {
		return fmt.Errorf("unsupported test schema version: %d", request.SchemaVersion)
	}
	if request.Flags.Target == "" {
		return fmt.Errorf("test target is required")
	}
	return nil
}
