package kato

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

// Limits on decompressed parts guard against zip bombs.
const (
	maxSheetBytes   = 256 << 20
	maxStringsBytes = 64 << 20
	maxColumns      = 16384 // Excel limit
	headerScanRows  = 20
)

var errTooLarge = errors.New("kato: xlsx part exceeds size limit")

// ParseXLSX reads the KATO workbook published by stat.gov.kz. It streams
// the first worksheet and expects a header row with the columns te
// (9-digit code), rus_name and kaz_name. Records are sorted by code and
// linked to their parents; call Validate before trusting them.
func ParseXLSX(r io.ReaderAt, size int64) ([]Record, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("kato: open xlsx: %w", err)
	}
	sheet := firstSheet(zr)
	sst, err := readSharedStrings(zr)
	if err != nil {
		return nil, err
	}
	f, err := zr.Open(sheet)
	if err != nil {
		return nil, fmt.Errorf("kato: open %s: %w", sheet, err)
	}
	defer f.Close()
	recs, err := readSheet(limitReader(f, maxSheetBytes), sst)
	if err != nil {
		return nil, err
	}
	link(recs)
	return recs, nil
}

// firstSheet resolves the first worksheet through the workbook relations,
// falling back to the conventional path.
func firstSheet(zr *zip.Reader) string {
	const fallback = "xl/worksheets/sheet1.xml"
	var wb struct {
		Sheets []struct {
			Attrs []xml.Attr `xml:",any,attr"`
		} `xml:"sheets>sheet"`
	}
	var rels struct {
		Rel []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if decodePart(zr, "xl/workbook.xml", &wb) != nil || len(wb.Sheets) == 0 ||
		decodePart(zr, "xl/_rels/workbook.xml.rels", &rels) != nil {
		return fallback
	}
	var id string
	for _, a := range wb.Sheets[0].Attrs {
		if a.Name.Local == "id" && a.Name.Space != "" { // r:id
			id = a.Value
		}
	}
	for _, rel := range rels.Rel {
		if id == "" || rel.ID != id {
			continue
		}
		if t, ok := strings.CutPrefix(rel.Target, "/"); ok {
			return t
		}
		return path.Join("xl", rel.Target)
	}
	return fallback
}

func decodePart(zr *zip.Reader, name string, v any) error {
	f, err := zr.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return xml.NewDecoder(limitReader(f, 1<<20)).Decode(v)
}

// richText is the content of <si> or <is>: plain <t> or rich-text runs.
// Phonetic hints (<rPh>) are not mapped and therefore skipped.
type richText struct {
	T string `xml:"t"`
	R []struct {
		T string `xml:"t"`
	} `xml:"r"`
}

func (rt *richText) String() string {
	if len(rt.R) == 0 {
		return rt.T
	}
	var b strings.Builder
	b.WriteString(rt.T)
	for _, r := range rt.R {
		b.WriteString(r.T)
	}
	return b.String()
}

func readSharedStrings(zr *zip.Reader) ([]string, error) {
	f, err := zr.Open("xl/sharedStrings.xml")
	if err != nil {
		return nil, nil // all strings may be inline
	}
	defer f.Close()
	var out []string
	d := xml.NewDecoder(limitReader(f, maxStringsBytes))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("kato: sharedStrings.xml: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "si" {
			var si richText
			if err := d.DecodeElement(&si, &se); err != nil {
				return nil, fmt.Errorf("kato: sharedStrings.xml: %w", err)
			}
			out = append(out, si.String())
		}
	}
}

type xmlCell struct {
	Ref  string    `xml:"r,attr"`
	Type string    `xml:"t,attr"`
	V    string    `xml:"v"`
	IS   *richText `xml:"is"`
}

type cell struct {
	s       string
	numeric bool
}

func (c xmlCell) value(sst []string) (cell, error) {
	switch c.Type {
	case "s":
		i, err := strconv.Atoi(strings.TrimSpace(c.V))
		if err != nil || i < 0 || i >= len(sst) {
			return cell{}, fmt.Errorf("kato: cell %s: bad shared string index %q", c.Ref, c.V)
		}
		return cell{s: sst[i]}, nil
	case "inlineStr":
		if c.IS == nil {
			return cell{}, nil
		}
		return cell{s: c.IS.String()}, nil
	case "", "n":
		return cell{s: c.V, numeric: true}, nil
	case "e":
		return cell{}, nil
	default: // str, b, d
		return cell{s: c.V}, nil
	}
}

// colIndex converts the letters of a reference like "AB12" to a 0-based column.
func colIndex(ref string) (int, bool) {
	n, i := 0, 0
	for ; i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z'; i++ {
		n = n*26 + int(ref[i]-'A'+1)
		if n > maxColumns {
			return 0, false
		}
	}
	return n - 1, i > 0
}

type columns struct{ code, ru, kk int }

func findHeader(row []cell) (columns, bool) {
	c := columns{-1, -1, -1}
	for i, v := range row {
		switch strings.ToLower(strings.TrimSpace(v.s)) {
		case "te":
			c.code = i
		case "rus_name":
			c.ru = i
		case "kaz_name":
			c.kk = i
		}
	}
	return c, c.code >= 0 && c.ru >= 0 && c.kk >= 0
}

func readSheet(r io.Reader, sst []string) ([]Record, error) {
	d := xml.NewDecoder(r)
	var (
		recs   []Record
		row    []cell
		col    columns
		header bool
		rowNum int
		next   int // column of the next cell without a reference
	)
	at := func(i int) cell {
		if i < len(row) {
			return row[i]
		}
		return cell{}
	}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("kato: worksheet: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				row, next = row[:0], 0
				rowNum++
			case "c":
				var xc xmlCell
				if err := d.DecodeElement(&xc, &t); err != nil {
					return nil, fmt.Errorf("kato: worksheet: %w", err)
				}
				i := next
				if xc.Ref != "" {
					var ok bool
					if i, ok = colIndex(xc.Ref); !ok {
						return nil, fmt.Errorf("kato: bad cell reference %q", xc.Ref)
					}
				}
				if i >= maxColumns {
					return nil, fmt.Errorf("kato: too many columns in row %d", rowNum)
				}
				next = i + 1
				v, err := xc.value(sst)
				if err != nil {
					return nil, err
				}
				for len(row) <= i {
					row = append(row, cell{})
				}
				row[i] = v
			}
		case xml.EndElement:
			if t.Name.Local != "row" {
				continue
			}
			if !header {
				if col, header = findHeader(row); !header && rowNum >= headerScanRows {
					return nil, errors.New("kato: header with te, rus_name, kaz_name not found")
				}
				continue
			}
			c := at(col.code)
			if strings.TrimSpace(c.s) == "" {
				continue // blank row
			}
			code, ok := normCode(c)
			if !ok {
				return nil, fmt.Errorf("kato: row %d: bad code %q", rowNum, c.s)
			}
			recs = append(recs, newRecord(code, at(col.ru).s, at(col.kk).s))
		}
	}
	if !header {
		return nil, errors.New("kato: header with te, rus_name, kaz_name not found")
	}
	return recs, nil
}

// normCode returns the 9-digit code. A numeric cell may come as "1E+8" or
// lose leading zeros, so it is reformatted and padded.
func normCode(c cell) (string, bool) {
	s := strings.TrimSpace(c.s)
	if c.numeric && !validCode(s) {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f < 0 || f >= 1e9 || f != float64(int64(f)) {
			return "", false
		}
		s = fmt.Sprintf("%09d", int64(f))
	}
	return s, validCode(s)
}

// limitReader fails with errTooLarge instead of silently truncating.
func limitReader(r io.Reader, n int64) io.Reader { return &limited{r: r, n: n} }

type limited struct {
	r io.Reader
	n int64
}

func (l *limited) Read(p []byte) (int, error) {
	if l.n <= 0 {
		var one [1]byte
		if _, err := io.ReadFull(l.r, one[:]); err != nil {
			return 0, err // io.EOF exactly at the limit
		}
		return 0, errTooLarge
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	k, err := l.r.Read(p)
	l.n -= int64(k)
	return k, err
}
