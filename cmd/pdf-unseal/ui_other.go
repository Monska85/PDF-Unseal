//go:build !windows

package main

import (
	"fmt"
	"os"

	"github.com/Monska85/PDF-Unseal/internal/i18n"
)

// run reports that no GUI exists yet for this platform.
func run() {
	fmt.Fprintln(os.Stderr, i18n.T("unsupported"))
	os.Exit(1)
}
