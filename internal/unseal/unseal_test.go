package unseal

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Monska85/PDF-Unseal/internal/testpdf"
)

func TestInspect(t *testing.T) {
	f := testpdf.Fixtures(t)
	want := map[string]Status{"plain": NotEncrypted, "rc4": NeedsPassword, "aes128": NeedsPassword, "aes256": NeedsPassword, "owner": OwnerOnly}
	for name, w := range want {
		got, err := Inspect(f[name])
		if err != nil || got != w {
			t.Errorf("%s: got %v, %v; want %v", name, got, err, w)
		}
	}
}

func TestUnseal(t *testing.T) {
	f := testpdf.Fixtures(t)
	cases := []struct{ file, pw string }{
		{"rc4", "user"}, {"aes128", "user"}, {"aes256", "user"}, {"aes256", "owner"}, {"owner", ""},
	}
	for _, c := range cases {
		out := filepath.Join(t.TempDir(), "out.pdf")
		if err := Unseal(f[c.file], out, c.pw); err != nil {
			t.Fatalf("%s/%s: %v", c.file, c.pw, err)
		}
		if s, err := Inspect(out); err != nil || s != NotEncrypted {
			t.Errorf("%s/%s: output status %v, %v", c.file, c.pw, s, err)
		}
		if out, err := exec.Command("qpdf", "--show-encryption", out).Output(); err != nil || string(out) != "File is not encrypted\n" {
			t.Errorf("%s/%s: qpdf says %q, %v", c.file, c.pw, out, err)
		}
	}
}

func TestUnsealWrongPassword(t *testing.T) {
	f := testpdf.Fixtures(t)
	err := Unseal(f["aes256"], filepath.Join(t.TempDir(), "out.pdf"), "nope")
	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("got %v, want ErrWrongPassword", err)
	}
}

func TestDefaultOutput(t *testing.T) {
	if got := DefaultOutput(`C:\docs\ciao.pdf`); got != `C:\docs\ciao - Unsealed.pdf` {
		t.Fatalf("got %q", got)
	}
}

func TestCheckPassword(t *testing.T) {
	f := testpdf.Fixtures(t)
	for pw, want := range map[string]error{"user": nil, "owner": nil, "nope": ErrWrongPassword, "": ErrWrongPassword} {
		if err := CheckPassword(f["aes256"], pw); !errors.Is(err, want) {
			t.Errorf("%q: got %v, want %v", pw, err, want)
		}
	}
}

func TestErrorKinds(t *testing.T) {
	f := testpdf.Fixtures(t)
	dir := t.TempDir()
	notPDF := filepath.Join(dir, "notes.pdf")
	if err := os.WriteFile(notPDF, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Inspect(filepath.Join(dir, "missing.pdf")); !errors.Is(err, ErrRead) {
		t.Errorf("missing input: got %v, want ErrRead", err)
	}
	if _, err := Inspect(notPDF); !errors.Is(err, ErrNotPDF) {
		t.Errorf("not a PDF: got %v, want ErrNotPDF", err)
	}
	if err := Unseal(f["owner"], filepath.Join(dir, "no-such-dir", "out.pdf"), ""); !errors.Is(err, ErrWrite) {
		t.Errorf("unwritable output: got %v, want ErrWrite", err)
	}
}

func TestUnsealFailureKeepsExistingOutput(t *testing.T) {
	f := testpdf.Fixtures(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "out.pdf")
	if err := os.WriteFile(out, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Unseal(f["aes256"], out, "nope"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("got %v, want ErrWrongPassword", err)
	}
	if b, _ := os.ReadFile(out); string(b) != "previous" {
		t.Errorf("existing output changed to %q", b)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
}

func TestUnsealKeepsSourcePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions")
	}
	f := testpdf.Fixtures(t)
	if err := os.Chmod(f["owner"], 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.pdf")
	if err := Unseal(f["owner"], out, ""); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("output mode = %v, want 0600", got)
	}
}

func TestRecordingWriterKeepsFirstError(t *testing.T) {
	first := errors.New("disk full")
	w := &recordingWriter{w: failingWriter{first}}
	w.Write([]byte("a"))
	w.w = failingWriter{errors.New("later")}
	w.Write([]byte("b"))
	if w.err != first {
		t.Errorf("err = %v, want %v", w.err, first)
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }
