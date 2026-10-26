// Package unseal detects PDF encryption and writes decrypted copies.
package unseal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// Status describes how a PDF is protected.
type Status int

const (
	// NotEncrypted means the PDF has no password of any kind.
	NotEncrypted Status = iota
	// NeedsPassword means the PDF has an open (user) password.
	NeedsPassword
	// OwnerOnly means the PDF opens freely but has an owner password restricting permissions.
	OwnerOnly
)

var (
	// ErrWrongPassword reports a password that does not open the PDF.
	ErrWrongPassword = pdfcpu.ErrWrongPassword
	// ErrRead reports that the input file could not be opened.
	ErrRead = errors.New("cannot read input file")
	// ErrNotPDF reports an input file that is not a readable PDF.
	ErrNotPDF = errors.New("not a valid PDF")
	// ErrWrite reports that the output file could not be written.
	ErrWrite = errors.New("cannot write output file")
)

func init() {
	// Keep pdfcpu from creating a config directory in the user's profile.
	api.DisableConfigDir()
}

func config(password string) *model.Configuration {
	conf := model.NewDefaultConfiguration()
	conf.UserPW = password
	// The entered password may be the owner password; pdfcpu falls back to the user password check.
	conf.OwnerPW = password
	return conf
}

// Inspect reports how the PDF at path is protected.
func Inspect(path string) (Status, error) {
	f, err := os.Open(path)
	if err != nil {
		return NotEncrypted, fmt.Errorf("%w: %w", ErrRead, err)
	}
	defer f.Close()

	ctx, err := api.ReadContext(context.Background(), f, config(""))
	if errors.Is(err, pdfcpu.ErrWrongPassword) {
		return NeedsPassword, nil
	}
	if err != nil {
		return NotEncrypted, fmt.Errorf("%w: %w", ErrNotPDF, err)
	}
	if ctx.Encrypt == nil {
		return NotEncrypted, nil
	}
	return OwnerOnly, nil
}

// CheckPassword returns ErrWrongPassword if password does not open the PDF at path.
func CheckPassword(path, password string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRead, err)
	}
	defer f.Close()

	_, err = api.ReadContext(context.Background(), f, config(password))
	if errors.Is(err, pdfcpu.ErrWrongPassword) {
		return ErrWrongPassword
	}
	return err
}

// Unseal writes a copy of in without any encryption or permission restrictions to out.
// password may be empty for owner-only protected files.
// The output appears only once complete; a failure leaves any existing file at out untouched.
func Unseal(in, out, password string) (err error) {
	f, err := os.Open(in)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRead, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRead, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(out), ".pdf-unseal-*")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	w := &recordingWriter{w: tmp}
	if err := api.Decrypt(context.Background(), f, w, config(password)); err != nil {
		switch {
		case errors.Is(err, pdfcpu.ErrWrongPassword):
			return ErrWrongPassword
		case w.err != nil:
			return fmt.Errorf("%w: %w", ErrWrite, w.err)
		}
		return err
	}
	// The copy is as private as the original.
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	if err := os.Rename(tmp.Name(), out); err != nil {
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	return nil
}

// recordingWriter remembers the first write error, so output failures can be told apart from PDF errors.
type recordingWriter struct {
	w   io.Writer
	err error
}

func (rw *recordingWriter) Write(p []byte) (int, error) {
	n, err := rw.w.Write(p)
	if err != nil && rw.err == nil {
		rw.err = err
	}
	return n, err
}

// DefaultOutput returns the suggested output path: "dir/name.pdf" -> "dir/name - Unsealed.pdf".
func DefaultOutput(in string) string {
	ext := filepath.Ext(in)
	return strings.TrimSuffix(in, ext) + " - Unsealed" + ext
}
