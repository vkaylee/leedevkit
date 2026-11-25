package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/vkaylee/leedevkit/internal/gates"
	"github.com/vkaylee/leedevkit/internal/lifecycle"
	"github.com/vkaylee/leedevkit/internal/modes"
	"github.com/vkaylee/leedevkit/internal/provider"
	runplan "github.com/vkaylee/leedevkit/internal/run"
	"github.com/vkaylee/leedevkit/internal/targets"
	"github.com/vkaylee/leedevkit/internal/testplan"
)

type gateProtocolRequest struct {
	SchemaVersion int                   `json:"schema_version"`
	Gates         []gates.Gate          `json:"gates"`
	Names         []string              `json:"names"`
	Definitions   map[string]gates.Gate `json:"definitions"`
}

type gateProtocolResponse struct {
	SchemaVersion int          `json:"schema_version"`
	Gates         []gates.Gate `json:"gates"`
}

type targetProtocolRequest struct {
	SchemaVersion int                   `json:"schema_version"`
	Target        string                `json:"target"`
	AllServices   []string              `json:"all_services"`
	Aliases       []targetProtocolAlias `json:"aliases"`
}

type targetProtocolAlias struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

type targetProtocolResponse struct {
	SchemaVersion int      `json:"schema_version"`
	Targets       []string `json:"targets"`
}

type modeProtocolRequest struct {
	SchemaVersion int                   `json:"schema_version"`
	Services      []modeProtocolService `json:"services"`
}

type modeProtocolService struct {
	Name     string `json:"name"`
	Language string `json:"language"`
}

type modeProtocolResponse struct {
	SchemaVersion int               `json:"schema_version"`
	Modes         map[string]string `json:"modes"`
}

type providerProtocolRequest struct {
	SchemaVersion int                       `json:"schema_version"`
	Provider      provider.ProtocolProvider `json:"provider"`
}

type providerProtocolResponse struct {
	SchemaVersion int            `json:"schema_version"`
	Diagnostics   map[string]any `json:"diagnostics"`
}

type lifecycleProtocolRequest struct {
	SchemaVersion int            `json:"schema_version"`
	Operation     string         `json:"operation"`
	Plan          lifecycle.Plan `json:"plan"`
}

type lifecycleProtocolResponse struct {
	SchemaVersion int      `json:"schema_version"`
	Operation     string   `json:"operation"`
	Args          []string `json:"args"`
}

type runProtocolRequest struct {
	SchemaVersion int             `json:"schema_version"`
	Plan          runplan.Request `json:"plan"`
}

type runProtocolResponse struct {
	SchemaVersion int      `json:"schema_version"`
	Args          []string `json:"args"`
}

type testProtocolResponse struct {
	SchemaVersion int                `json:"schema_version"`
	Result        testplan.RunResult `json:"result"`
}

func decodeSingleJSON(input io.Reader, target any) error {
	decoder := json.NewDecoder(input)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return fmt.Errorf("trailing JSON data: %w", err)
	}
	return nil
}

func handleProtocolJSON[Req any, Resp any](stdin io.Reader, stdout, stderr io.Writer, name string, schemaVersion func(*Req) int, handle func(Req) (Resp, error)) int {
	var request Req
	if err := decodeSingleJSON(stdin, &request); err != nil {
		fmt.Fprintf(stderr, "malformed %s request: %v\n", name, err)
		return 2
	}
	if schemaVersion != nil {
		if v := schemaVersion(&request); v != 1 {
			fmt.Fprintf(stderr, "unsupported %s schema version: %d\n", name, v)
			return 2
		}
	}
	response, err := handle(request)
	if err != nil {
		if schemaVersion != nil {
			fmt.Fprintf(stderr, "invalid %s request: %v\n", name, err)
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 2
	}
	if err := json.NewEncoder(stdout).Encode(response); err != nil {
		fmt.Fprintf(stderr, "failed to write %s response: %v\n", name, err)
		return 2
	}
	return 0
}

func dispatchProtocolFlag(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, bool) {
	if len(args) != 1 {
		return 0, false
	}
	switch args[0] {
	case "--plan-lifecycle":
		return handleProtocolJSON(stdin, stdout, stderr, "lifecycle", func(r *lifecycleProtocolRequest) int { return r.SchemaVersion }, func(request lifecycleProtocolRequest) (lifecycleProtocolResponse, error) {
			operation := request.Operation
			if operation == "" {
				operation = "up"
			}
			var planned []string
			var err error
			switch operation {
			case "up":
				planned, err = request.Plan.Up()
			case "down":
				planned, err = request.Plan.Down()
			default:
				err = fmt.Errorf("unsupported lifecycle operation %q", operation)
			}
			if err != nil {
				return lifecycleProtocolResponse{}, err
			}
			return lifecycleProtocolResponse{SchemaVersion: 1, Operation: operation, Args: planned}, nil
		}), true
	case "--plan-run":
		return handleProtocolJSON(stdin, stdout, stderr, "run", func(r *runProtocolRequest) int { return r.SchemaVersion }, func(request runProtocolRequest) (runProtocolResponse, error) {
			planned, err := runplan.Plan(request.Plan)
			if err != nil {
				return runProtocolResponse{}, err
			}
			return runProtocolResponse{SchemaVersion: 1, Args: planned}, nil
		}), true
	case "--plan-test":
		return handleProtocolJSON(stdin, stdout, stderr, "test", nil, func(request testplan.ProtocolRequest) (testProtocolResponse, error) {
			if err := testplan.ValidateProtocol(request); err != nil {
				return testProtocolResponse{}, err
			}
			return testProtocolResponse{SchemaVersion: 1, Result: testplan.Run(request.Flags, nil, nil)}, nil
		}), true
	case "--resolve-modes":
		return handleProtocolJSON(stdin, stdout, stderr, "mode", func(r *modeProtocolRequest) int { return r.SchemaVersion }, func(request modeProtocolRequest) (modeProtocolResponse, error) {
			services := make([]modes.Service, len(request.Services))
			for index, service := range request.Services {
				services[index] = modes.Service{Name: service.Name, Language: service.Language}
			}
			return modeProtocolResponse{SchemaVersion: 1, Modes: modes.Resolve(services)}, nil
		}), true
	case "--resolve-test-targets":
		return handleProtocolJSON(stdin, stdout, stderr, "test target", func(r *targetProtocolRequest) int { return r.SchemaVersion }, func(request targetProtocolRequest) (targetProtocolResponse, error) {
			aliases := make([]targets.Alias, len(request.Aliases))
			for index, alias := range request.Aliases {
				aliases[index] = targets.Alias{Name: alias.Name, Services: alias.Services}
			}
			return targetProtocolResponse{
				SchemaVersion: 1,
				Targets:       targets.Resolve(targets.Request{Target: request.Target, AllServices: request.AllServices, Aliases: aliases}),
			}, nil
		}), true
	case "--resolve-provider":
		return handleProtocolJSON(stdin, stdout, stderr, "provider", func(r *providerProtocolRequest) int { return r.SchemaVersion }, func(request providerProtocolRequest) (providerProtocolResponse, error) {
			diagnostics, err := provider.ValidateProtocol(request.Provider)
			if err != nil {
				return providerProtocolResponse{}, err
			}
			return providerProtocolResponse{SchemaVersion: 1, Diagnostics: diagnostics}, nil
		}), true
	case "--plan-gates":
		return handleProtocolJSON(stdin, stdout, stderr, "gate", func(r *gateProtocolRequest) int { return r.SchemaVersion }, func(request gateProtocolRequest) (gateProtocolResponse, error) {
			names := request.Names
			definitions := request.Definitions
			if len(names) == 0 && len(request.Gates) > 0 {
				definitions = make(map[string]gates.Gate, len(request.Gates))
				for _, gate := range request.Gates {
					definitions[gate.Name] = gate
					names = append(names, gate.Name)
				}
			}
			selected, err := gates.Select(names, definitions)
			if err != nil {
				return gateProtocolResponse{}, err
			}
			return gateProtocolResponse{SchemaVersion: 1, Gates: selected}, nil
		}), true
	default:
		return 0, false
	}
}
