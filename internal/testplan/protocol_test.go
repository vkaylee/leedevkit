package testplan

import "testing"

func TestValidateProtocol(t *testing.T) {
	if err := ValidateProtocol(ProtocolRequest{SchemaVersion: 1, Flags: Flags{Target: "all"}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProtocol(ProtocolRequest{SchemaVersion: 2, Flags: Flags{Target: "all"}}); err == nil {
		t.Fatal("expected schema error")
	}
	if err := ValidateProtocol(ProtocolRequest{SchemaVersion: 1}); err == nil {
		t.Fatal("expected target error")
	}
}
