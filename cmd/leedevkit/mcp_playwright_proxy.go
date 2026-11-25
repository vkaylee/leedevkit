package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/vkaylee/leedevkit/internal/process"
)

type playwrightProxyOptions struct {
	Command            []string
	Dir                string
	Stdin              io.Reader
	Stdout             io.Writer
	Stderr             io.Writer
	Instructions       string
	StartupTimeout     time.Duration
	RequestTimeout     time.Duration
	SessionIdleTimeout time.Duration
	Grace              time.Duration
	CleanupContainer   func()
}

type stdioLine struct {
	data []byte
	eof  bool
	err  error
}

func readLines(r io.Reader, ch chan<- stdioLine) {
	defer close(ch)
	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			ch <- stdioLine{data: line}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				ch <- stdioLine{eof: true}
			} else {
				ch <- stdioLine{err: err}
			}
			return
		}
	}
}

func parseJSONRPCMessages(line []byte) ([]map[string]any, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, false
	}
	var single map[string]any
	if err := json.Unmarshal(trimmed, &single); err == nil && single != nil {
		return []map[string]any{single}, false
	}
	var batch []map[string]any
	if err := json.Unmarshal(trimmed, &batch); err == nil && len(batch) > 0 {
		return batch, true
	}
	return nil, false
}

func canonicalMessageID(msg map[string]any) (string, bool) {
	raw, ok := msg["id"]
	if !ok {
		return "", false
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func stopTimer(t *time.Timer) {
	if t == nil {
		return
	}
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

func resetTimer(t **time.Timer, d time.Duration) <-chan time.Time {
	if d <= 0 {
		return nil
	}
	if *t == nil {
		*t = time.NewTimer(d)
	} else {
		stopTimer(*t)
		(*t).Reset(d)
	}
	return (*t).C
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	if l.w == nil {
		return len(p), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

type playwrightProxyState struct {
	opts          playwrightProxyOptions
	grace         time.Duration
	cmd           *exec.Cmd
	childStdin    io.WriteCloser
	initialized   bool
	terminating   bool
	timedOut      bool
	stdinClosed   bool
	cleanupRan    bool
	pending       map[string]struct{}
	initializeIDs map[string]struct{}
	toolIDs       map[string]struct{}
	startupTimer  *time.Timer
	requestTimer  *time.Timer
	idleTimer     *time.Timer
	eofTimer      *time.Timer
	killTimer     *time.Timer
	startupCh     <-chan time.Time
	requestCh     <-chan time.Time
	idleCh        <-chan time.Time
	eofCh         <-chan time.Time
	killCh        <-chan time.Time
}

func (s *playwrightProxyState) closeChildStdin() {
	if !s.stdinClosed {
		s.stdinClosed = true
		_ = s.childStdin.Close()
	}
}

func (s *playwrightProxyState) armStartup() {
	s.startupCh = resetTimer(&s.startupTimer, s.opts.StartupTimeout)
}

func (s *playwrightProxyState) armRequest() {
	s.requestCh = resetTimer(&s.requestTimer, s.opts.RequestTimeout)
}

func (s *playwrightProxyState) disarmRequest() {
	stopTimer(s.requestTimer)
	s.requestCh = nil
}

func (s *playwrightProxyState) armIdle() {
	s.idleCh = resetTimer(&s.idleTimer, s.opts.SessionIdleTimeout)
}

func (s *playwrightProxyState) terminate() {
	if s.terminating {
		return
	}
	s.terminating = true
	s.closeChildStdin()
	stopTimer(s.startupTimer)
	s.startupCh = nil
	s.disarmRequest()
	stopTimer(s.idleTimer)
	s.idleCh = nil
	stopTimer(s.eofTimer)
	s.eofCh = nil
	_ = process.SignalProcessGroup(s.cmd.Process, false)
	s.killCh = resetTimer(&s.killTimer, s.grace)
}

func (s *playwrightProxyState) stopAllTimers() {
	stopTimer(s.startupTimer)
	stopTimer(s.requestTimer)
	stopTimer(s.idleTimer)
	stopTimer(s.eofTimer)
	stopTimer(s.killTimer)
	s.closeChildStdin()
}

func (s *playwrightProxyState) runCleanup() {
	if s.cleanupRan || s.opts.CleanupContainer == nil {
		return
	}
	s.cleanupRan = true
	s.opts.CleanupContainer()
}

func (s *playwrightProxyState) handleRequestLine(raw []byte) {
	if s.terminating {
		return
	}
	if _, err := s.childStdin.Write(raw); err != nil {
		s.terminate()
		return
	}
	msgs, _ := parseJSONRPCMessages(raw)
	for _, msg := range msgs {
		id, hasID := canonicalMessageID(msg)
		method, hasMethod := msg["method"].(string)
		if !hasID || !hasMethod {
			continue
		}
		if s.initialized && len(s.pending) == 0 {
			s.armRequest()
		}
		switch method {
		case "initialize":
			s.initializeIDs[id] = struct{}{}
		case "tools/call":
			s.toolIDs[id] = struct{}{}
		}
		s.pending[id] = struct{}{}
	}
}

func (s *playwrightProxyState) handleResponseLine(raw []byte) {
	msgs, isBatch := parseJSONRPCMessages(raw)
	modified := false
	for _, msg := range msgs {
		id, hasID := canonicalMessageID(msg)
		if !hasID {
			continue
		}
		if _, hasMethod := msg["method"]; hasMethod {
			continue
		}
		if _, ok := s.pending[id]; !ok {
			continue
		}
		delete(s.pending, id)
		if _, isTool := s.toolIDs[id]; isTool {
			delete(s.toolIDs, id)
			if s.initialized {
				s.armIdle()
			}
		}
		if _, isInit := s.initializeIDs[id]; !isInit {
			continue
		}
		delete(s.initializeIDs, id)
		resMap, ok := msg["result"].(map[string]any)
		if !ok {
			continue
		}
		s.initialized = true
		stopTimer(s.startupTimer)
		s.startupCh = nil
		s.armIdle()
		if s.opts.Instructions != "" {
			resMap["instructions"] = s.opts.Instructions
			modified = true
		}
	}

	outgoing := raw
	if modified {
		var payload any = msgs[0]
		if isBatch {
			payload = msgs
		}
		if encoded, encErr := json.Marshal(payload); encErr == nil {
			outgoing = append(encoded, '\n')
		}
	}
	_, _ = s.opts.Stdout.Write(outgoing)

	if s.initialized {
		if len(s.pending) == 0 {
			s.disarmRequest()
		} else {
			s.armRequest()
		}
	}
}

func servePlaywrightStdioProxy(ctx context.Context, opts playwrightProxyOptions) int {
	safeStderr := &lockedWriter{w: opts.Stderr}
	if len(opts.Command) == 0 {
		fmt.Fprintln(safeStderr, "Playwright MCP command is empty")
		return 1
	}
	grace := opts.Grace
	if grace <= 0 {
		grace = 2 * time.Second
	}

	childStdoutR, childStdoutW, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(safeStderr, "Playwright MCP pipe error: %v\n", err)
		return 1
	}
	cmd := exec.Command(opts.Command[0], opts.Command[1:]...)
	cmd.Dir = opts.Dir
	cmd.Stdout = childStdoutW
	cmd.Stderr = safeStderr
	process.ConfigureProcessGroup(cmd)

	childStdin, err := cmd.StdinPipe()
	if err != nil {
		_ = childStdoutR.Close()
		_ = childStdoutW.Close()
		fmt.Fprintf(safeStderr, "Playwright MCP stdin pipe error: %v\n", err)
		return 1
	}
	if err := cmd.Start(); err != nil {
		_ = childStdin.Close()
		_ = childStdoutR.Close()
		_ = childStdoutW.Close()
		fmt.Fprintf(safeStderr, "start Playwright MCP: %v\n", err)
		return 127
	}
	_ = childStdoutW.Close()
	defer childStdoutR.Close()

	reqCh := make(chan stdioLine, 16)
	respCh := make(chan stdioLine, 16)
	waitCh := make(chan error, 1)
	go readLines(opts.Stdin, reqCh)
	go readLines(childStdoutR, respCh)
	go func() { waitCh <- cmd.Wait() }()

	state := &playwrightProxyState{
		opts:          opts,
		grace:         grace,
		cmd:           cmd,
		childStdin:    childStdin,
		pending:       map[string]struct{}{},
		initializeIDs: map[string]struct{}{},
		toolIDs:       map[string]struct{}{},
	}
	defer state.stopAllTimers()
	state.armStartup()

	var (
		waitErr error
		exited  bool
	)
	for !exited {
		select {
		case waitErr = <-waitCh:
			exited = true
		case <-ctx.Done():
			state.terminate()
		case ev, ok := <-reqCh:
			if !ok || ev.eof || ev.err != nil {
				reqCh = nil
				state.closeChildStdin()
				if !state.terminating && state.eofTimer == nil {
					state.eofCh = resetTimer(&state.eofTimer, grace)
				}
				continue
			}
			state.handleRequestLine(ev.data)
		case ev, ok := <-respCh:
			if !ok || ev.eof || ev.err != nil {
				respCh = nil
				if !state.terminating {
					state.terminate()
				}
				continue
			}
			state.handleResponseLine(ev.data)
		case <-state.startupCh:
			state.startupCh = nil
			if !state.initialized {
				state.timedOut = true
				fmt.Fprintf(safeStderr, "Playwright MCP startup timed out after %s; sent TERM with %s cleanup grace.\n", opts.StartupTimeout, grace)
				state.terminate()
			}
		case <-state.requestCh:
			state.requestCh = nil
			if len(state.pending) > 0 {
				state.timedOut = true
				fmt.Fprintf(safeStderr, "Playwright MCP request timed out after %s; sent TERM with %s cleanup grace.\n", opts.RequestTimeout, grace)
				state.terminate()
			}
		case <-state.idleCh:
			state.idleCh = nil
			if state.initialized && len(state.pending) == 0 {
				state.timedOut = true
				fmt.Fprintf(safeStderr, "Playwright MCP session idle for %s; sent TERM with %s cleanup grace.\n", opts.SessionIdleTimeout, grace)
				state.terminate()
			}
		case <-state.eofCh:
			state.eofCh = nil
			state.terminate()
		case <-state.killCh:
			state.killCh = nil
			_ = process.SignalProcessGroup(cmd.Process, true)
			state.runCleanup()
			waitErr = <-waitCh
			exited = true
		}
	}

	// Drain any buffered response lines flushed before the child exited.
	if respCh != nil {
		for ev := range respCh {
			if len(ev.data) > 0 {
				state.handleResponseLine(ev.data)
			}
		}
	}
	if state.terminating || state.timedOut {
		_ = process.SignalProcessGroup(cmd.Process, true)
		state.runCleanup()
	}

	if state.timedOut {
		return playwrightTimeoutExitStatus
	}
	if waitErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
	}
	return 1
}
