package resource

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type unreadableInput struct{}

func (unreadableInput) Read([]byte) (int, error) { panic("non-TTY tried to read stdin") }
func TestScopedConfirmationCLIAndStaleReady(t *testing.T) {
	body := []byte("verified-resource")
	var a, b atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		if r.Method == "HEAD" {
			return
		}
		if r.URL.Path == "/a" {
			a.Add(1)
		} else {
			b.Add(1)
		}
		w.Write(body)
	}))
	defer server.Close()
	cfg := func(path, url string) *Config {
		return &Config{Enabled: true, Files: []File{spec(path, url, body)}, Download: opts()}
	}
	c := Controller{Root: t.TempDir(), Sets: map[string]*Config{"a": cfg("a", server.URL+"/a"), "b": cfg("b", server.URL+"/b")}}
	before, err := c.Status(context.Background(), "a")
	if err != nil || before.State != "awaiting_confirmation" {
		t.Fatal(before, err)
	}
	var out, stderr bytes.Buffer
	if code := RunCLI(context.Background(), c, []string{"prepare", "a"}, unreadableInput{}, &out, &stderr, false); code != 3 {
		t.Fatal(code)
	}
	if a.Load()+b.Load() != 0 || strings.Contains(stderr.String(), "yes/no") {
		t.Fatal("unconfirmed read/download/prompt")
	}
	if _, err = c.Prepare(context.Background(), "b", before.Token); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatal("confirmation crossed set", err)
	}
	// Token must bind the canonical destination, not just the URL/digest.
	changed := *c.Sets["a"]
	changed.Files = append([]File(nil), changed.Files...)
	changed.Files[0].Path = "another"
	c.Sets["a"] = &changed
	if _, err = c.Prepare(context.Background(), "a", before.Token); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatal("stale config confirmation accepted", err)
	}
	c.Sets["a"].Files[0].Path = "a"
	out.Reset()
	stderr.Reset()
	if code := RunCLI(context.Background(), c, []string{"prepare", "a", "--confirm", before.Token}, unreadableInput{}, &out, &stderr, false); code != 0 {
		t.Fatal(code, stderr.String())
	}
	if a.Load() != 1 || b.Load() != 0 {
		t.Fatal("wrong resource set downloaded")
	}
	var s Snapshot
	if err = json.Unmarshal(out.Bytes(), &s); err != nil || s.State != "ready" {
		t.Fatal(s, err)
	}
	if !strings.Contains(stderr.String(), "bytes") {
		t.Fatal("no real byte progress")
	}
	if err = os.Remove(filepath.Join(c.Root, "a")); err != nil {
		t.Fatal(err)
	}
	s, err = c.Status(context.Background(), "a")
	if err != nil || s.State == "ready" {
		t.Fatal("stale ready after deletion", s, err)
	}
	out.Reset()
	if code := RunCLI(context.Background(), c, []string{"verify", "a"}, unreadableInput{}, &out, &stderr, false); code == 0 {
		t.Fatal("verify allowed missing resource")
	}
}
func TestNonTerminalDeviceIsNotInteractive(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Fatal("/dev/null mistaken for TTY")
	}
}
func TestIncompleteFinalIsPreservedWithoutNetwork(t *testing.T) {
	body := []byte("complete")
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer server.Close()
	root := t.TempDir()
	path := filepath.Join(root, "file")
	os.WriteFile(path, []byte("bad"), 0600)
	p := Preparer{Root: root}
	if err := p.Prepare(context.Background(), spec("file", server.URL, body), opts()); !errors.Is(err, ErrIntegrity) {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "bad" || hits.Load() != 0 {
		t.Fatal("replaced existing incomplete source")
	}
}
func TestHeadlessBoundedStreaming(t *testing.T) {
	for _, mode := range []int{200, 405, 501} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			body := []byte("safe-body")
			var gets atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "HEAD" {
					w.WriteHeader(mode)
					return
				}
				gets.Add(1)
				w.(http.Flusher).Flush()
				w.Write(body)
			}))
			defer server.Close()
			p := Preparer{Root: t.TempDir()}
			if err := p.Prepare(context.Background(), spec("file", server.URL, body), opts()); err != nil {
				t.Fatal(err)
			}
			if gets.Load() != 1 {
				t.Fatal(gets.Load())
			}
		})
	}
}
func TestCancellationAwareLocalIO(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := Preparer{Root: t.TempDir()}
	f := spec("file", "http://not-used.invalid", []byte("data"))
	os.WriteFile(filepath.Join(p.Root, "file"), []byte("data"), 0600)
	if err := p.Prepare(ctx, f, opts()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStatusDoesNotHideReadyBehindInterruptedSnapshot(t *testing.T) {
	root := t.TempDir()
	body := []byte("verified")
	file := spec("file", "", body)
	c := Controller{Root: root, Sets: map[string]*Config{"set": {Enabled: true, Files: []File{file}, Download: opts()}}}
	_, token, err := c.selected("set")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.persist(Snapshot{Set: "set", Token: token, State: "downloading"}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "file"), body, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := c.Status(context.Background(), "set")
	if err != nil || s.State != "ready" {
		t.Fatal(s, err)
	}
}

func TestInteractiveConsentCancellation(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = readConsent(ctx, r); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestConfirmationBindsSetNameEvenForAliases(t *testing.T) {
	cfg := &Config{Enabled: true, Files: []File{spec("file", "http://unused.invalid", []byte("x"))}, Download: opts()}
	c := Controller{Root: t.TempDir(), Sets: map[string]*Config{"one": cfg, "alias": cfg}}
	s, err := c.Status(context.Background(), "one")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Prepare(context.Background(), "alias", s.Token); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatal("set name not scoped", err)
	}
}
func TestCorruptLaterResourceStopsEntireSetBeforeNetwork(t *testing.T) {
	var gets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { gets.Add(1) }))
	defer srv.Close()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "later"), []byte("corrupt"), 0600)
	cfg := &Config{Enabled: true, Files: []File{spec("first", srv.URL, []byte("valid")), spec("later", srv.URL, []byte("valid"))}, Download: opts()}
	c := Controller{Root: root, Sets: map[string]*Config{"set": cfg}}
	_, token, err := c.selected("set")
	if err != nil {
		t.Fatal(err)
	}
	if s, err := c.Status(context.Background(), "set"); !errors.Is(err, ErrIntegrity) || s.State != "invalid" {
		t.Fatal(s, err)
	}
	if _, err = c.Prepare(context.Background(), "set", token); !errors.Is(err, ErrIntegrity) {
		t.Fatal(err)
	}
	if gets.Load() != 0 {
		t.Fatal("download started before complete preflight")
	}
}
