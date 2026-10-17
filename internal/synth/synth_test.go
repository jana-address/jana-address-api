package synth

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestDeterministic(t *testing.T) {
	a, b := Generate(500, 7), Generate(500, 7)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed gave different output")
	}
	c := Generate(500, 8)
	same := 0
	for i := range a {
		if a[i].Raw == c[i].Raw {
			same++
		}
	}
	if same > 25 {
		t.Fatalf("different seeds share %d of 500 raw strings", same)
	}
}

func TestLabels(t *testing.T) {
	recs := Generate(10000, 1)
	if len(recs) != 10000 {
		t.Fatalf("got %d records", len(recs))
	}
	ids := map[string]bool{}
	tags := map[string]int{}
	cityCount := map[string]int{}
	for i, g := range recs {
		fail := func(format string, args ...any) {
			t.Helper()
			t.Errorf("%s %q: %s", g.ID, g.Raw, fmt.Sprintf(format, args...))
		}
		if want := fmt.Sprintf("syn_%05d", i+1); g.ID != want {
			fail("id, want %s", want)
		}
		if ids[g.ID] {
			fail("duplicate id")
		}
		ids[g.ID] = true
		if len(g.Expected) != 11 {
			fail("%d keys", len(g.Expected))
		}
		val := func(f string) string {
			if v := g.Expected[f]; v != nil {
				return *v
			}
			return ""
		}
		for _, f := range Fields {
			if _, ok := g.Expected[f]; !ok {
				fail("missing key %s", f)
			}
			if v := g.Expected[f]; v != nil && strings.TrimSpace(*v) == "" {
				fail("empty %s", f)
			}
		}
		raw := strings.ToLower(g.Raw)
		for _, f := range []string{"street_name", "landmark", "living_area"} {
			if v := val(f); v != "" && !strings.Contains(raw, strings.ToLower(v)) {
				fail("%s %q not in raw", f, v)
			}
		}
		if h := val("house"); h == "" {
			fail("no house")
		} else if !strings.Contains(strings.ReplaceAll(g.Raw, " ", ""), strings.ReplaceAll(h, " ", "")) {
			fail("house %q not in raw", h)
		}
		for _, f := range []string{"apartment", "entrance", "floor"} {
			if v := val(f); v != "" && !isNumber(v) {
				fail("%s %q is not digits", f, v)
			}
		}
		if v := val("street_type"); v != "" && !slices.Contains(StreetTypes, v) {
			fail("street_type %q", v)
		}
		if val("street_name") == "" {
			fail("no street_name")
		}
		for _, f := range g.InString {
			if g.Expected[f] == nil {
				fail("in_string %s is null", f)
			}
		}
		inStr := func(f string) bool { return slices.Contains(g.InString, f) }
		for _, f := range []string{"district", "living_area", "street_name", "house", "city"} {
			if val(f) != "" && !inStr(f) {
				fail("%s set but not in_string", f)
			}
		}
		city, prov := val("city"), val("province")
		if city != "" && prov == "" {
			fail("city without province")
		}
		if city != "" && city == prov && !inStr("province") {
			fail("city-level province not in_string")
		}
		if city == "" && val("district") != "" {
			fail("district without city")
		}
		for _, tg := range g.Tags {
			tags[tg]++
		}
		if slices.Contains(g.Tags, "office") && val("apartment") != "" {
			fail("office with apartment")
		}
		if slices.Contains(g.Tags, "no-city") != (city == "") {
			fail("no-city tag mismatch")
		}
		if city != "" {
			cityCount[city]++
		}
	}
	for _, tg := range []string{"fast", "careful", "typo", "no-commas", "lowercase", "glued", "camel", "latin",
		"kazakh", "bilingual", "landmark", "mkr", "zhk", "living-area", "district", "oblast", "no-city",
		"dup-city", "old-name", "fraction", "letter", "postcode", "country", "extra-spaces", "abbreviations",
		"reversed-order", "block", "village"} {
		if tags[tg] < 200 {
			t.Errorf("tag %s in %d of 10000 records, want >= 2%%", tg, tags[tg])
		}
	}
	if tags["office"] == 0 {
		t.Error("no office records")
	}
	if len(cityCount) < 25 {
		t.Errorf("%d distinct cities", len(cityCount))
	}
}

func TestHelpers(t *testing.T) {
	b := &builder{r: rand.New(rand.NewPCG(1, 2)), tags: map[string]bool{}}
	for range 200 {
		got, ok := b.typo("Сейфуллина")
		if !ok || got == "Сейфуллина" {
			t.Fatalf("typo did nothing: %q", got)
		}
		if d := len([]rune(got)) - len([]rune("Сейфуллина")); d < -1 || d > 1 {
			t.Fatalf("typo changed length by %d: %q", d, got)
		}
	}
	if got := translit("Аль-Фараби"); got != "Al-Farabi" {
		t.Errorf("translit = %q", got)
	}
	if got := translit("Мәңгілік Ел"); got != "Mangilik El" {
		t.Errorf("translit = %q", got)
	}
	if got := deKaz("Қабанбай батыр көшесі"); got != "Кабанбай батыр кошеси" {
		t.Errorf("deKaz = %q", got)
	}
}
