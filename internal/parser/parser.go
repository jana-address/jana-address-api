package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

type Parsed struct {
	Region     string `json:"region"`
	City       string `json:"city"`
	District   string `json:"district"`
	StreetType string `json:"street_type"`
	Street     string `json:"street"`
	House      string `json:"house"`
	Apartment  string `json:"apartment"`
	Entrance   string `json:"entrance"`
	Floor      string `json:"floor"`
	Landmark   string `json:"landmark"`
}

type Result struct {
	Parsed     Parsed            `json:"parsed"`
	Sources    map[string]string `json:"sources"` // "directory" | "string"
	Confidence float64           `json:"confidence"`
	Status     string            `json:"status"` // confident | review | unparsed
	Display    string            `json:"display"`
	Warnings   []string          `json:"warnings"`
}

var cities = []struct{ name, region string }{
	{"Алматы", "г. Алматы"}, {"Астана", "г. Астана"}, {"Шымкент", "г. Шымкент"},
	{"Актобе", "Актюбинская область"}, {"Караганда", "Карагандинская область"},
	{"Атырау", "Атырауская область"}, {"Павлодар", "Павлодарская область"},
	{"Тараз", "Жамбылская область"}, {"Семей", "Абайская область"}, {"Костанай", "Костанайская область"},
}

const num = `\d+[\p{L}]?(?:/\d+)?`

var (
	reKK       = regexp.MustCompile(`(?i)/\s*\p{L}+\s+қаласы`)
	reWords    = regexp.MustCompile(`(?i)(?:^|[\s,])(?:город|г\.|қаласы)(?:$|[\s,])`)
	reLand     = regexp.MustCompile(`(?i)((?:магазин|школа|тц|жк|кафе|аптека)\s+[\p{L}\d]+)`)
	reEnt      = regexp.MustCompile(`(?i)(\d+)\s*(?:подъезд|подьезд|под\.?)|(?:подъезд|подьезд|под\.?)\s*(\d+)`)
	reFloor    = regexp.MustCompile(`(?i)(\d+)\s*(?:этаж|эт\.?)|(?:этаж|эт\.?)\s*(\d+)`)
	reAptPre   = regexp.MustCompile(`(?i)(?:кв(?:артира)?|flat|пәтер)\.?\s*(\d+[\p{L}]?)`)
	reAptSuf   = regexp.MustCompile(`(?i)(\d+)\s*(?:кв(?:артира)?|flat)`)
	reDistrict = regexp.MustCompile(`(?i)(\p{L}+)\s+район`)
	reHouse    = regexp.MustCompile(`(?i)(?:^|[^\p{L}])(?:дом|д)\.?\s*(` + num + `)|(` + num + `)\s*дом`)
	reMkr1     = regexp.MustCompile(`(?i)(?:мкр|микрорайон|мкрн)\.?\s+(\p{L}+(?:\s\d+[\p{L}]?)?)`)
	reMkr2     = regexp.MustCompile(`(?i)(\p{L}+(?:\s\d+)?)\s+(?:микрорайон|мкр)(?:\s|,|$)`)
	reNumOnly  = regexp.MustCompile(`^` + num + `$`)
	reTrailNum = regexp.MustCompile(`^(.+?)\s+(` + num + `)$`)
	reTypePre  = regexp.MustCompile(`(?i)^(ул|улица|пр|пр-т|проспект|пер|переулок|бул|бульвар)\.?\s+(.+)$`)
	reTypeSuf  = regexp.MustCompile(`(?i)^(.+?)\s+(улица|переулок|проспект|бульвар)$`)
)

var typeNorm = map[string]string{
	"ул": "улица", "улица": "улица", "пр": "проспект", "пр-т": "проспект", "проспект": "проспект",
	"пер": "переулок", "переулок": "переулок", "бул": "бульвар", "бульвар": "бульвар",
}

var typeShort = map[string]string{
	"улица": "ул.", "проспект": "пр.", "переулок": "пер.", "бульвар": "бул.", "микрорайон": "мкр.",
}

// take вырезает первое совпадение из строки и возвращает первую непустую группу.
func take(s *string, re *regexp.Regexp) string {
	m := re.FindStringSubmatchIndex(*s)
	if m == nil {
		return ""
	}
	val := ""
	for i := 2; i+1 < len(m); i += 2 {
		if m[i] >= 0 {
			val = (*s)[m[i]:m[i+1]]
			break
		}
	}
	*s = (*s)[:m[0]] + "," + (*s)[m[1]:]
	return strings.TrimSpace(val)
}

func Parse(raw string) Result {
	s := strings.TrimSpace(raw)
	var p Parsed
	src := map[string]string{}

	s = reKK.ReplaceAllString(s, "")
	for _, c := range cities {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(c.name))
		if found := re.FindString(s); found != "" {
			// название возвращаем ровно так, как оно написано в исходной строке
			p.City, p.Region = found, c.region
			s = re.ReplaceAllString(s, ",")
			src["city"], src["region"] = "string", "directory"
			break
		}
	}
	s = reWords.ReplaceAllString(s, ",")

	p.Entrance = take(&s, reEnt)
	p.Floor = take(&s, reFloor)
	if p.Apartment = take(&s, reAptPre); p.Apartment == "" {
		p.Apartment = take(&s, reAptSuf)
	}
	p.Landmark = take(&s, reLand)
	p.District = take(&s, reDistrict)
	p.House = take(&s, reHouse)
	mkr := take(&s, reMkr1)
	if mkr == "" {
		mkr = take(&s, reMkr2)
	}

	var segs []string
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			segs = append(segs, t)
		}
	}
	// отдельный сегмент-число — это номер дома
	rest := segs[:0:0]
	for _, t := range segs {
		if p.House == "" && reNumOnly.MatchString(t) {
			p.House = t
			continue
		}
		rest = append(rest, t)
	}

	switch {
	case mkr != "":
		p.StreetType, p.Street = "микрорайон", mkr
	case len(rest) > 0:
		t := rest[0]
		if m := reTypePre.FindStringSubmatch(t); m != nil {
			p.StreetType, p.Street = typeNorm[strings.ToLower(m[1])], strings.TrimSpace(m[2])
		} else if m := reTypeSuf.FindStringSubmatch(t); m != nil {
			p.StreetType, p.Street = typeNorm[strings.ToLower(m[2])], strings.TrimSpace(m[1])
		} else {
			p.Street = t
		}
		if p.House == "" {
			if m := reTrailNum.FindStringSubmatch(p.Street); m != nil {
				p.Street, p.House = strings.TrimSpace(m[1]), m[2]
			}
		}
	}

	for k, v := range map[string]string{
		"district": p.District, "street_type": p.StreetType, "street": p.Street, "house": p.House,
		"apartment": p.Apartment, "entrance": p.Entrance, "floor": p.Floor, "landmark": p.Landmark,
	} {
		if v != "" {
			src[k] = "string"
		}
	}

	var w []string
	if p.City == "" {
		w = append(w, "missing_city")
	}
	if p.Street == "" {
		w = append(w, "missing_street")
	}
	if p.House == "" {
		w = append(w, "missing_house")
	}

	c := 0.0
	add := func(ok bool, v float64) {
		if ok {
			c += v
		}
	}
	add(p.City != "", .2)
	add(p.Street != "", .3)
	add(p.House != "", .3)
	add(p.Apartment != "", .1)
	add(p.StreetType != "", .1)
	add(p.City == "", -.1)
	c = math.Round(math.Min(math.Max(c, .05), .97)*100) / 100

	status := "review"
	switch {
	case p.Street == "" || c < .5:
		status = "unparsed"
	case c >= .85:
		status = "confident"
	}

	return Result{Parsed: p, Sources: src, Confidence: c, Status: status, Display: display(p), Warnings: w}
}

func display(p Parsed) string {
	var parts []string
	if p.City != "" {
		parts = append(parts, p.City)
	}
	addr := strings.TrimSpace(typeShort[p.StreetType] + " " + p.Street)
	if p.House != "" {
		addr += ", " + p.House
	}
	if addr = strings.Trim(addr, ", "); addr != "" {
		parts = append(parts, addr)
	}
	if p.Entrance != "" {
		parts = append(parts, "под. "+p.Entrance)
	}
	if p.Floor != "" {
		parts = append(parts, "эт. "+p.Floor)
	}
	if p.Apartment != "" {
		parts = append(parts, "кв "+p.Apartment)
	}
	return strings.Join(parts, " · ")
}

func idFor(i int) string { return fmt.Sprintf("rec_%05d", i+1) }
