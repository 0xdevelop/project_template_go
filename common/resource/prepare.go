// Package resource prepares verified files before a consumer is allowed to start.
// It knows nothing about model names, task scheduling, or inference progress.
package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ErrConfirmationRequired = errors.New("resource download awaiting confirmation")
	ErrDownloadDisabled     = errors.New("resource download disabled")
	ErrIntegrity            = errors.New("resource integrity verification failed")
	errRangeIgnored         = errors.New("server ignored range")
)

// File's digest must be supplied by a trusted manifest or explicitly pinned by
// the operator. A locally computed digest is not an upstream authenticity claim.
// TarSource describes a plain tar payload and a single regular-file member.
// Both the archive and the published member need separately trusted digests.
type TarSource struct {
	URL    string `json:"url" yaml:"url" toml:"url"`
	SHA256 string `json:"sha256" yaml:"sha256" toml:"sha256"`
	Size   int64  `json:"size" yaml:"size" toml:"size"`
	Entry  string `json:"entry" yaml:"entry" toml:"entry"`
}
type File struct {
	Archive *TarSource `json:"archive,omitempty" yaml:"archive,omitempty" toml:"archive,omitempty"`
	ID      string     `json:"id,omitempty" yaml:"id,omitempty" toml:"id,omitempty"`
	Path    string     `json:"path" yaml:"path" toml:"path"`
	URL     string     `json:"url" yaml:"url" toml:"url"`
	SHA256  string     `json:"sha256" yaml:"sha256" toml:"sha256"`
	Size    int64      `json:"size" yaml:"size" toml:"size"`
}

// Options are process-local, not environment variables. RetryCount means retries
// after the first attempt; zero means no retries, never unlimited.
type Options struct {
	DownloadEnabled bool  `json:"download_enabled" yaml:"download_enabled" toml:"download_enabled"`
	Confirmed       bool  `json:"-" yaml:"-" toml:"-"`
	RetryEnabled    bool  `json:"retry_enabled" yaml:"retry_enabled" toml:"retry_enabled"`
	RetryCount      int   `json:"retry_count" yaml:"retry_count" toml:"retry_count"`
	Parallelism     int   `json:"parallelism" yaml:"parallelism" toml:"parallelism"`
	ChunkSize       int64 `json:"chunk_size" yaml:"chunk_size" toml:"chunk_size"`
	// Progress reports actual written payload bytes, separate from task.progress.
	// Called serially; callbacks must not call Prepare recursively on the same file.
	Progress func(Progress) `json:"-" yaml:"-" toml:"-"`
}
type Progress struct {
	Bytes, Total int64
	Phase        string
}
type Preparer struct {
	Root   string
	Client *http.Client
}

// Resolve anchors relative resources to an explicit executable directory, never cwd.
func Resolve(root, path string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", errors.New("resource root must be absolute")
	}
	if path == "" {
		return "", errors.New("resource path is empty")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return filepath.Clean(path), nil
}

func verifyContext(ctx context.Context, path string, f File) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	in, err := openResourceFile(path)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%w: not a regular file", ErrIntegrity)
	}
	if f.Size > 0 && st.Size() != f.Size {
		return fmt.Errorf("%w: size", ErrIntegrity)
	}
	h := sha256.New()
	if _, err = io.Copy(h, contextReader{ctx, in}); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(f.SHA256) {
		return fmt.Errorf("%w: sha256", ErrIntegrity)
	}
	return nil
}

// canonicalDestination follows configured symlinks (including a symlinked data
// directory), so aliases share the same lock. Broken symlinks are rejected.
func canonicalDestination(path string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}
	if _, err := os.Lstat(path); err == nil {
		return "", errors.New("resource destination exists but cannot be resolved")
	}
	var suffix []string
	parent := filepath.Dir(path)
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Join(resolved, filepath.Base(path)), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", err
		}
		suffix = append(suffix, filepath.Base(parent))
		parent = next
	}
}

// Prepare never reads stdin. A daemon returns ErrConfirmationRequired; a CLI or
// frontend must record an explicit confirmation before calling with Confirmed.
// Consumers must not modify a ready file; resource ownership is exclusive.
func (p Preparer) Prepare(ctx context.Context, f File, o Options) error {
	digest, err := hex.DecodeString(f.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("%w: trusted sha256 required", ErrIntegrity)
	}
	if f.Size < 0 || o.RetryCount < 0 || o.Parallelism < 0 || o.Parallelism > 16 || o.ChunkSize < 0 {
		return errors.New("invalid resource options")
	}
	path, err := Resolve(p.Root, f.Path)
	if err != nil {
		return err
	}
	path, err = canonicalDestination(path)
	if err != nil {
		return err
	}
	// Reuse is read-only and needs neither network nor permission to write the parent.
	if err = verifyContext(ctx, path, f); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if !o.DownloadEnabled {
		return ErrDownloadDisabled
	}
	if !o.Confirmed {
		return ErrConfirmationRequired
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	unlock, err := lock(ctx, path+".prepare.lock")
	if err != nil {
		return err
	}
	defer unlock()
	if err = verifyContext(ctx, path, f); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if f.Archive != nil {
		return p.prepareTar(ctx, path, f, o)
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	tries := 1
	if o.RetryEnabled {
		tries += o.RetryCount
	}
	for attempt := 0; attempt < tries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = download(ctx, client, path, f, o)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrIntegrity) {
			return err
		}
	}
	return err
}

// WithReady is the only ordering gate: no consumer is called on pending,
// cancelled, unknown-integrity, or failed resources. Callers provide the complete
// domain resource set; an incomplete manifest cannot prevent upstream downloads.
func (p Preparer) WithReady(ctx context.Context, files []File, o Options, start func(context.Context) error) error {
	if len(files) == 0 {
		return errors.New("resource manifest is empty")
	}
	for _, f := range files {
		if err := p.Prepare(ctx, f, o); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return start(ctx)
}

type metadata struct {
	URL, SHA256, ETag string
	Size, Chunk       int64
}

func download(ctx context.Context, c *http.Client, path string, f File, o Options) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, f.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "identity")
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusMethodNotAllowed && res.StatusCode != http.StatusNotImplemented {
		return fmt.Errorf("resource HEAD status %d", res.StatusCode)
	}
	size := res.ContentLength
	fallback := res.StatusCode != http.StatusOK || size <= 0
	if fallback {
		// A trusted manifest length bounds GET even with chunked/HEAD-less sources.
		if f.Size <= 0 {
			return errors.New("trusted resource size required for HEAD-less/unknown-length download")
		}
		size = f.Size
	}
	if f.Size > 0 && size != f.Size {
		return fmt.Errorf("%w: remote size", ErrIntegrity)
	}
	etag := res.Header.Get("ETag")
	strong := etag != "" && !strings.HasPrefix(etag, "W/")
	chunk := o.ChunkSize
	if chunk == 0 {
		chunk = 8 << 20
	}
	parallel := o.Parallelism
	if parallel == 0 {
		parallel = 1
	}
	ranged := !fallback && strong && res.Header.Get("Accept-Ranges") == "bytes"
	if !ranged {
		chunk = size
		parallel = 1
		etag = ""
	}
	state := path + ".prepare"
	if st, e := os.Lstat(state); e == nil && (!st.IsDir() || st.Mode()&os.ModeSymlink != 0) {
		return errors.New("unsafe resource staging directory")
	}
	if err = os.MkdirAll(state, 0700); err != nil {
		return err
	}
	meta := metadata{f.URL, strings.ToLower(f.SHA256), etag, size, chunk}
	data, _ := json.Marshal(meta)
	var old []byte
	metaIn, readErr := openStateFile(filepath.Join(state, "manifest.json"), os.O_RDONLY)
	if readErr == nil {
		old, readErr = io.ReadAll(metaIn)
		metaIn.Close()
	}
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	// Only this component's staging files are discarded. Never delete final files or links.
	if string(old) != string(data) || !ranged {
		if err = clearParts(state); err != nil {
			return err
		}
	}
	metaOut, err := openStateFile(filepath.Join(state, "manifest.json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}
	_, err = metaOut.Write(data)
	closeMetaErr := metaOut.Close()
	if err != nil {
		return err
	}
	if closeMetaErr != nil {
		return closeMetaErr
	}
	count := int((size + chunk - 1) / chunk)
	var done atomic.Int64
	var mu sync.Mutex
	report := func(n int64, phase string) {
		mu.Lock()
		defer mu.Unlock()
		done.Add(n)
		if o.Progress != nil {
			o.Progress(Progress{done.Load(), size, phase})
		}
	}
	// Include already persisted bytes from a prior attempt; truncate oversize chunks.
	for i := 0; i < count; i++ {
		part, e := openStateFile(partPath(state, i), os.O_RDONLY)
		var st os.FileInfo
		if e == nil {
			st, e = part.Stat()
			part.Close()
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil {
			want := min(chunk, size-int64(i)*chunk)
			if st.Size() > want {
				if err = os.Remove(partPath(state, i)); err != nil {
					return err
				}
			} else {
				done.Add(st.Size())
			}
		}
	}
	report(0, "downloading")
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int)
	errs := make(chan error, parallel)
	var wg sync.WaitGroup
	for w := 0; w < parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				start := int64(i) * chunk
				end := min(size, start+chunk) - 1
				if e := transfer(workCtx, c, f.URL, etag, partPath(state, i), start, end, size, ranged, func(n int64) { report(n, "downloading") }); e != nil {
					errs <- e
					cancel()
					return
				}
			}
		}()
	}
feed:
	for i := 0; i < count; i++ {
		select {
		case jobs <- i:
		case <-workCtx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)
	var first error
	ignored := false
	for e := range errs {
		if first == nil {
			first = e
		}
		if errors.Is(e, errRangeIgnored) {
			ignored = true
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if ignored {
		_ = os.Remove(filepath.Join(state, "manifest.json"))
		if err = clearParts(state); err != nil {
			return err
		}
		done.Store(0)
		// Safe single-stream downgrade. Never append a 200 response to a range part.
		if err = transfer(ctx, c, f.URL, "", partPath(state, 0), 0, size-1, size, false, func(n int64) { report(n, "downloading") }); err != nil {
			return err
		}
		count = 1
	} else if first != nil {
		return first
	}
	report(0, "verifying")
	outPath := filepath.Join(state, "complete")
	out, err := openStateFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		in, e := openStateFile(partPath(state, i), os.O_RDONLY)
		if e != nil {
			out.Close()
			return e
		}
		_, e = io.Copy(out, contextReader{ctx, in})
		in.Close()
		if e != nil {
			out.Close()
			return e
		}
	}
	err = out.Sync()
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	f.Size = size
	if err = verifyContext(ctx, outPath, f); err != nil {
		_ = clearParts(state)
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// Cross-process writers use the same lock; never replace external files/symlinks.
	if _, err = os.Lstat(path); err == nil {
		return errors.New("resource destination appeared while preparing")
	} else if !os.IsNotExist(err) {
		return err
	}
	// Atomic no-replace publication, even if an external writer races the lock.
	if err = os.Link(outPath, path); err != nil {
		return err
	}
	if err = os.Remove(outPath); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err == nil {
		err = dir.Sync()
		dir.Close()
	}
	if err != nil {
		return err
	}
	report(0, "ready")
	return clearParts(state)
}
func partPath(dir string, i int) string { return filepath.Join(dir, fmt.Sprintf("part-%06d", i)) }
func clearParts(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "part-") || e.Name() == "complete" {
			if e.IsDir() || e.Type()&os.ModeSymlink != 0 {
				return errors.New("unsafe staging entry")
			}
			if err = os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
func transfer(ctx context.Context, c *http.Client, url, etag, path string, start, end, total int64, ranged bool, progress func(int64)) error {
	offset := int64(0)
	if ranged {
		if st, e := os.Stat(path); e == nil {
			offset = st.Size()
		}
	}
	if offset == end-start+1 {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "identity")
	if ranged {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start+offset, end))
		req.Header.Set("If-Range", etag)
	}
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if ranged {
		if res.StatusCode == http.StatusOK {
			return errRangeIgnored
		}
		if res.StatusCode != http.StatusPartialContent {
			return fmt.Errorf("resource range status %d", res.StatusCode)
		}
		if res.Header.Get("Content-Range") != fmt.Sprintf("bytes %d-%d/%d", start+offset, end, total) || res.Header.Get("ETag") != etag {
			return errors.New("resource range identity changed")
		}
	} else if res.StatusCode != http.StatusOK {
		return fmt.Errorf("resource status %d", res.StatusCode)
	}
	expected := end - start + 1 - offset
	if res.ContentLength >= 0 && res.ContentLength != expected {
		return errors.New("resource response length changed")
	}
	flags := os.O_WRONLY | os.O_CREATE
	if ranged {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	out, err := openStateFile(path, flags)
	if err != nil {
		return err
	}
	// Reading one extra byte detects bodies that exceed their declared range.
	buf := make([]byte, 64<<10)
	var written int64
	reader := io.LimitReader(res.Body, expected+1)
	for {
		n, e := reader.Read(buf)
		if n > 0 {
			if written+int64(n) > expected {
				out.Close()
				return errors.New("resource response exceeds range")
			}
			m, we := out.Write(buf[:n])
			written += int64(m)
			progress(int64(m))
			if we != nil {
				out.Close()
				return we
			}
		}
		if e != nil {
			if e != io.EOF {
				out.Close()
				return e
			}
			break
		}
	}
	err = out.Sync()
	ce := out.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	if written != expected {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// contextReader makes large local hashes/assembly cancellable between IO blocks.
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// CanonicalPath resolves configured aliases for manifest/destination agreement.
func CanonicalPath(root, path string) (string, error) {
	p, err := Resolve(root, path)
	if err != nil {
		return "", err
	}
	return canonicalDestination(p)
}
