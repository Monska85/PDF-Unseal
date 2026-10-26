// Command pdf-unseal removes passwords and permission restrictions from PDF files.
package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Monska85/PDF-Unseal/internal/i18n"
	"github.com/Monska85/PDF-Unseal/internal/unseal"
)

const (
	appTitle = "PDF Unseal"
	author   = "Daniele M."
	repoURL  = "https://github.com/Monska85/PDF-Unseal"
)

// version is set by release builds.
var version = "dev"

func main() {
	i18n.Init()
	run()
}

// process unseals the PDF at path, asking the user for anything it needs.
func process(u ui, path string) {
	status, err := unseal.Inspect(path)
	if err != nil {
		u.ShowError(errorMessage(err))
		return
	}

	password := ""
	switch status {
	case unseal.NotEncrypted:
		u.ShowError(i18n.T("error.alreadyUnsealed"))
		return
	case unseal.NeedsPassword:
		var ok bool
		if password, ok = askPassword(u, path); !ok {
			return
		}
	}

	out, ok, err := u.AskSavePath(unseal.DefaultOutput(path))
	if err != nil {
		u.ShowError(errorMessage(err))
		return
	}
	if !ok {
		return
	}
	// The save dialog only confirmed overwriting the name as typed; ask again if adding .pdf hits another file.
	if !strings.EqualFold(filepath.Ext(out), ".pdf") {
		out += ".pdf"
		if _, err := os.Stat(out); err == nil && !u.ConfirmOverwrite(out) {
			return
		}
	}

	if err := unseal.Unseal(path, out, password); err != nil {
		u.ShowError(errorMessage(err))
		return
	}
	u.ShowSaved(out)
}

// askPassword prompts until the password opens the PDF or the user cancels.
func askPassword(u ui, path string) (string, bool) {
	message := i18n.T("password.prompt")
	for {
		password, ok := u.AskPassword(filepath.Base(path), message)
		if !ok {
			return "", false
		}
		err := unseal.CheckPassword(path, password)
		if err == nil {
			return password, true
		}
		if !errors.Is(err, unseal.ErrWrongPassword) {
			u.ShowError(errorMessage(err))
			return "", false
		}
		message = i18n.T("password.wrong")
	}
}

// errorMessage turns err into a translated message for the user.
func errorMessage(err error) string {
	switch {
	case errors.Is(err, unseal.ErrRead):
		return i18n.T("error.read")
	case errors.Is(err, unseal.ErrNotPDF):
		return i18n.T("error.notPDF")
	case errors.Is(err, unseal.ErrWrite):
		return i18n.T("error.write")
	default:
		return i18n.T("error.unexpected", err)
	}
}
