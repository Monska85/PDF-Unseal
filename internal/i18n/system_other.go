//go:build !windows

package i18n

import "os"

// systemLanguages returns the POSIX locale settings in priority order, such as "it_IT.UTF-8".
func systemLanguages() []string {
	var langs []string
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if l := os.Getenv(v); l != "" {
			langs = append(langs, l)
		}
	}
	return langs
}
