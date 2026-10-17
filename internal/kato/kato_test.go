package kato

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

const (
	nsMain = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`
	nsRel  = `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
)

// buildXLSX assembles a minimal workbook whose only sheet is stored under a
// non-default name, so the workbook relations must be followed.
func buildXLSX(t *testing.T, rows, sst string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?><workbook `+nsMain+` `+nsRel+
		`><sheets><sheet name="katonew1" sheetId="1" r:id="rId3"/></sheets></workbook>`)
	add("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>`+
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Target="styles.xml"/>`+
		`<Relationship Id="rId3" Target="worksheets/data.xml"/></Relationships>`)
	add("xl/worksheets/data.xml", `<?xml version="1.0" encoding="UTF-8"?><worksheet `+nsMain+
		`><dimension ref="A1:H9"/><sheetData>`+rows+`</sheetData></worksheet>`)
	if sst != "" {
		add("xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8"?><sst `+nsMain+`>`+sst+`</sst>`)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func parseBytes(b []byte) ([]Record, error) { return ParseXLSX(bytes.NewReader(b), int64(len(b))) }

const testSST = `<si><t>te</t></si><si><t>ab</t></si><si><t>kaz_name</t></si><si><t>rus_name</t></si>` +
	`<si><t>100000000</t></si><si><t>Абай облысы</t></si>` +
	// rich text with a phonetic hint that must be ignored
	`<si><r><t xml:space="preserve">область </t></r><r><rPr><b/></rPr><t>Абай</t></r><rPh sb="0" eb="1"><t>x</t></rPh></si>`

func TestParseXLSX(t *testing.T) {
	inline := func(ref, s string) string {
		return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, s)
	}
	rows := `<row r="1">` + inline("A1", "Классификатор КАТО") + `</row>` +
		// header from shared strings, columns C-F absent
		`<row r="2"><c r="A2" t="s"><v>0</v></c><c r="B2" t="s"><v>1</v></c>` +
		`<c r="G2" t="s"><v>2</v></c><c r="H2" t="s"><v>3</v></c></row>` +
		`<row r="3"><c r="A3" t="s"><v>4</v></c><c r="B3"><v>10</v></c>` +
		`<c r="G3" t="s"><v>5</v></c><c r="H3" t="s"><v>6</v></c></row>` +
		// numeric code, inline names
		`<row r="4"><c r="A4"><v>101000000</v></c>` + inline("G4", "Семей Қ.Ә.") + inline("H4", "Семей Г.А.") + `</row>` +
		// exponent form, missing kaz_name cell
		`<row r="5"><c r="A5"><v>1.0101E8</v></c>` + inline("H5", " г.Семей ") + `</row>` +
		`<row r="6"/>` +
		// cells without references follow each other
		`<row r="7">` + strings.Replace(inline("", "101010123"), ` r=""`, "", 1) +
		`<c/><c/><c/><c/><c/>` + strings.Replace(inline("", "Ақкөл а."), ` r=""`, "", 1) +
		strings.Replace(inline("", "с.Акколь"), ` r=""`, "", 1) + `</row>`

	recs, err := parseBytes(buildXLSX(t, rows, testSST))
	if err != nil {
		t.Fatal(err)
	}
	want := []Record{
		{Code: "100000000", Kind: Region, NameRU: "область Абай", NameKK: "Абай облысы", NameRUClean: "Абай", NameKKClean: "Абай"},
		{Code: "101000000", Parent: "100000000", Kind: District, NameRU: "Семей Г.А.", NameKK: "Семей Қ.Ә.", NameRUClean: "Семей", NameKKClean: "Семей"},
		{Code: "101010000", Parent: "101000000", Kind: City, NameRU: "г.Семей", NameRUClean: "Семей"},
		{Code: "101010123", Parent: "101010000", Kind: Village, NameRU: "с.Акколь", NameKK: "Ақкөл а.", NameRUClean: "Акколь", NameKKClean: "Ақкөл"},
	}
	if !slices.Equal(recs, want) {
		t.Errorf("got\n%+v\nwant\n%+v", recs, want)
	}
}

func TestParseXLSXErrors(t *testing.T) {
	header := `<row r="1"><c r="A1" t="s"><v>0</v></c><c r="G1" t="s"><v>2</v></c><c r="H1" t="s"><v>3</v></c></row>`
	tests := map[string][]byte{
		"no header": buildXLSX(t, `<row r="1"><c r="A1" t="s"><v>4</v></c></row>`, testSST),
		"partial header": buildXLSX(t,
			`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="H1" t="s"><v>3</v></c></row>`, testSST),
		"bad code": buildXLSX(t, header+
			`<row r="2"><c r="A2" t="inlineStr"><is><t>12-34</t></is></c></row>`, testSST),
		"short string code": buildXLSX(t, header+
			`<row r="2"><c r="A2" t="inlineStr"><is><t>1234</t></is></c></row>`, testSST),
		"bad shared index": buildXLSX(t, header+`<row r="2"><c r="A2" t="s"><v>99</v></c></row>`, testSST),
		"not a zip":        []byte("<html>not found</html>"),
	}
	for name, b := range tests {
		if _, err := parseBytes(b); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		code, ru, kk     string
		kind             Kind
		ruClean, kkClean string
	}{
		{"100000000", "область Абай", "Абай облысы", Region, "Абай", "Абай"},
		{"110000000", "Акмолинская область", "Ақмола облысы", Region, "Акмолинская", "Ақмола"},
		{"710000000", "г.Астана", "Астана қ.", Region, "Астана", "Астана"},
		{"101000000", "Семей Г.А.", "Семей Қ.Ә.", District, "Семей", "Семей"},
		{"101010000", "г.Семей", "Семей қ.", City, "Семей", "Семей"},
		{"103200000", "Абайский район", "Абай ауданы", District, "Абайский", "Абай"},
		{"593600000", "Район Магжана Жумабаева", "Мағжан Жұмабаев ауданы", District, "Магжана Жумабаева", "Мағжан Жұмабаев"},
		{"103230000", "Карааульский с.о.", "Қараауыл а.о.", Okrug, "Карааульский", "Қараауыл"},
		{"103233000", "Кенгирбайбийский с.о", "Кеңгірбай би а.о.", Okrug, "Кенгирбайбийский", "Кеңгірбай би"},
		{"104163000", "Чаганская п.а.", "Шаған к.ә.", Okrug, "Чаганская", "Шаған"},
		{"117020000", "Щучинская г.а.", "Щучинск қ.ә.", Okrug, "Щучинская", "Щучинск"},
		{"634820000", "г.а.Алтай", "Алтай қ.ә.", Okrug, "Алтай", "Алтай"},
		{"393245000", "с.о.имени Омара Шипина", "Омар Шипин атындағы а.о.", Okrug, "имени Омара Шипина", "Омар Шипин атындағы"},
		{"433639000", "им.Бухарбай батыра с.о.", "Бұқарбай батыр а.о.", Okrug, "им.Бухарбай батыра", "Бұқарбай батыр"},
		{"105030000", "Койгельдинский с.о.", "Қойгелді Аухадиев атын. а.о.", Okrug, "Койгельдинский", "Қойгелді Аухадиев атын."},
		{"351011000", "Әлихан Бөкейхан р.а.", "Әлихан Бөкейхан а.ә.", District, "Әлихан Бөкейхан", "Әлихан Бөкейхан"},
		{"351645100", "п.Саяк", "Саяқ к.", Village, "Саяк", "Саяқ"},
		{"113233100", "с.Акколь", "Ақкөл а.", Village, "Акколь", "Ақкөл"},
		{"395431100", "Заречное с.", "Заречный а.", Village, "Заречное", "Заречный"},
		{"623641100", "с.им.Мукажана Жумажанова", "Мұқажан Жұмажанов атын.а.", Village, "им.Мукажана Жумажанова", "Мұқажан Жұмажанов атын."},
		{"791910000", "район Тұран", "Тұран ауданы", District, "Тұран", "Тұран"},
		{"751110000", "Алмалинский район", "Алмалы ауданы", District, "Алмалинский", "Алмалы"},
		{"314049300", "ст.Жума", "Жұма ст.", Other, "Жума", "Жұма"},
		{"623871133", "зимовка Кишикотыр", "Кішіқотыр қыстауы", Other, "Кишикотыр", "Кішіқотыр"},
		{"621039214", "кр.хоз. «Касымов Б.Б.»", "«Касымов Б.Б.» шар.қож.", Other, "Касымов Б.Б.", "Касымов Б.Б."},
		{"393630000", "Аулиекольский с.о.", "Әулиекөл а.о.", Okrug, "Аулиекольский", "Әулиекөл"},
		// no marker: the code level decides
		{"195253103", "Молочный комплекс", "Сүт кешені", Other, "Молочный комплекс", "Сүт кешені"},
		{"104500000", "Маканчи", "Мақаншы", District, "Маканчи", "Мақаншы"},
		{"104530000", "Маканчи", "Мақаншы", Okrug, "Маканчи", "Мақаншы"},
		// marker alone is kept as the name
		{"103841300", "точка", "мекені", Other, "точка", "мекені"},
	}
	for _, tt := range tests {
		r := newRecord(tt.code, tt.ru, tt.kk)
		if r.Kind != tt.kind || r.NameRUClean != tt.ruClean || r.NameKKClean != tt.kkClean {
			t.Errorf("%s %q / %q: got %v %q %q, want %v %q %q", tt.code, tt.ru, tt.kk,
				r.Kind, r.NameRUClean, r.NameKKClean, tt.kind, tt.ruClean, tt.kkClean)
		}
	}
	if r := newRecord("105030000", "a  b ", "c"); r.NameRU != "a b" {
		t.Errorf("whitespace not collapsed: %q", r.NameRU)
	}
}

func TestKindText(t *testing.T) {
	for _, k := range Kinds {
		b, err := k.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var got Kind
		if err := got.UnmarshalText(b); err != nil || got != k {
			t.Errorf("%v: round trip got %v, %v", k, got, err)
		}
	}
	if _, err := Kind(0).MarshalText(); err == nil {
		t.Error("zero kind marshaled")
	}
}

// validSet returns 20 regions with 500 settlements each.
func validSet() []Record {
	var recs []Record
	for i := range TopLevelCount {
		region := fmt.Sprintf("%02d0000000", 10+i)
		recs = append(recs, Record{Code: region, Kind: Region, NameRU: "r"})
		for j := range 500 {
			recs = append(recs, Record{Code: fmt.Sprintf("%02d1010%03d", 10+i, j+1), Parent: region, Kind: Village, NameRU: "v"})
		}
	}
	return recs
}

func TestValidate(t *testing.T) {
	if err := Validate(validSet(), 0); err != nil {
		t.Fatal(err)
	}
	if err := Validate(validSet(), 9000); err != nil {
		t.Errorf("11%% growth rejected: %v", err)
	}
	tests := map[string]struct {
		edit func([]Record) []Record
		prev int
	}{
		"too few rows":   {func(r []Record) []Record { return r[:5000] }, 0},
		"duplicate code": {func(r []Record) []Record { return append(r, r[5]) }, 0},
		"missing parent": {func(r []Record) []Record { r[5].Parent = "990000000"; return r }, 0},
		"no parent":      {func(r []Record) []Record { r[5].Parent = ""; return r }, 0},
		"foreign parent": {func(r []Record) []Record { r[5].Parent = r[600].Parent; return r }, 0},
		"21 regions": {func(r []Record) []Record {
			return append(r, Record{Code: "900000000", Kind: Region, NameRU: "x"})
		}, 0},
		"19 regions":     {func(r []Record) []Record { r[0].Code = "101000000"; return r }, 0},
		"bad code":       {func(r []Record) []Record { r[7].Code = "12345"; return r }, 0},
		"empty name":     {func(r []Record) []Record { r[7].NameRU = ""; return r }, 0},
		"rows jump up":   {func(r []Record) []Record { return r }, 8000},
		"rows jump down": {func(r []Record) []Record { return r }, 12000},
	}
	for name, tt := range tests {
		if err := Validate(tt.edit(validSet()), tt.prev); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestFindXLSXLink(t *testing.T) {
	page := []byte(`<html><body>
<a href="/upload/iblock/3f1/k2/Методические_указания.xlsx">Методика</a>
<a class="doc" href='/upload/iblock/a7d/eqog/КАТО_18.09.2026.xlsx?v=1&amp;lang=ru'>КАТО (xlsx)</a>
<a href="/upload/iblock/000/old/КАТО_01.06.2026.xlsx">прежняя версия</a>
<a href="https://example.com/КАТО.pdf">pdf</a>
</body></html>`)
	base, _ := url.Parse("https://stat.gov.kz/ru/classifiers/statistical/21/")
	u, err := FindXLSXLink(page, base)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "stat.gov.kz" || u.Path != "/upload/iblock/a7d/eqog/КАТО_18.09.2026.xlsx" || u.RawQuery != "v=1&lang=ru" {
		t.Errorf("got %s", u)
	}
	if got := DisplayURL(u.String()); got != "https://stat.gov.kz/upload/iblock/a7d/eqog/КАТО_18.09.2026.xlsx?v=1&lang=ru" {
		t.Errorf("DisplayURL = %s", got)
	}
	if _, err := FindXLSXLink([]byte(`<a href="/a/Методика.xlsx">x</a>`), base); err == nil {
		t.Error("no KATO link: no error")
	}
	if _, err := FindXLSXLink([]byte(`<a href="/a/КАТО_18.09.2026.xlsx">x</a>`), nil); err == nil {
		t.Error("relative link without base: no error")
	}
}

func TestSourceDate(t *testing.T) {
	for in, want := range map[string]string{
		"https://stat.gov.kz/upload/iblock/a7d/x/КАТО_18.09.2026.xlsx":        "2026-09-18",
		"https://stat.gov.kz/upload/%D0%9A%D0%90%D0%A2%D0%9E_01.06.2025.xlsx": "2025-06-01",
		"/tmp/КАТО_31.02.2026.xlsx":                                           "",
		"https://stat.gov.kz/upload/iblock/a7d/12.12.2020/КАТО.xlsx":          "",
		"КАТО_18.09.2026.xlsx":                                                "2026-09-18",
	} {
		if got := SourceDate(in); got != want {
			t.Errorf("SourceDate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetch(t *testing.T) {
	xlsx := buildXLSX(t, `<row r="1"><c r="A1" t="s"><v>0</v></c></row>`, testSST)
	mux := http.NewServeMux()
	mux.HandleFunc("/page/", func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != UserAgent {
			t.Errorf("User-Agent %q", ua)
		}
		fmt.Fprint(w, `<a href="/files/report.xlsx">r</a> <a href="/files/КАТО_18.09.2026.xlsx">КАТО</a>`)
	})
	mux.HandleFunc("/bad/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<a href="/files/КАТО_01.01.2026.xlsx">КАТО</a>`)
	})
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files/КАТО_18.09.2026.xlsx":
			w.Write(xlsx)
		case "/files/КАТО_01.01.2026.xlsx":
			fmt.Fprint(w, "<html>maintenance</html>")
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	data, src, err := Fetch(context.Background(), srv.Client(), srv.URL+"/page/")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, xlsx) || src != srv.URL+"/files/КАТО_18.09.2026.xlsx" {
		t.Errorf("got %d bytes from %s", len(data), src)
	}
	if _, _, err := Fetch(context.Background(), srv.Client(), srv.URL+"/bad/"); err == nil {
		t.Error("non-zip file: no error")
	}
	if _, _, err := Fetch(context.Background(), srv.Client(), srv.URL+"/missing"); err == nil {
		t.Error("404 page: no error")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	recs := []Record{
		{Code: "101010000", Parent: "101000000", Kind: City, NameRU: "г.Семей", NameKK: "Семей қ.", NameRUClean: "Семей", NameKKClean: "Семей"},
		{Code: "100000000", Kind: Region, NameRU: "область Абай", NameKK: "Абай облысы", NameRUClean: "Абай", NameKKClean: "Абай"},
		{Code: "621039214", Parent: "621039000", Kind: Other, NameRU: `кр.хоз. "Тест, ""А""`, NameKK: "a\nb", NameRUClean: `Тест, "А"`},
	}
	var a, b bytes.Buffer
	if err := WriteSnapshot(&a, recs); err != nil {
		t.Fatal(err)
	}
	if err := WriteSnapshot(&b, recs); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("output is not reproducible")
	}
	got, err := ReadSnapshot(&a)
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(recs)
	slices.SortFunc(want, byCode)
	if !slices.Equal(got, want) {
		t.Errorf("got\n%+v\nwant\n%+v", got, want)
	}
	if recs[0].Code != "101010000" {
		t.Error("WriteSnapshot reordered the caller's slice")
	}
	if err := WriteSnapshot(&a, []Record{{Code: "100000000"}}); err == nil {
		t.Error("zero kind written")
	}
}

func TestEmbedded(t *testing.T) {
	recs, man, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) < 15000 || man.Rows != len(recs) || len(man.SHA256) != 64 || man.SourceDate == "" {
		t.Fatalf("%d records, manifest %+v", len(recs), man)
	}
	if err := Validate(recs, 0); err != nil {
		t.Fatal(err)
	}
	var regions []string
	idx := map[string]Record{}
	for _, r := range recs {
		idx[r.Code] = r
		if r.Kind == Region {
			regions = append(regions, r.NameRUClean)
		}
	}
	if len(regions) != TopLevelCount {
		t.Errorf("%d regions: %v", len(regions), regions)
	}
	for _, name := range []string{"Абай", "Жетісу", "Ұлытау", "Астана", "Алматы", "Шымкент"} {
		if !slices.Contains(regions, name) {
			t.Errorf("region %s missing", name)
		}
	}
	for code, want := range map[string]struct {
		kind  Kind
		clean string
	}{
		"101000000": {District, "Семей"},
		"101010000": {City, "Семей"},
		"103200000": {District, "Абайский"},
		"103230000": {Okrug, "Карааульский"},
		"351645100": {Village, "Саяк"},
		"791910000": {District, "Тұран"},
	} {
		if r := idx[code]; r.Kind != want.kind || r.NameRUClean != want.clean {
			t.Errorf("%s: got %v %q, want %v %q", code, r.Kind, r.NameRUClean, want.kind, want.clean)
		}
	}
	recs[0].Code = "changed"
	if again, _, _ := Embedded(); again[0].Code == "changed" {
		t.Error("Embedded returned shared storage")
	}
}
