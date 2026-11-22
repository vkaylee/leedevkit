package debug

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestProcessComposeRunnersPreserveOutputAndErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run := ProcessComposeRunner(&stdout, &stderr)
	if err := run(context.Background(), []string{"sh", "-c", "printf stdout; printf stderr >&2"}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "stdout" || stderr.String() != "stderr" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if err := run(context.Background(), []string{"sh", "-c", "exit 7"}); err == nil {
		t.Fatal("failing command accepted")
	}
	output := ProcessComposeOutputRunner()
	data, err := output(context.Background(), []string{"sh", "-c", "printf result"})
	if err != nil || string(data) != "result" {
		t.Fatalf("output=%q error=%v", data, err)
	}
	if _, err := output(context.Background(), []string{"sh", "-c", "exit 5"}); err == nil {
		t.Fatal("failing output command accepted")
	}
}

func TestValidateComposeConfigRejectsMissingApprovedResources(t *testing.T) {
	environment := Environment{Networks: []string{"backend"}, Services: map[string]Service{"api": {ID: "api", Host: "api"}}}
	for _, tc := range []struct {
		config map[string]any
		want   string
	}{
		{map[string]any{}, "no services"},
		{map[string]any{"services": map[string]any{"api": map[string]any{}}}, "no networks"},
		{map[string]any{"services": map[string]any{"api": map[string]any{}}, "networks": map[string]any{}}, "approved network"},
		{map[string]any{"services": map[string]any{"other": map[string]any{}}, "networks": map[string]any{"backend": map[string]any{}}}, "approved service"},
	} {
		if err := ValidateComposeConfig(tc.config, environment); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("config=%v error=%v want=%q", tc.config, err, tc.want)
		}
	}
}
