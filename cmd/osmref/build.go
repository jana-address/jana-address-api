package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// skipHighways are named ways that are not addresses.
var skipHighways = map[string]bool{"footway": true, "cycleway": true, "path": true, "steps": true, "bridleway": true,
	"corridor": true, "platform": true, "proposed": true, "construction": true, "elevator": true, "bus_stop": true}

// counts is the number of votes per district: one per street segment and one per building.
type counts map[string]int

func (c counts) add(o counts) {
	for d, k := range o {
		c[d] += k
	}
}

func (c counts) best() (string, int, int) {
	var name string
	n, total := 0, 0
	for d, k := range c {
		total += k
		if k > n || (k == n && d < name) {
			name, n = d, k
		}
	}
	return name, n, total
}

// node is one spelling of a street: its key and the type word written in the name ("" if none).
// The Russian and Kazakh names of one way ("проспект Абая", "Абай даңғылы") are joined into one street,
// while "улица Абая" and "проспект Абая" stay two streets, as the case requires.
type node struct{ city, key, typ string }

type nodeData struct {
	votes, held counts            // held: without the buildings left out for the check
	houses      map[string]counts // house key -> district votes
}

type builder struct {
	nodes  map[node]*nodeData
	parent map[node]node
	areas  map[[2]string]counts
}

func (b *builder) data(n node) *nodeData {
	d := b.nodes[n]
	if d == nil {
		d = &nodeData{votes: counts{}, held: counts{}, houses: map[string]counts{}}
		b.nodes[n] = d
		b.parent[n] = n
	}
	return d
}

func (b *builder) find(n node) node {
	for b.parent[n] != n {
		b.parent[n] = b.parent[b.parent[n]]
		n = b.parent[n]
	}
	return n
}

func (b *builder) union(ns []node) {
	for _, n := range ns[1:] {
		if r1, r2 := b.find(ns[0]), b.find(n); r1 != r2 {
			b.parent[r2] = r1
		}
	}
}

// names lists the distinct spellings in the given tags; a value may hold several, split by ";".
func names(tags map[string]string, keys ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range keys {
		for _, v := range strings.Split(tags[k], ";") {
			if v = strings.TrimSpace(v); v != "" && !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// nodesOf turns names into street nodes of one city, without repeats.
func nodesOf(city string, ns []string) []node {
	var out []node
	seen := map[node]bool{}
	for _, n := range ns {
		k, typ := dict.StreetKey(n)
		if nd := (node{city, k, typ}); k != "" && !seen[nd] {
			seen[nd] = true
			out = append(out, nd)
		}
	}
	return out
}

// decision is the district of a street: the majority of its votes and their share.
type decision struct {
	district string
	share    float64
	n        int
}

func decide(c counts) decision {
	d, n, total := c.best()
	if total == 0 {
		return decision{}
	}
	return decision{district: d, share: float64(n) / float64(total), n: total}
}

// group is one street: the spellings joined by the ways that carry them.
type group struct {
	members     []node
	votes, held counts
	houses      map[string]counts
	typ         string // the type most of its spellings carry
}

func build(args []string) error {
	fl := flag.NewFlagSet("build", flag.ExitOnError)
	raw := fl.String("raw", defaultRaw, "directory with the Overpass answers")
	out := fl.String("out", defaultOut, "snapshot directory")
	partial := fl.Bool("partial", false, "skip districts not fetched yet (a trial build, not for the snapshot)")
	fl.Parse(args)

	b := &builder{nodes: map[node]*nodeData{}, parent: map[node]node{}, areas: map[[2]string]counts{}}
	type addr struct {
		n               node
		house, district string
	}
	var test []addr
	var manifestDistricts []map[string]any
	osmBase := ""
	for _, d := range districts {
		body, err := os.ReadFile(filepath.Join(*raw, fmt.Sprintf("%d.json", d.Rel)))
		if err != nil && *partial && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("%s %s: %w (run osmref fetch first)", d.City, d.District, err)
		}
		var ans answer
		if err := json.Unmarshal(body, &ans); err != nil {
			return fmt.Errorf("%s %s: %w", d.City, d.District, err)
		}
		if ans.OSM3S.Timestamp > osmBase {
			osmBase = ans.OSM3S.Timestamp
		}
		nWays, nHouses, nAreas := 0, 0, 0
		for _, e := range ans.Elements {
			tg := e.Tags
			if hw := tg["highway"]; hw != "" && !skipHighways[hw] {
				official := nodesOf(d.City, names(tg, "name", "name:ru", "name:kk"))
				other := nodesOf(d.City, names(tg, "alt_name", "old_name", "official_name", "short_name"))
				for _, n := range append(official, other...) {
					nd := b.data(n)
					nd.votes[d.District]++
					nd.held[d.District]++
				}
				if len(official) > 1 {
					b.union(official) // old and alternative names may now belong to another street
				}
				if len(official)+len(other) > 0 {
					nWays++
				}
			}
			if hn := tg["addr:housenumber"]; hn != "" {
				ns := nodesOf(d.City, names(tg, "addr:street", "addr:street:ru", "addr:street:kk"))
				if len(ns) > 0 {
					h := dict.HouseKey(hn)
					for _, n := range ns {
						nd := b.data(n)
						isTest := holdout(n, h)
						nd.votes[d.District]++
						if !isTest {
							nd.held[d.District]++
						}
						if nd.houses[h] == nil {
							nd.houses[h] = counts{}
						}
						nd.houses[h][d.District]++
						if isTest {
							test = append(test, addr{n, h, d.District})
						}
					}
					if len(ns) > 1 {
						b.union(ns)
					}
					nHouses++
				}
			}
			if tg["place"] != "" || tg["landuse"] == "residential" {
				for _, n := range names(tg, "name", "name:ru", "name:kk", "alt_name", "old_name") {
					k, _ := dict.StreetKey(n)
					if k == "" || isDigits(k) {
						continue
					}
					ak := [2]string{d.City, k}
					if b.areas[ak] == nil {
						b.areas[ak] = counts{}
					}
					b.areas[ak][d.District]++
					nAreas++
				}
			}
		}
		manifestDistricts = append(manifestDistricts, map[string]any{
			"relation": d.Rel, "city": d.City, "district": d.District, "elements": len(ans.Elements),
			"named_ways": nWays, "addresses": nHouses, "named_areas": nAreas,
		})
	}

	// streets: the spellings joined into one street sum their votes and buildings
	groups := map[node]*group{}
	for n, nd := range b.nodes {
		r := b.find(n)
		g := groups[r]
		if g == nil {
			g = &group{votes: counts{}, held: counts{}, houses: map[string]counts{}}
			groups[r] = g
		}
		g.members = append(g.members, n)
		g.votes.add(nd.votes)
		g.held.add(nd.held)
		for h, c := range nd.houses {
			if g.houses[h] == nil {
				g.houses[h] = counts{}
			}
			// the same building under a Russian and a Kazakh street name votes once per name; the
			// majority is what matters here
			g.houses[h].add(c)
		}
	}
	byKey := map[[2]string][]*group{} // city, key -> streets with a spelling of that key
	for _, g := range groups {
		tv := map[string]int{}
		for _, m := range g.members {
			if m.typ != "" {
				tv[m.typ] += b.nodes[m].votes.total()
			}
		}
		g.typ = bestKey(tv)
		seen := map[[2]string]bool{}
		for _, m := range g.members {
			if k := [2]string{m.city, m.key}; !seen[k] {
				seen[k] = true
				byKey[k] = append(byKey[k], g)
			}
		}
	}

	// streets.tsv: one row per spelling with its type, and one "*" row per key for a line that
	// writes no type: all streets of that key together, with their type when they share one
	var streets, houses, areas [][]string
	row := func(city, key, typ string, dc decision, onlyType string) {
		streets = append(streets, []string{city, key, typ, dc.district, strconv.FormatFloat(dc.share, 'f', 2, 64), strconv.Itoa(dc.n), onlyType})
	}
	houseRows := func(city, key, typ string, hs map[string]counts) {
		for h, c := range hs {
			if d, n, total := c.best(); n*2 > total {
				houses = append(houses, []string{city, key, typ, h, d})
			}
		}
	}
	for _, g := range groups {
		dc := decide(g.votes)
		for _, m := range g.members {
			if m.typ == "" {
				continue
			}
			row(m.city, m.key, m.typ, dc, g.typ)
			if dc.share < 1 {
				houseRows(m.city, m.key, m.typ, g.houses)
			}
		}
	}
	anyKey := func(gs []*group, held bool) (decision, string, map[string]counts) {
		c, hs, typs := counts{}, map[string]counts{}, map[string]bool{}
		for _, g := range gs {
			if held {
				c.add(g.held)
			} else {
				c.add(g.votes)
			}
			typs[g.typ] = true
			for h, hc := range g.houses {
				if hs[h] == nil {
					hs[h] = counts{}
				}
				hs[h].add(hc)
			}
		}
		only := ""
		if len(typs) == 1 {
			for t := range typs {
				only = t
			}
		}
		return decide(c), only, hs
	}
	for k, gs := range byKey {
		dc, only, hs := anyKey(gs, false)
		row(k[0], k[1], "*", dc, only)
		if dc.share < 1 {
			houseRows(k[0], k[1], "*", hs)
		}
	}
	for k, c := range b.areas {
		dc := decide(c)
		areas = append(areas, []string{k[0], k[1], dc.district, strconv.FormatFloat(dc.share, 'f', 2, 64)})
	}

	// holdout: every tenth building is left out of the votes; its district is then restored from the
	// street alone, the way the parser does for a house the directory does not know
	rep := map[string]map[string]int{}
	for _, a := range test {
		r := rep[a.n.city]
		if r == nil {
			r = map[string]int{}
			rep[a.n.city] = r
		}
		r["addresses"]++
		var dc decision
		if a.n.typ != "" {
			dc = decide(groups[b.find(a.n)].held)
		} else {
			dc, _, _ = anyKey(byKey[[2]string{a.n.city, a.n.key}], true)
		}
		if dc.share < dict.MinStreetShare {
			continue
		}
		r["restored"]++
		if dc.district == a.district {
			r["right"]++
		}
	}
	holdoutReport := map[string]any{}
	for city, r := range rep {
		cov, acc := 0.0, 0.0
		if r["addresses"] > 0 {
			cov = float64(r["restored"]) / float64(r["addresses"])
		}
		if r["restored"] > 0 {
			acc = float64(r["right"]) / float64(r["restored"])
		}
		holdoutReport[city] = map[string]any{"addresses": r["addresses"], "restored": r["restored"], "right": r["right"],
			"coverage": round3(cov), "accuracy": round3(acc)}
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	files := map[string]string{}
	for name, rows := range map[string][][]string{"streets.tsv.gz": streets, "houses.tsv.gz": houses, "areas.tsv.gz": areas} {
		sum, err := writeTSV(filepath.Join(*out, name), rows)
		if err != nil {
			return err
		}
		files[name] = sum
	}
	man := map[string]any{
		"source":             "OpenStreetMap, Overpass API (" + defaultAPI + ")",
		"license":            "ODbL 1.0, © OpenStreetMap contributors",
		"timestamp_osm_base": osmBase,
		"min_street_share":   dict.MinStreetShare,
		"districts":          manifestDistricts,
		"rows":               map[string]int{"streets": len(streets), "houses": len(houses), "areas": len(areas)},
		"sha256":             files,
		"holdout": map[string]any{
			"method": "every tenth building (by hash) is left out of the votes, then its district is restored from the street alone",
			"cities": holdoutReport,
		},
	}
	js, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "manifest.json"), append(js, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "streets %d, houses %d, areas %d\n%s\n", len(streets), len(houses), len(areas), js)
	return nil
}

func (c counts) total() int {
	t := 0
	for _, k := range c {
		t += k
	}
	return t
}

func bestKey(m map[string]int) string {
	best, n := "", 0
	for k, v := range m {
		if v > n || (v == n && k < best) {
			best, n = k, v
		}
	}
	return best
}

func holdout(n node, house string) bool {
	h := fnv.New32a()
	h.Write([]byte(n.city + "\x00" + n.key + "\x00" + house))
	return h.Sum32()%10 == 0
}

func round3(f float64) float64 { return float64(int(f*1000+0.5)) / 1000 }

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// writeTSV writes sorted rows, gzipped without a timestamp so the same input gives the same bytes.
func writeTSV(path string, rows [][]string) (string, error) {
	sort.Slice(rows, func(i, j int) bool { return strings.Join(rows[i], "\t") < strings.Join(rows[j], "\t") })
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		for _, f := range r {
			if strings.ContainsAny(f, "\t\n") {
				return "", fmt.Errorf("%s: tab or newline in %q", path, f)
			}
		}
		fmt.Fprintln(zw, strings.Join(r, "\t"))
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:]), nil
}
