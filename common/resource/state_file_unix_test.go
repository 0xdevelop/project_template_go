//go:build unix

package resource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestResumedStagingCannotWriteThroughLinks(t *testing.T) {
	body := []byte("correct model payload")
	digest := sha256.Sum256(body)
	sha := hex.EncodeToString(digest[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"stable"`)
		http.ServeContent(w, r, "model", time.Time{}, bytes.NewReader(body))
	}))
	defer server.Close()
	for _, slot := range []string{"manifest.json", "part-000000", "complete"} {
		for _, kind := range []string{"symlink", "hardlink"} {
			t.Run(slot+"/"+kind, func(t *testing.T) {
				root := t.TempDir()
				state := filepath.Join(root, "model.prepare")
				if err := os.Mkdir(state, 0700); err != nil {
					t.Fatal(err)
				}
				meta, _ := json.Marshal(metadata{server.URL, sha, `"stable"`, int64(len(body)), 4})
				if slot != "manifest.json" {
					if err := os.WriteFile(filepath.Join(state, "manifest.json"), meta, 0600); err != nil {
						t.Fatal(err)
					}
				}
				victim := filepath.Join(root, "existing-unrelated-file")
				original := []byte("SOURCE MUST NOT CHANGE")
				if err := os.WriteFile(victim, original, 0600); err != nil {
					t.Fatal(err)
				}
				var err error
				if kind == "symlink" {
					err = os.Symlink(victim, filepath.Join(state, slot))
				} else {
					err = os.Link(victim, filepath.Join(state, slot))
				}
				if err != nil {
					t.Fatal(err)
				}
				o := opts()
				o.ChunkSize = 4
				o.Parallelism = 2
				err = (Preparer{Root: root}).Prepare(context.Background(), File{Path: "model", URL: server.URL, SHA256: sha, Size: int64(len(body))}, o)
				if err == nil {
					t.Fatal("unsafe staged alias accepted")
				}
				after, e := os.ReadFile(victim)
				if e != nil || !bytes.Equal(after, original) {
					t.Fatal("unrelated source was overwritten")
				}
				if _, e = os.Stat(filepath.Join(root, "model")); !os.IsNotExist(e) {
					t.Fatal("unsafe staging published")
				}
			})
		}
	}
}

func TestFIFOResourceAndStateRefuseWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan []error, 1)
	go func() {
		resourceErr := verifyContext(context.Background(), path, File{})
		f, stateErr := openStateFile(path, os.O_RDONLY)
		if f != nil {
			f.Close()
		}
		done <- []error{resourceErr, stateErr}
	}()
	select {
	case errs := <-done:
		if !errors.Is(errs[0], ErrIntegrity) || errs[1] == nil {
			t.Fatalf("unexpected FIFO errors: %v", errs)
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO open blocked")
	}
}
