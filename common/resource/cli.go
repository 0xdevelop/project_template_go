package resource

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// RunCLI is a product entry point shared with the service, not a second server.
// --confirm TOKEN is explicit scoped consent; redirected stdin never prompts.
func RunCLI(ctx context.Context, c Controller, args []string, in io.Reader, out, stderr io.Writer, interactive bool) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: resource status|verify|prepare SET [--confirm TOKEN]")
		return 2
	}
	action, id := args[0], args[1]
	token := ""
	if action != "status" && action != "prepare" && action != "verify" {
		fmt.Fprintln(stderr, "unknown resource action")
		return 2
	}
	if len(args) > 2 {
		if len(args) != 4 || args[2] != "--confirm" {
			return 2
		}
		token = args[3]
	}
	status, err := c.Status(ctx, id)
	if err != nil {
		json.NewEncoder(out).Encode(status)
		fmt.Fprintln(stderr, "resource unavailable: inspect configured manifest/integrity; no download attempted")
		return 1
	}
	if action == "verify" {
		if json.NewEncoder(out).Encode(status) != nil {
			return 1
		}
		if status.State != "ready" {
			return 3
		}
		return 0
	}
	if action == "status" || status.State == "ready" {
		if err = json.NewEncoder(out).Encode(status); err != nil {
			return 1
		}
		return 0
	}
	if token == "" && interactive {
		fmt.Fprintf(stderr, "Resource set %s is missing. Download this exact configured set? [yes/no] ", id)
		line, e := readConsent(ctx, in)
		if e != nil && e != io.EOF {
			if ctx.Err() != nil {
				return 130
			}
			return 1
		}
		if strings.TrimSpace(line) == "yes" {
			token = status.Token
		}
	}
	if token == "" {
		json.NewEncoder(out).Encode(status)
		return 3
	}
	// Observable bytes are streamed on stderr, never inference task progress.
	original := c.Sets[id]
	cloned := *original
	cloned.Download.Progress = func(p Progress) { fmt.Fprintf(stderr, "download %d/%d bytes %s\n", p.Bytes, p.Total, p.Phase) }
	sets := make(map[string]*Config, len(c.Sets))
	for k, v := range c.Sets {
		sets[k] = v
	}
	sets[id] = &cloned
	c.Sets = sets
	status, err = c.Prepare(ctx, id, token)
	if e := json.NewEncoder(out).Encode(status); e != nil {
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "resource preparation did not complete; no inference started")
		return 1
	}
	return 0
}

func readConsent(ctx context.Context, in io.Reader) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	type response struct {
		line string
		err  error
	}
	done := make(chan response, 1)
	go func() { line, err := bufio.NewReader(in).ReadString('\n'); done <- response{line, err} }()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-done:
		return r.line, r.err
	}
}
