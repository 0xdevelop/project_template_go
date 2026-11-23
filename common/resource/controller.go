package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Controller exposes only operator-configured sets, never caller supplied URLs/paths.
// Async work is owned by the existing task worker, not a goroutine in this package.
type Controller struct {
	Root string
	Sets map[string]*Config
}
type Snapshot struct {
	Set       string    `json:"set"`
	Token     string    `json:"confirmation_token,omitempty"`
	State     string    `json:"state"`
	File      int       `json:"file"`
	Bytes     int64     `json:"downloaded_bytes"`
	Total     int64     `json:"total_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

var ErrPreparationFailed = errors.New("resource preparation failed; inspect source availability and local IO permissions")
var ErrSetUnavailable = errors.New("resource set unavailable or disabled; configure an enabled trusted manifest")

func (c Controller) selected(id string) (*Config, string, error) {
	cfg := c.Sets[id]
	if cfg == nil || !cfg.Enabled || len(cfg.Files) == 0 {
		return nil, "", ErrSetUnavailable
	}
	copyCfg := *cfg
	copyCfg.Files = append([]File(nil), cfg.Files...)
	for i, f := range copyCfg.Files {
		digest, err := hex.DecodeString(f.SHA256)
		if err != nil || len(digest) != sha256.Size {
			return nil, "", fmt.Errorf("%w: trusted sha256 required", ErrIntegrity)
		}
		path, err := Resolve(c.Root, f.Path)
		if err != nil {
			return nil, "", err
		}
		path, err = canonicalDestination(path)
		if err != nil {
			return nil, "", err
		}
		copyCfg.Files[i].Path = path
	}
	raw, err := json.Marshal(struct {
		Set    string
		Config Config
	}{id, copyCfg})
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	return &copyCfg, hex.EncodeToString(sum[:]), nil
}
func (c Controller) statePath(token string) string {
	return filepath.Join(c.Root, "data", "resource_state", token+".json")
}
func (c Controller) persist(s Snapshot) error {
	path := c.statePath(s.Token)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	buf, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(buf); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Status is read-only. Progress is the last durable observation, not a claim that
// an interrupted process is still alive. task.get remains the worker lifecycle truth.
func (c Controller) Status(ctx context.Context, id string) (Snapshot, error) {
	s := Snapshot{Set: id, State: "unsupported", UpdatedAt: time.Now().UTC()}
	cfg, token, err := c.selected(id)
	if err != nil {
		return s, err
	}
	s.Token = token
	// During work return the last durable byte observation without rehashing every
	// earlier multi-GB file on each poll. This does not assert process liveness.
	if data, e := os.ReadFile(c.statePath(token)); e == nil {
		var snapshot Snapshot
		if e = json.Unmarshal(data, &snapshot); e != nil {
			return s, errors.New("invalid resource status snapshot")
		}
		if snapshot.Token != token {
			return s, errors.New("resource status identity mismatch")
		}
		switch snapshot.State {
		case "preparing", "downloading", "verifying", "archive_downloading", "archive_verifying", "archive_ready", "extracting":
			missing := false
			for _, f := range cfg.Files {
				if _, err := os.Stat(f.Path); os.IsNotExist(err) {
					missing = true
				} else if err != nil {
					return s, err
				}
			}
			if missing {
				snapshot.Set = id
				return snapshot, nil
			}
		}
	} else if !os.IsNotExist(e) {
		return s, e
	}
	all := true
	for i, f := range cfg.Files {
		if err = verifyContext(ctx, f.Path, f); err != nil {
			if !os.IsNotExist(err) {
				s.State = "invalid"
				return s, err
			}
			all = false
			s.File = i
			continue
		}
	}
	if all {
		s.State = "ready"
		return s, nil
	}
	s.State = "awaiting_confirmation"
	if !cfg.Download.DownloadEnabled {
		s.State = "download_disabled"
		return s, nil
	}
	var saved Snapshot
	if data, e := os.ReadFile(c.statePath(token)); e == nil {
		if e = json.Unmarshal(data, &saved); e != nil {
			return s, errors.New("invalid resource status snapshot")
		}
		if saved.Token != token {
			return s, errors.New("resource status identity mismatch")
		}
		saved.Set = id
		if saved.State == "ready" {
			return s, nil
		}
		return saved, nil
	} else if !os.IsNotExist(e) {
		return s, e
	}
	return s, nil
}

// Prepare only confirms this exact configured set fingerprint. A changed URL,
// digest, destination (including symlink target) or options requires reconfirmation.
func (c Controller) Prepare(ctx context.Context, id, confirmation string) (Snapshot, error) {
	cfg, token, err := c.selected(id)
	s := Snapshot{Set: id, Token: token, State: "awaiting_confirmation", UpdatedAt: time.Now().UTC()}
	if err != nil {
		return s, err
	}
	if confirmation == "" || confirmation != token {
		return s, ErrConfirmationRequired
	}
	// Preflight the whole configured set before downloading its first file.
	// A known-corrupt later file must not cause an earlier missing file to download.
	allReady := true
	for _, f := range cfg.Files {
		if e := verifyContext(ctx, f.Path, f); e != nil {
			if !os.IsNotExist(e) {
				s.State = "invalid"
				return s, e
			}
			if e := validateArchive(f.Archive); e != nil {
				return s, e
			}
			allReady = false
		}
	}
	if allReady {
		s.State = "ready"
		return s, nil
	}
	if !cfg.Download.DownloadEnabled {
		s.State = "download_disabled"
		return s, ErrDownloadDisabled
	}
	// Serialize snapshot ownership per set; file locks still coordinate aliases,
	// other sets and other executables that share a physical destination.
	if err = os.MkdirAll(filepath.Dir(c.statePath(token)), 0700); err != nil {
		return s, err
	}
	unlock, err := lock(ctx, c.statePath(token)+".lock")
	if err != nil {
		return s, err
	}
	defer unlock()
	// A preceding owner may already have completed the exact set.
	ready := true
	for _, f := range cfg.Files {
		if e := verifyContext(ctx, f.Path, f); e != nil {
			ready = false
			break
		}
	}
	if ready {
		s.State = "ready"
		s.UpdatedAt = time.Now().UTC()
		return s, c.persist(s)
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	o := cfg.Download
	o.Confirmed = true
	var writeErr error
	var last time.Time
	o.Progress = func(p Progress) {
		if writeErr != nil {
			return
		}
		now := time.Now()
		if p.Phase == s.State && now.Sub(last) < 100*time.Millisecond && (s.Bytes > 0 || p.Bytes == 0) {
			return
		}
		last = now
		s.State = p.Phase
		s.Bytes = p.Bytes
		s.Total = p.Total
		s.UpdatedAt = now.UTC()
		writeErr = c.persist(s)
		if writeErr != nil {
			cancel()
			return
		}
		if cfg.Download.Progress != nil {
			cfg.Download.Progress(p)
		}
	}
	s.State = "preparing"
	if err = c.persist(s); err != nil {
		return s, err
	}
	p := Preparer{Root: c.Root}
	for i, f := range cfg.Files {
		s.File = i
		s.Bytes = 0
		s.Total = f.Size
		if err = p.Prepare(workCtx, f, o); err != nil {
			break
		}
		if writeErr != nil {
			err = writeErr
			break
		}
	}
	if writeErr != nil {
		err = writeErr
	}
	s.UpdatedAt = time.Now().UTC()
	if err != nil {
		s.State = "failed"
		if ctx.Err() != nil {
			s.State = "cancelled"
		}
	} else {
		s.State = "ready"
	}
	if err != nil && !errors.Is(err, ErrIntegrity) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		err = ErrPreparationFailed
	}
	if e := c.persist(s); e != nil {
		return s, errors.Join(err, e)
	}
	return s, err
}
