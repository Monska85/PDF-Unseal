// Package testpdf builds protected PDF fixtures for tests with qpdf.
package testpdf

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const minimalPDF = `%PDF-1.4
1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj
2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj
3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R >> endobj
4 0 obj << /Length 24 >> stream
0 0 m 100 100 l S
endstream endobj
trailer << /Root 1 0 R /Size 5 >>
%%EOF
`

// Fixtures returns paths of test PDFs keyed by name: "plain", user+owner password
// ("rc4", "aes128", "aes256", user password "user", owner password "owner"), and owner password only ("owner").
// It skips the test when qpdf is missing, except in CI where it fails.
func Fixtures(t *testing.T) map[string]string {
	t.Helper()
	if _, err := exec.LookPath("qpdf"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("qpdf is required in CI")
		}
		t.Skip("qpdf not installed")
	}
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.pdf")
	if err := os.WriteFile(raw, []byte(minimalPDF), 0o600); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "plain.pdf")
	Run(t, "qpdf", "--warning-exit-0", raw, plain) // qpdf rebuilds the missing xref table
	files := map[string]string{"plain": plain}
	for name, args := range map[string][]string{
		"rc4":    {"--allow-weak-crypto", "--encrypt", "user", "owner", "128", "--use-aes=n", "--"},
		"aes128": {"--encrypt", "user", "owner", "128", "--use-aes=y", "--"},
		"aes256": {"--encrypt", "user", "owner", "256", "--"},
		"owner":  {"--encrypt", "", "owner", "256", "--print=none", "--extract=n", "--modify=none", "--"},
	} {
		out := filepath.Join(dir, name+".pdf")
		Run(t, "qpdf", append(append([]string{}, args...), plain, out)...)
		files[name] = out
	}
	return files
}

// Run runs a command and fails the test if it fails.
func Run(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
