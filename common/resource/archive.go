package resource

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func validateArchive(a *TarSource) error {
	if a == nil {
		return nil
	}
	digest, err := hex.DecodeString(a.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("%w: trusted archive sha256 required separately from member", ErrIntegrity)
	}
	entry := path.Clean(a.Entry)
	if a.Size < 0 || a.URL == "" || entry == "." || path.IsAbs(entry) || entry == ".." || strings.HasPrefix(entry, "../") || strings.Contains(a.Entry, "\\") {
		return errors.New("invalid plain-tar source descriptor")
	}
	return nil
}

// prepareTar is called under the canonical destination lock. The verified archive
// cache sits beside its destination family; two AF2 members share one payload and
// one blob lock even when installer and daemon executables live in different dirs.
func (p Preparer) prepareTar(ctx context.Context, dest string, f File, o Options) error {
	a := f.Archive
	if err := validateArchive(a); err != nil {
		return err
	}
	cache := filepath.Join(filepath.Dir(dest), ".resource-archives")
	if info, e := os.Lstat(cache); e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return errors.New("unsafe archive cache directory")
	} else if e != nil && !os.IsNotExist(e) {
		return e
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		return err
	}
	blob := filepath.Join(cache, strings.ToLower(a.SHA256)+".tar")
	var transferred, total int64
	report := func(phase string) {
		if o.Progress != nil {
			o.Progress(Progress{transferred, total, phase})
		}
	}
	blobOptions := o
	blobOptions.Progress = func(progress Progress) {
		transferred, total = progress.Bytes, progress.Total
		report("archive_" + progress.Phase)
	}
	if err := p.Prepare(ctx, File{Path: blob, URL: a.URL, SHA256: a.SHA256, Size: a.Size}, blobOptions); err != nil {
		return err
	}
	archive, err := os.Open(blob)
	if err != nil {
		return err
	}
	defer archive.Close()
	st, err := archive.Stat()
	if err != nil {
		return err
	}
	if total == 0 {
		total = st.Size()
	}
	report("extracting")
	tr := tar.NewReader(contextReader{ctx, archive})
	var header *tar.Header
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return fmt.Errorf("%w: tar member absent", ErrIntegrity)
		}
		if e != nil {
			return e
		}
		if path.Clean(h.Name) != path.Clean(a.Entry) {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return fmt.Errorf("%w: tar member must be regular", ErrIntegrity)
		}
		if h.Size < 0 || h.Size > st.Size() || (f.Size > 0 && h.Size != f.Size) {
			return fmt.Errorf("%w: tar member length", ErrIntegrity)
		}
		header = h
		break
	}
	state := dest + ".prepare"
	if info, e := os.Lstat(state); e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return errors.New("unsafe resource staging directory")
	}
	if err = os.MkdirAll(state, 0700); err != nil {
		return err
	}
	if err = clearParts(state); err != nil {
		return err
	}
	complete := filepath.Join(state, "complete")
	out, err := os.OpenFile(complete, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(complete)
	copied, copyErr := io.CopyN(out, contextReader{ctx, tr}, header.Size)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if copied != header.Size {
		return io.ErrUnexpectedEOF
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	report("verifying")
	f.Size = header.Size
	if err = verifyContext(ctx, complete, f); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// No-replace publication cannot overwrite configured existing files/symlinks.
	if err = os.Link(complete, dest); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(dest))
	if err != nil {
		return err
	}
	err = directory.Sync()
	directory.Close()
	if err != nil {
		return err
	}
	report("ready")
	return nil
}
