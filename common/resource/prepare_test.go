package resource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func spec(path, url string, body []byte) File {
	h := sha256.Sum256(body)
	return File{Path: path, URL: url, SHA256: hex.EncodeToString(h[:]), Size: int64(len(body))}
}
func opts() Options {
	return Options{DownloadEnabled: true, Confirmed: true, Parallelism: 3, ChunkSize: 1024}
}

type origin struct {
	body               []byte
	etag               string
	ignore, broken     atomic.Bool
	gets, active, peak atomic.Int64
	mu                 sync.Mutex
	ranges             []string
}

func (o *origin) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Length", fmt.Sprint(len(o.body)))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("ETag", o.etag)
	if r.Method == "HEAD" {
		return
	}
	o.gets.Add(1)
	n := o.active.Add(1)
	defer o.active.Add(-1)
	for {
		p := o.peak.Load()
		if n <= p || o.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(time.Millisecond * 2)
	start, end := 0, len(o.body)-1
	if rh := r.Header.Get("Range"); rh != "" && !o.ignore.Load() {
		o.mu.Lock()
		o.ranges = append(o.ranges, rh)
		o.mu.Unlock()
		if _, e := fmt.Sscanf(rh, "bytes=%d-%d", &start, &end); e != nil || start < 0 || end >= len(o.body) || start > end {
			w.WriteHeader(416)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(o.body)))
	}
	if o.broken.Swap(false) {
		conn, _, e := w.(http.Hijacker).Hijack()
		if e != nil {
			return
		}
		defer conn.Close()
		// Deliberately truncate a valid 206 body after 100 bytes.
		fmt.Fprintf(conn, "HTTP/1.1 206 Partial Content\r\nContent-Length: %d\r\nContent-Range: bytes %d-%d/%d\r\nETag: %s\r\n\r\n", end-start+1, start, end, len(o.body), o.etag)
		conn.Write(o.body[start:min(start+100, end+1)])
		return
	}
	if r.Header.Get("Range") != "" && !o.ignore.Load() {
		w.WriteHeader(206)
	}
	w.Write(o.body[start : end+1])
}
func TestSharedPrepareAndGate(t *testing.T) {
	root := t.TempDir()
	o := &origin{body: bytes.Repeat([]byte("abcdef"), 2048), etag: `"v1"`}
	srv := httptest.NewServer(http.HandlerFunc(o.serve))
	defer srv.Close()
	p := Preparer{Root: root}
	f := spec("weights/data", srv.URL, o.body)
	var starts atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- p.WithReady(context.Background(), []File{f}, opts(), func(context.Context) error {
				if e := verifyContext(context.Background(), filepath.Join(root, f.Path), f); e != nil {
					return e
				}
				starts.Add(1)
				return nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if starts.Load() != 2 {
		t.Fatal(starts.Load())
	}
	if o.gets.Load() != 12 {
		t.Fatalf("same file downloaded twice: %d", o.gets.Load())
	}
	if o.peak.Load() > 3 || o.peak.Load() < 2 {
		t.Fatalf("parallel bound %d", o.peak.Load())
	}
}
func TestConfirmationAndIntegrityGate(t *testing.T) {
	root := t.TempDir()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	p := Preparer{Root: root}
	f := spec("a", srv.URL, []byte("abc"))
	o := opts()
	o.Confirmed = false
	called := false
	e := p.WithReady(context.Background(), []File{f}, o, func(context.Context) error { called = true; return nil })
	if !errors.Is(e, ErrConfirmationRequired) || called || hits.Load() != 0 {
		t.Fatalf("confirmation gate: %v %v %d", e, called, hits.Load())
	}
	f.SHA256 = ""
	if e = p.Prepare(context.Background(), f, opts()); !errors.Is(e, ErrIntegrity) {
		t.Fatal(e)
	}
	if hits.Load() != 0 {
		t.Fatal("unknown digest accessed network")
	}
}
func TestInterruptedResumeAndChangedEntity(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(fmt.Sprint(change), func(t *testing.T) {
			root := t.TempDir()
			o := &origin{body: bytes.Repeat([]byte("a"), 4096), etag: `"v1"`}
			o.broken.Store(true)
			srv := httptest.NewServer(http.HandlerFunc(o.serve))
			defer srv.Close()
			p := Preparer{Root: root}
			f := spec("weights", srv.URL, o.body)
			op := opts()
			op.Parallelism = 1
			if e := p.Prepare(context.Background(), f, op); e == nil {
				t.Fatal("expected interruption")
			}
			if _, e := os.Stat(filepath.Join(root, f.Path)); !os.IsNotExist(e) {
				t.Fatal("partial published")
			}
			if change {
				o.body = bytes.Repeat([]byte("b"), 4096)
				o.etag = `"v2"`
				f = spec("weights", srv.URL, o.body)
			}
			if e := p.Prepare(context.Background(), f, op); e != nil {
				t.Fatal(e)
			}
			o.mu.Lock()
			ranges := append([]string(nil), o.ranges...)
			o.mu.Unlock()
			expected := "bytes=100-1023"
			if change {
				expected = "bytes=0-1023"
			}
			if len(ranges) < 2 || ranges[1] != expected {
				t.Fatalf("resume ranges: %v want %s", ranges, expected)
			}
			if e := verifyContext(context.Background(), filepath.Join(root, f.Path), f); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestRangeIgnoredAndChecksumFailure(t *testing.T) {
	for _, badHash := range []bool{false, true} {
		t.Run(fmt.Sprint(badHash), func(t *testing.T) {
			root := t.TempDir()
			o := &origin{body: bytes.Repeat([]byte("foo"), 2000), etag: `"v1"`}
			o.ignore.Store(true)
			srv := httptest.NewServer(http.HandlerFunc(o.serve))
			defer srv.Close()
			p := Preparer{Root: root}
			f := spec("a", srv.URL, o.body)
			if badHash {
				f.SHA256 = strings.Repeat("0", 64)
			}
			e := p.Prepare(context.Background(), f, opts())
			if badHash {
				if !errors.Is(e, ErrIntegrity) {
					t.Fatal(e)
				}
				if _, e = os.Stat(filepath.Join(root, "a")); !os.IsNotExist(e) {
					t.Fatal("corruption published")
				}
			} else if e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestConfiguredSymlinkReuseAndRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	body := []byte("existing verified bytes")
	os.WriteFile(filepath.Join(outside, "a"), body, 0600)
	if e := os.Symlink(outside, filepath.Join(root, "data")); e != nil {
		t.Fatal(e)
	}
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	p := Preparer{Root: root}
	if e := p.Prepare(context.Background(), spec("data/a", srv.URL, body), Options{}); e != nil {
		t.Fatal(e)
	}
	if hits.Load() != 0 {
		t.Fatal("ready symlink accessed network")
	}
	if _, e := Resolve("relative", "data/a"); e == nil {
		t.Fatal("relative root allowed")
	}
	// CWD-independent resolution without changing process-wide cwd during tests.
	for _, path := range []string{"data/a", filepath.Join(root, "data/a")} {
		resolved, e := Resolve(root, path)
		if e != nil || resolved != filepath.Join(root, "data/a") {
			t.Fatalf("%s %v", resolved, e)
		}
	}
}
func TestCancelledDoesNotStartConsumer(t *testing.T) {
	root := t.TempDir()
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Header().Set("ETag", `"v1"`)
		if r.Method == "HEAD" {
			return
		}
		close(started)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	p := Preparer{Root: root}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var called atomic.Bool
	go func() {
		done <- p.WithReady(ctx, []File{spec("a", srv.URL, bytes.Repeat([]byte("a"), 1000))}, opts(), func(context.Context) error { called.Store(true); return nil })
	}()
	<-started
	cancel()
	if e := <-done; e == nil {
		t.Fatal("cancel succeeded")
	}
	if called.Load() {
		t.Fatal("CLI started while resource cancelled")
	}
}
func TestProcessLock(t *testing.T) {
	if os.Getenv("RESOURCE_LOCK_CHILD") == "1" {
		p := Preparer{Root: os.Getenv("RESOURCE_TEST_ROOT")}
		f := spec("a", os.Getenv("RESOURCE_TEST_URL"), bytes.Repeat([]byte("abc"), 1024))
		if e := p.Prepare(context.Background(), f, opts()); e != nil {
			t.Fatal(e)
		}
		return
	}
	root := t.TempDir()
	o := &origin{body: bytes.Repeat([]byte("abc"), 1024), etag: `"v1"`}
	srv := httptest.NewServer(http.HandlerFunc(o.serve))
	defer srv.Close()
	var cmds []*exec.Cmd
	for range 2 {
		cmd := exec.Command(os.Args[0], "-test.run=^TestProcessLock$")
		cmd.Env = append(os.Environ(), "RESOURCE_LOCK_CHILD=1", "RESOURCE_TEST_ROOT="+root, "RESOURCE_TEST_URL="+srv.URL)
		cmd.Stdout = io.Discard
		cmd.Stderr = os.Stderr
		if e := cmd.Start(); e != nil {
			t.Fatal(e)
		}
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if e := cmd.Wait(); e != nil {
			t.Fatal(e)
		}
	}
	if o.gets.Load() != 3 {
		t.Fatalf("cross-process duplicate GETs: %d", o.gets.Load())
	}
}

func TestChangedRangeIdentityNeverPublishes(t *testing.T) {
	root := t.TempDir()
	body := []byte("abc")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"v1"`)
		if r.Method == "HEAD" {
			return
		}
		w.Header().Set("ETag", `"v2"`)
		w.Header().Set("Content-Range", "bytes 0-2/3")
		w.WriteHeader(206)
		w.Write(body)
	}))
	defer srv.Close()
	p := Preparer{Root: root}
	if err := p.Prepare(context.Background(), spec("file", srv.URL, body), opts()); err == nil {
		t.Fatal("changed range accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "file")); !os.IsNotExist(err) {
		t.Fatal("changed entity published")
	}
}
func TestExplicitRetryCounts(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			var hits atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(503) }))
			defer srv.Close()
			op := opts()
			op.RetryEnabled = enabled
			op.RetryCount = 2
			p := Preparer{Root: t.TempDir()}
			if err := p.Prepare(context.Background(), spec("file", srv.URL, []byte("a")), op); err == nil {
				t.Fatal("503 succeeded")
			}
			want := int64(1)
			if enabled {
				want = 3
			}
			if hits.Load() != want {
				t.Fatalf("retry requests %d want %d", hits.Load(), want)
			}
		})
	}
}
