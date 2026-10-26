// Package i18n translates the user interface.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Fallback is the language used for missing languages and keys.
const Fallback = "en"

// EnvVar forces the language, for example PDF_UNSEAL_LANG=it.
const EnvVar = "PDF_UNSEAL_LANG"

//go:embed locales/*.json
var files embed.FS

var (
	catalogs = mustLoad()
	current  = Fallback
)

func mustLoad() map[string]map[string]string {
	entries, err := files.ReadDir("locales")
	if err != nil {
		panic(err)
	}
	catalogs := map[string]map[string]string{}
	for _, e := range entries {
		data, err := files.ReadFile("locales/" + e.Name())
		if err != nil {
			panic(err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			panic(fmt.Sprintf("i18n: %s: %v", e.Name(), err))
		}
		catalogs[strings.TrimSuffix(e.Name(), ".json")] = messages
	}
	return catalogs
}

// Init selects the language from EnvVar, then from the system preferences, then Fallback.
func Init() {
	current = choose(append([]string{os.Getenv(EnvVar)}, systemLanguages()...))
}

// Language returns the selected language code.
func Language() string {
	return current
}

// choose returns the first supported language among tags such as "it-IT" or "it_IT.UTF-8".
func choose(tags []string) string {
	for _, tag := range tags {
		base, _, _ := strings.Cut(strings.ToLower(tag), "-")
		base, _, _ = strings.Cut(base, "_")
		base, _, _ = strings.Cut(base, ".")
		if _, ok := catalogs[base]; ok {
			return base
		}
	}
	return Fallback
}

// T returns the message for key in the selected language, formatted with args when given.
func T(key string, args ...any) string {
	msg, ok := catalogs[current][key]
	if !ok {
		if msg, ok = catalogs[Fallback][key]; !ok {
			msg = key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}
