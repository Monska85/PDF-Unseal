package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Monska85/PDF-Unseal/internal/i18n"
	"github.com/Monska85/PDF-Unseal/internal/testpdf"
	"github.com/Monska85/PDF-Unseal/internal/unseal"
)

// fakeUI records what process shows and answers prompts from queued values.
type fakeUI struct {
	passwords []string // answers to AskPassword; running out means cancel
	savePath  string   // answer to AskSavePath; empty means cancel
	prompts   []string
	suggested string
	errors    []string
	saved     []string
	confirms  []string
	overwrite bool // answer to ConfirmOverwrite
}

func (f *fakeUI) ShowError(m string) { f.errors = append(f.errors, m) }
func (f *fakeUI) ShowSaved(p string) { f.saved = append(f.saved, p) }

func (f *fakeUI) AskPassword(_, message string) (string, bool) {
	f.prompts = append(f.prompts, message)
	if len(f.passwords) == 0 {
		return "", false
	}
	p := f.passwords[0]
	f.passwords = f.passwords[1:]
	return p, true
}

func (f *fakeUI) ConfirmOverwrite(p string) bool {
	f.confirms = append(f.confirms, p)
	return f.overwrite
}

func (f *fakeUI) AskSavePath(suggested string) (string, bool, error) {
	f.suggested = suggested
	return f.savePath, f.savePath != "", nil
}

func TestProcess(t *testing.T) {
	fx := testpdf.Fixtures(t)

	tests := []struct {
		name        string
		file        string
		passwords   []string
		save        bool
		wantPrompts int
		wantSaved   bool
		wantError   string
	}{
		{name: "not encrypted", file: "plain", save: true, wantError: i18n.T("error.alreadyUnsealed")},
		{name: "owner only skips password", file: "owner", save: true, wantSaved: true},
		{name: "user password", file: "aes256", passwords: []string{"user"}, save: true, wantPrompts: 1, wantSaved: true},
		{name: "wrong then right", file: "aes128", passwords: []string{"nope", "owner"}, save: true, wantPrompts: 2, wantSaved: true},
		{name: "password cancelled", file: "rc4", wantPrompts: 1},
		{name: "save cancelled", file: "aes256", passwords: []string{"user"}, wantPrompts: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &fakeUI{passwords: tt.passwords}
			out := filepath.Join(t.TempDir(), "out") // no extension: process must add .pdf
			if tt.save {
				u.savePath = out
			}

			process(u, fx[tt.file])

			if len(u.prompts) != tt.wantPrompts {
				t.Errorf("prompts = %q, want %d", u.prompts, tt.wantPrompts)
			}
			if tt.wantError != "" {
				if len(u.errors) != 1 || u.errors[0] != tt.wantError {
					t.Errorf("errors = %q, want %q", u.errors, tt.wantError)
				}
			} else if len(u.errors) != 0 {
				t.Errorf("unexpected errors: %q", u.errors)
			}
			if !tt.wantSaved {
				if len(u.saved) != 0 {
					t.Errorf("unexpected saved: %q", u.saved)
				}
				return
			}
			if u.suggested != unseal.DefaultOutput(fx[tt.file]) {
				t.Errorf("suggested = %q", u.suggested)
			}
			if len(u.saved) != 1 || u.saved[0] != out+".pdf" {
				t.Errorf("saved = %q, want %q", u.saved, out+".pdf")
			}
			if s, err := unseal.Inspect(out + ".pdf"); err != nil || s != unseal.NotEncrypted {
				t.Errorf("output status = %v, %v", s, err)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{fmt.Errorf("%w: boom", unseal.ErrRead), i18n.T("error.read")},
		{fmt.Errorf("%w: boom", unseal.ErrNotPDF), i18n.T("error.notPDF")},
		{fmt.Errorf("%w: boom", unseal.ErrWrite), i18n.T("error.write")},
		{errors.New("boom"), i18n.T("error.unexpected", "boom")},
	} {
		if got := errorMessage(tc.err); got != tc.want {
			t.Errorf("errorMessage(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestProcessConfirmsOverwriteAfterAddingExtension(t *testing.T) {
	fx := testpdf.Fixtures(t)
	dir := t.TempDir()
	existing := filepath.Join(dir, "out.pdf")

	for _, overwrite := range []bool{false, true} {
		if err := os.WriteFile(existing, []byte("previous"), 0o600); err != nil {
			t.Fatal(err)
		}
		u := &fakeUI{savePath: filepath.Join(dir, "out"), overwrite: overwrite}
		process(u, fx["owner"])

		if len(u.confirms) != 1 || u.confirms[0] != existing {
			t.Errorf("overwrite=%v: confirms = %q", overwrite, u.confirms)
		}
		b, _ := os.ReadFile(existing)
		if replaced := string(b) != "previous"; replaced != overwrite {
			t.Errorf("overwrite=%v: file replaced = %v", overwrite, replaced)
		}
	}
}
