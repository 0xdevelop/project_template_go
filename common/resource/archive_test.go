package resource

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func tarFixture(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for name, body := range map[string]string{"member-a": "data-a", "member-b": "data-b", "../escape": "must-not-extract"} {
		if err := w.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func tarFile(name, member, url string, blob, body []byte) File {
	f := spec(name, "", body)
	sum := sha256.Sum256(blob)
	f.Archive = &TarSource{URL: url, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(blob)), Entry: member}
	return f
}
func TestTarMembersShareVerifiedPayloadAndNoTraversal(t *testing.T) {
	blob := tarFixture(t)
	var gets atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(blob)))
		if r.Method != "HEAD" {
			gets.Add(1)
			w.Write(blob)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	p := Preparer{Root: root}
	files := []File{tarFile("family/a", "member-a", server.URL, blob, []byte("data-a")), tarFile("family/b", "member-b", server.URL, blob, []byte("data-b"))}
	var wg sync.WaitGroup
	for _, f := range files {
		wg.Add(1)
		go func(f File) {
			defer wg.Done()
			if err := p.Prepare(context.Background(), f, opts()); err != nil {
				t.Error(err)
			}
		}(f)
	}
	wg.Wait()
	if gets.Load() != 1 {
		t.Fatal("archive duplicated", gets.Load())
	}
	for _, f := range files {
		if err := verifyContext(context.Background(), filepath.Join(root, f.Path), f); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
		t.Fatal("tar traversal")
	}
}
func TestTarIntegrityAndCancellationNeverPublishMember(t *testing.T) {
	for _, mode := range []string{"archive_hash", "member_hash", "cancel", "unsafe_entry"} {
		t.Run(mode, func(t *testing.T) {
			blob := tarFixture(t)
			var gets atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", fmt.Sprint(len(blob)))
				if r.Method != "HEAD" {
					gets.Add(1)
					w.Write(blob)
				}
			}))
			defer server.Close()
			f := tarFile("member", "member-a", server.URL, blob, []byte("data-a"))
			o := opts()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "archive_hash":
				f.Archive.SHA256 = strings.Repeat("0", 64)
			case "member_hash":
				f.SHA256 = strings.Repeat("0", 64)
			case "cancel":
				o.Progress = func(p Progress) {
					if p.Phase == "extracting" {
						cancel()
					}
				}
			case "unsafe_entry":
				f.Archive.Entry = "../escape"
			}
			root := t.TempDir()
			p := Preparer{Root: root}
			err := p.Prepare(ctx, f, o)
			if err == nil {
				t.Fatal("invalid member accepted")
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "member")); !os.IsNotExist(err) {
				t.Fatal("member exposed before valid ready")
			}
			if mode == "unsafe_entry" && gets.Load() != 0 {
				t.Fatal("unsafe archive requested")
			}
		})
	}
}

func TestArchiveCacheCannotRedirectImplicitWrites(t *testing.T) {
	root := t.TempDir()
	family := filepath.Join(root, "family")
	os.MkdirAll(family, 0700)
	unrelated := t.TempDir()
	if err := os.Symlink(unrelated, filepath.Join(family, ".resource-archives")); err != nil {
		t.Fatal(err)
	}
	blob := tarFixture(t)
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer server.Close()
	p := Preparer{Root: root}
	if err := p.Prepare(context.Background(), tarFile("family/a", "member-a", server.URL, blob, []byte("data-a")), opts()); err == nil {
		t.Fatal("implicit archive cache symlink followed")
	}
	entries, err := os.ReadDir(unrelated)
	if err != nil || len(entries) != 0 || hits.Load() != 0 {
		t.Fatal("unrelated directory/network touched")
	}
}

func TestTarRejectsLinkMembersAndPreservesRacingDestination(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "no_replace"} {
		t.Run(kind, func(t *testing.T) {
			var buffer bytes.Buffer
			writer := tar.NewWriter(&buffer)
			header := &tar.Header{Name: "member-a", Mode: 0600, Size: 6, Typeflag: tar.TypeReg}
			if kind != "no_replace" {
				header.Size = 0
				header.Linkname = "../unrelated"
				header.Typeflag = tar.TypeSymlink
				if kind == "hardlink" {
					header.Typeflag = tar.TypeLink
				}
			}
			if err := writer.WriteHeader(header); err != nil {
				t.Fatal(err)
			}
			if header.Size > 0 {
				if _, err := writer.Write([]byte("data-a")); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			blob := buffer.Bytes()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", fmt.Sprint(len(blob)))
				if r.Method != "HEAD" {
					w.Write(blob)
				}
			}))
			defer server.Close()
			root := t.TempDir()
			victim := filepath.Join(root, "unrelated")
			os.WriteFile(victim, []byte("preserve"), 0600)
			f := tarFile("destination", "member-a", server.URL, blob, []byte("data-a"))
			o := opts()
			if kind == "no_replace" {
				o.Progress = func(p Progress) {
					if p.Phase == "verifying" {
						if err := os.WriteFile(filepath.Join(root, "destination"), []byte("external owner"), 0600); err != nil {
							t.Error(err)
						}
					}
				}
			}
			if err := (Preparer{Root: root}).Prepare(context.Background(), f, o); err == nil {
				t.Fatal("unsafe/link/racing member accepted")
			}
			body, err := os.ReadFile(victim)
			if err != nil || string(body) != "preserve" {
				t.Fatal("unrelated target changed")
			}
			if kind == "no_replace" {
				body, err = os.ReadFile(filepath.Join(root, "destination"))
				if err != nil || string(body) != "external owner" {
					t.Fatal("racing final overwritten")
				}
			} else {
				if _, err = os.Lstat(filepath.Join(root, "destination")); !os.IsNotExist(err) {
					t.Fatal("tar link published")
				}
			}
		})
	}
}
func TestTarCancelledExtractionRestartReusesVerifiedArchive(t *testing.T) {
	blob := tarFixture(t)
	var gets atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(blob)))
		if r.Method != "HEAD" {
			gets.Add(1)
			w.Write(blob)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	f := tarFile("destination", "member-a", server.URL, blob, []byte("data-a"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o := opts()
	o.Progress = func(p Progress) {
		if p.Phase == "extracting" {
			cancel()
		}
	}
	if err := (Preparer{Root: root}).Prepare(ctx, f, o); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "destination")); !os.IsNotExist(err) {
		t.Fatal("cancelled extraction published")
	}
	if err := (Preparer{Root: root}).Prepare(context.Background(), f, opts()); err != nil {
		t.Fatal(err)
	}
	if gets.Load() != 1 {
		t.Fatal("restart downloaded verified archive again", gets.Load())
	}
}
