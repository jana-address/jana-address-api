// Package kato loads the Kazakhstan administrative-territorial classifier
// (KATO) published by the Bureau of National Statistics at stat.gov.kz.
//
// A KATO code has 9 digits ab|cd|ef|hij: region, district, rural okrug and
// settlement. Lower levels are zero for higher-level units.
package kato

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// Kind is the type of an administrative unit or settlement.
type Kind uint8

const (
	Region   Kind = iota + 1 // oblast or city of republican significance
	District                 // район, city administration (Г.А.), city district
	Okrug                    // rural okrug (с.о.), settlement or town administration
	City                     // г.
	Village                  // с., п., аул
	Other                    // station, siding, winter camp, farm and the like
)

var kindNames = [...]string{"", "region", "district", "okrug", "city", "village", "other"}

// Kinds lists all valid kinds in order.
var Kinds = []Kind{Region, District, Okrug, City, Village, Other}

func (k Kind) String() string {
	if int(k) < len(kindNames) && k != 0 {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", uint8(k))
}

// ParseKind is the inverse of Kind.String.
func ParseKind(s string) (Kind, error) {
	for _, k := range Kinds {
		if kindNames[k] == s {
			return k, nil
		}
	}
	return 0, fmt.Errorf("kato: unknown kind %q", s)
}

func (k Kind) MarshalText() ([]byte, error) {
	if k < Region || k > Other {
		return nil, fmt.Errorf("kato: invalid kind %d", uint8(k))
	}
	return []byte(k.String()), nil
}

func (k *Kind) UnmarshalText(b []byte) error {
	v, err := ParseKind(string(b))
	if err != nil {
		return err
	}
	*k = v
	return nil
}

// Record is one classifier row.
type Record struct {
	Code   string // 9 digits
	Parent string // code of the nearest existing ancestor, empty for regions
	Kind   Kind
	NameRU string // as published, whitespace (incl. NBSP) collapsed
	NameKK string
	// Names without the type prefix or suffix: "г.Семей" -> "Семей".
	NameRUClean string
	NameKKClean string
}

const codeLen = 9

func validCode(s string) bool {
	if len(s) != codeLen {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// IsTopLevel reports whether code is a region or a city of republican
// significance (cd, ef and hij are all zero).
func IsTopLevel(code string) bool { return code[2:] == "0000000" }

// ancestors returns candidate parent codes, nearest first.
func ancestors(code string) []string {
	var out []string
	for _, p := range []int{6, 4, 2} {
		if strings.Trim(code[p:], "0") != "" {
			out = append(out, code[:p]+strings.Repeat("0", codeLen-p))
		}
	}
	return out
}

// newRecord classifies a raw row; Parent is filled in by link.
func newRecord(code, ru, kk string) Record {
	ru, kk = collapse(ru), collapse(kk)
	kind, ruClean := classifyRU(code, ru)
	return Record{
		Code:        code,
		Kind:        kind,
		NameRU:      ru,
		NameKK:      kk,
		NameRUClean: ruClean,
		NameKKClean: cleanKK(kk),
	}
}

func byCode(a, b Record) int { return cmp.Compare(a.Code, b.Code) }

// link sorts recs by code and sets Parent to the nearest existing ancestor.
func link(recs []Record) {
	slices.SortStableFunc(recs, byCode)
	exists := make(map[string]bool, len(recs))
	for _, r := range recs {
		exists[r.Code] = true
	}
	for i := range recs {
		recs[i].Parent = ""
		for _, a := range ancestors(recs[i].Code) {
			if exists[a] {
				recs[i].Parent = a
				break
			}
		}
	}
}

// marker is a type abbreviation or word attached to a name.
type marker struct {
	s    string
	kind Kind
	fold bool // case-insensitive (whole words like "район")
}

// Russian markers; byLen puts longer ones first so "с.о." wins over "с.".
var (
	ruPrefixes = byLen([]marker{
		{"с.о.", Okrug, false}, {"с.а.", Okrug, false}, {"п.а.", Okrug, false},
		{"г.а.", Okrug, false}, {"р.а.", District, false},
		{"г.", City, false},
		{"с.", Village, false}, {"п.", Village, false}, {"пос.", Village, false},
		{"крес.пос.", Village, false}, {"аул", Village, true}, {"село", Village, true},
		{"поселок", Village, true}, {"посёлок", Village, true},
		{"район", District, true}, {"область", Region, true},
		{"ст.", Other, false}, {"рзд.", Other, false}, {"уч.", Other, false},
		{"отг.", Other, false}, {"отг.ст.", Other, false},
		{"кр.", Other, false}, {"кр.хоз", Other, false}, {"кр.хоз.", Other, false},
		{"Жив.т.", Other, false}, {"Жив.точка", Other, false},
		{"зимовка", Other, true}, {"точка", Other, true}, {"поселье", Other, true},
	})
	ruSuffixes = byLen([]marker{
		{"с.о.", Okrug, false}, {"с.о", Okrug, false}, {"с.а.", Okrug, false},
		{"п.а.", Okrug, false}, {"г.а.", Okrug, false},
		{"Г.А.", District, false}, {"р.а.", District, false},
		{"район", District, true}, {"область", Region, true},
		{"г.", City, false}, {"с.", Village, false}, {"п.", Village, false},
		{"ст.", Other, false}, {"рзд.", Other, false}, {"уч.", Other, false},
	})
	kkPrefixes = byLen([]marker{{s: "рзд."}, {s: "ст."}, {s: "а."}})
	kkSuffixes = byLen([]marker{
		{s: "облысы"}, {s: "ауданы"}, {s: "Қ.Ә."}, {s: "қ.ә."}, {s: "Г.А."},
		{s: "к.ә."}, {s: "а.ә."}, {s: "а.ә"}, {s: "а.о."}, {s: "а.о"},
		{s: "қ."}, {s: "ќ."}, {s: "к."}, {s: "а."}, {s: "а"}, {s: "с."}, {s: "с"},
		{s: "ауылы"}, {s: "ауыл"}, {s: "кенті"},
		{s: "ст."}, {s: "рзд."}, {s: "уч."}, {s: "ж."},
		{s: "қыстауы"}, {s: "мекені"}, {s: "мал жайылымы"},
		{s: "шарқож"}, {s: "шар.қож."}, {s: "қонысы"}, {s: "шар.қонысы"},
		{s: "шаруа қонысы"}, {s: "шаруаш.қонысы"},
	})
)

func byLen(ms []marker) []marker {
	slices.SortStableFunc(ms, func(a, b marker) int { return len(b.s) - len(a.s) })
	return ms
}

func hasPrefix(s string, m marker) bool {
	if len(s) < len(m.s) {
		return false
	}
	if m.fold {
		return strings.EqualFold(s[:len(m.s)], m.s)
	}
	return s[:len(m.s)] == m.s
}

func hasSuffix(s string, m marker) bool {
	if len(s) < len(m.s) {
		return false
	}
	if m.fold {
		return strings.EqualFold(s[len(s)-len(m.s):], m.s)
	}
	return s[len(s)-len(m.s):] == m.s
}

// cutPrefix strips a leading marker. An abbreviation ending in a dot may be
// glued to the name ("с.Акколь"); a word needs a space after it.
func cutPrefix(s string, ms []marker) (rest string, m marker, ok bool) {
	for _, m := range ms {
		if !hasPrefix(s, m) {
			continue
		}
		rest := s[len(m.s):]
		if !strings.HasSuffix(m.s, ".") && !strings.HasPrefix(rest, " ") {
			continue
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			return rest, m, true
		}
	}
	return s, marker{}, false
}

// cutSuffix strips a trailing marker preceded by a space, or by a dot when
// dotAfter is set ("Мұқажанов атын.а." -> "Мұқажанов атын.").
func cutSuffix(s string, ms []marker, dotAfter bool) (rest string, m marker, ok bool) {
	for _, m := range ms {
		if !hasSuffix(s, m) {
			continue
		}
		rest := s[:len(s)-len(m.s)]
		last, _ := utf8.DecodeLastRuneInString(rest)
		if last != ' ' && (!dotAfter || last != '.' || utf8.RuneCountInString(m.s) < 2) {
			continue
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			return rest, m, true
		}
	}
	return s, marker{}, false
}

// classifyRU derives the kind from the Russian name, falling back to the
// code level, and returns the cleaned name.
func classifyRU(code, name string) (Kind, string) {
	rest, m, ok := cutPrefix(name, ruPrefixes)
	if !ok {
		rest, m, ok = cutSuffix(name, ruSuffixes, false)
	}
	kind := m.kind
	switch {
	case IsTopLevel(code):
		kind = Region // includes г.Астана, г.Алматы, г.Шымкент
	case ok:
	case hasWord(name, "район"):
		kind = District
	case code[4:] == "00000":
		kind = District
	case code[6:] == "000":
		kind = Okrug
	default:
		kind = Other
	}
	return kind, tidy(rest, name)
}

func cleanKK(name string) string {
	rest, _, ok := cutSuffix(name, kkSuffixes, true)
	if !ok {
		rest, _, _ = cutPrefix(name, kkPrefixes)
	}
	return tidy(rest, name)
}

// hasWord reports whether s contains w as a separate word, ignoring case.
func hasWord(s, w string) bool {
	for f := range strings.FieldsSeq(s) {
		if strings.EqualFold(f, w) {
			return true
		}
	}
	return false
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// tidy collapses spaces and drops quotes around the whole name; it returns
// the tidied fallback if nothing is left.
func tidy(s, fallback string) string {
	s = collapse(s)
	for _, q := range [][2]string{{"«", "»"}, {`"`, `"`}, {"“", "”"}} {
		if len(s) > len(q[0])+len(q[1]) && strings.HasPrefix(s, q[0]) && strings.HasSuffix(s, q[1]) {
			inner := s[len(q[0]) : len(s)-len(q[1])]
			if !strings.ContainsAny(inner, `«»"“”`) {
				s = strings.TrimSpace(inner)
			}
		}
	}
	if s == "" {
		return collapse(fallback)
	}
	return s
}

// Validation limits.
const (
	MinRows       = 10_000
	TopLevelCount = 20 // 17 oblasts + Astana, Almaty, Shymkent
	MaxRowsChange = 0.15
)

// Validate runs sanity checks before a snapshot is replaced. prevRows is the
// row count of the previous snapshot, 0 to skip the change check.
func Validate(recs []Record, prevRows int) error {
	var errs []error
	add := func(format string, a ...any) {
		if len(errs) < 10 {
			errs = append(errs, fmt.Errorf(format, a...))
		} else if len(errs) == 10 {
			errs = append(errs, errors.New("more errors omitted"))
		}
	}
	if len(recs) < MinRows {
		add("%d rows, want at least %d", len(recs), MinRows)
	}
	if prevRows > 0 {
		if d := float64(len(recs)-prevRows) / float64(prevRows); d > MaxRowsChange || d < -MaxRowsChange {
			add("%d rows vs %d in the previous snapshot (%+.1f%%)", len(recs), prevRows, d*100)
		}
	}
	seen := make(map[string]bool, len(recs))
	top := 0
	for _, r := range recs {
		if !validCode(r.Code) {
			add("bad code %q", r.Code)
			continue
		}
		if seen[r.Code] {
			add("duplicate code %s", r.Code)
		}
		seen[r.Code] = true
		if IsTopLevel(r.Code) {
			top++
		}
		if r.NameRU == "" {
			add("%s: empty Russian name", r.Code)
		}
		if r.Kind < Region || r.Kind > Other {
			add("%s: invalid kind %d", r.Code, r.Kind)
		}
	}
	if top != TopLevelCount {
		add("%d top-level rows, want %d", top, TopLevelCount)
	}
	for _, r := range recs {
		switch {
		case !validCode(r.Code):
		case IsTopLevel(r.Code):
			if r.Parent != "" {
				add("%s: top-level row has parent %s", r.Code, r.Parent)
			}
		case r.Parent == "":
			add("%s: no parent", r.Code)
		case !seen[r.Parent]:
			add("%s: parent %s does not exist", r.Code, r.Parent)
		case !slices.Contains(ancestors(r.Code), r.Parent):
			add("%s: parent %s is not an ancestor", r.Code, r.Parent)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("kato: invalid data: %w", errors.Join(errs...))
	}
	return nil
}
