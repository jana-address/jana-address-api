// Package eval scores parser output with the rules of the hackathon metric:
// per field, lower case, punctuation replaced by spaces, spaces collapsed, exact match.
// Address fields weigh 60%, details 25%, reference fields (district, living_area) 15%
// and the latter count only where the value is written in the string.
package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

// Gold is one labeled address.
type Gold struct {
	ID       string             `json:"id"`
	Raw      string             `json:"raw_address"`
	Expected map[string]*string `json:"expected"`
	InString []string           `json:"in_string"`
	Tags     []string           `json:"tags"`
}

var (
	AddrFields   = []string{"province", "city", "street_name", "house"}
	DetailFields = []string{"apartment", "entrance", "floor"}
	RefFields    = []string{"district", "living_area"}
	// InfoFields are reported but not part of the score.
	InfoFields = []string{"street_type", "landmark"}
)

// Norm applies the metric normalization; "" means null.
func Norm(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			r = ' '
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// Counter is correct/total for one field or group.
type Counter struct{ Correct, Total int }

func (c Counter) Acc() float64 {
	if c.Total == 0 {
		return 1
	}
	return float64(c.Correct) / float64(c.Total)
}

// Failure is one wrong field value.
type Failure struct{ ID, Raw, Field, Want, Got string }

// Report holds per-field and per-group accuracy and the weighted score.
type Report struct {
	Fields   map[string]*Counter
	Groups   map[string]*Counter
	Tags     map[string]*Counter // records with all scored fields right, per tag
	Score    float64
	Failures []Failure
	Records  int
}

// Load reads a JSONL gold file.
func Load(path string) ([]Gold, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Gold
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var g Gold
		if err := json.Unmarshal([]byte(line), &g); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		out = append(out, g)
	}
	return out, sc.Err()
}

// Score runs predict on every gold record and scores it.
func Score(gold []Gold, predict func(raw string) map[string]string) Report {
	rep := Report{Fields: map[string]*Counter{}, Groups: map[string]*Counter{}, Tags: map[string]*Counter{}, Records: len(gold)}
	for _, g := range [][]string{AddrFields, DetailFields, RefFields, InfoFields} {
		for _, f := range g {
			rep.Fields[f] = &Counter{}
		}
	}
	for _, name := range []string{"addr", "detail", "ref"} {
		rep.Groups[name] = &Counter{}
	}
	for _, g := range gold {
		pred := predict(g.Raw)
		before := len(rep.Failures)
		inStr := map[string]bool{}
		for _, f := range g.InString {
			inStr[f] = true
		}
		check := func(group, f string) {
			want := ""
			if v := g.Expected[f]; v != nil {
				want = Norm(*v)
			}
			if want == "" || (group == "ref" && !inStr[f]) {
				return
			}
			got := Norm(pred[f])
			rep.Fields[f].Total++
			if group != "" {
				rep.Groups[group].Total++
			}
			if got == want {
				rep.Fields[f].Correct++
				if group != "" {
					rep.Groups[group].Correct++
				}
				return
			}
			w := ""
			if v := g.Expected[f]; v != nil {
				w = *v
			}
			rep.Failures = append(rep.Failures, Failure{g.ID, g.Raw, f, w, pred[f]})
		}
		for _, f := range AddrFields {
			check("addr", f)
		}
		for _, f := range DetailFields {
			check("detail", f)
		}
		for _, f := range RefFields {
			check("ref", f)
		}
		for _, f := range InfoFields {
			check("", f)
		}
		ok := true
		for _, f := range rep.Failures[before:] {
			if f.Field != "street_type" && f.Field != "landmark" {
				ok = false
			}
		}
		for _, tag := range g.Tags {
			c := rep.Tags[tag]
			if c == nil {
				c = &Counter{}
				rep.Tags[tag] = c
			}
			c.Total++
			if ok {
				c.Correct++
			}
		}
	}
	rep.Score = 0.60*rep.Groups["addr"].Acc() + 0.25*rep.Groups["detail"].Acc() + 0.15*rep.Groups["ref"].Acc()
	return rep
}

// String renders a compact table.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "records: %d   SCORE: %.3f   (addr %.3f · detail %.3f · ref %.3f)\n", r.Records, r.Score,
		r.Groups["addr"].Acc(), r.Groups["detail"].Acc(), r.Groups["ref"].Acc())
	names := make([]string, 0, len(r.Fields))
	for f := range r.Fields {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		c := r.Fields[f]
		fmt.Fprintf(&b, "  %-12s %5d/%-5d %.3f\n", f, c.Correct, c.Total, c.Acc())
	}
	if len(r.Tags) > 0 {
		tags := make([]string, 0, len(r.Tags))
		for t := range r.Tags {
			tags = append(tags, t)
		}
		sort.Slice(tags, func(i, j int) bool { return r.Tags[tags[i]].Acc() < r.Tags[tags[j]].Acc() })
		b.WriteString("  records fully right, by tag (worst first):\n")
		for _, t := range tags {
			c := r.Tags[t]
			fmt.Fprintf(&b, "    %-15s %5d/%-5d %.3f\n", t, c.Correct, c.Total, c.Acc())
		}
	}
	return b.String()
}
