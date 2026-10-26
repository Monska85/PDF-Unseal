package i18n

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestLocalesHaveSameKeys(t *testing.T) {
	want := slices.Sorted(maps.Keys(catalogs[Fallback]))
	for lang, messages := range catalogs {
		if got := slices.Sorted(maps.Keys(messages)); !slices.Equal(got, want) {
			t.Errorf("%s.json keys differ from %s.json:\n got %v\nwant %v", lang, Fallback, got, want)
		}
	}
}

func TestLocalesKeepPlaceholders(t *testing.T) {
	for key, msg := range catalogs[Fallback] {
		for lang, messages := range catalogs {
			if n, m := strings.Count(msg, "%s"), strings.Count(messages[key], "%s"); n != m {
				t.Errorf("%s: %q has %d %%s, %s has %d", key, lang, m, Fallback, n)
			}
		}
	}
}

func TestChoose(t *testing.T) {
	for _, tc := range []struct {
		tags []string
		want string
	}{
		{[]string{"", "it-IT"}, "it"},
		{[]string{"it_IT.UTF-8"}, "it"},
		{[]string{"IT"}, "it"},
		{[]string{"de-DE", "it-IT"}, "it"},
		{[]string{"en-US", "it-IT"}, "en"},
		{[]string{"de-DE"}, Fallback},
		{[]string{"C.UTF-8"}, Fallback},
		{nil, Fallback},
	} {
		if got := choose(tc.tags); got != tc.want {
			t.Errorf("choose(%q) = %q, want %q", tc.tags, got, tc.want)
		}
	}
}

func TestInitHonoursEnvVar(t *testing.T) {
	t.Setenv(EnvVar, "it")
	t.Setenv("LANG", "en_US.UTF-8")
	Init()
	t.Cleanup(func() { current = Fallback })
	if Language() != "it" {
		t.Fatalf("Language() = %q, want it", Language())
	}
	if got := T("password.unlock"); got != "Sblocca" {
		t.Errorf("T = %q", got)
	}
}

func TestTFallbacks(t *testing.T) {
	current = "it"
	t.Cleanup(func() { current = Fallback })
	if got := T("status.saved", "a.pdf"); got != "✓  Salvato a.pdf" {
		t.Errorf("formatted = %q", got)
	}
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("missing key = %q", got)
	}
}
