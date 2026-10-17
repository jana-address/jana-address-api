package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jana-address/jana-address-api/internal/addr"
	"github.com/jana-address/jana-address-api/internal/parser"
)

const maxBody = 64 << 20

type inItem struct {
	ID  json.RawMessage `json:"id"`
	Raw json.RawMessage `json:"raw_address"`
}

// parseBatch is the jury endpoint: JSON array in, JSON array out, same order, one result per record.
// It is stateless: every record is parsed from its text, nothing is looked up by id or by the whole string.
func (a *api) parseBatch(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(10 * time.Minute))
	_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Minute))

	body, err := readBody(w, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	items, err := decodeItems(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	start := time.Now()
	out := make([]addr.Result, len(items))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				out[i] = a.parseItem(items[i])
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	w.Header().Set("X-Records", strconv.Itoa(len(out)))
	w.Header().Set("X-Parse-Ms", strconv.FormatInt(time.Since(start).Milliseconds(), 10))
	writeJSON(w, http.StatusOK, out)
}

// parseItem never fails: a bad or crashing record yields nulls with zero confidence.
func (a *api) parseItem(it inItem) (res addr.Result) {
	res = addr.Result{ID: it.ID, RawAddress: it.Raw}
	defer func() {
		if v := recover(); v != nil {
			a.log.Error("parse panic", "panic", v)
			res = addr.Result{ID: it.ID, RawAddress: it.Raw}
		}
	}()
	var raw string
	if len(it.Raw) == 0 || it.Raw[0] != '"' || json.Unmarshal(it.Raw, &raw) != nil {
		return res // missing or non-string raw_address: nulls, zero confidence
	}
	parser.Parse(raw).Fill(&res)
	return res
}

type parseResponse struct {
	Result  addr.Result       `json:"result"`
	Spans   []parser.Span     `json:"spans"`
	Sources map[string]string `json:"sources"`
	Status  string            `json:"status"`
	// Suggestion is the address in a clean form ("Астана, проспект Мәңгілік Ел, д. 55") for the hint under the input.
	Suggestion string `json:"suggestion"`
}

// parseOne parses a single line for the UI and also returns highlight spans and value sources.
func (a *api) parseOne(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RawAddress string `json:"raw_address"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected {\"raw_address\": \"...\"}"})
		return
	}
	raw, _ := json.Marshal(in.RawAddress)
	o := parser.Parse(in.RawAddress)
	resp := parseResponse{Result: addr.Result{RawAddress: raw}, Spans: o.Spans, Sources: map[string]string{}, Status: o.Status(), Suggestion: o.Suggest()}
	o.Fill(&resp.Result)
	if resp.Spans == nil {
		resp.Spans = []parser.Span{}
	}
	for _, f := range parser.Fields {
		resp.Sources[string(f)] = o.SourceOf(f)
	}
	writeJSON(w, http.StatusOK, resp)
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, errors.New("multipart: expected a \"file\" field")
		}
		defer f.Close()
		return io.ReadAll(f)
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		return nil, errors.New("request body is too large or unreadable")
	}
	return body, nil
}

// decodeItems accepts a JSON array (the jury format), an object wrapping an array, or NDJSON.
func decodeItems(body []byte) ([]inItem, error) {
	body = bytes.TrimSpace(bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF")))
	if len(body) == 0 {
		return nil, errors.New("empty body: expected a JSON array of {id, raw_address}")
	}
	switch body[0] {
	case '[':
		var raws []json.RawMessage
		if err := json.Unmarshal(body, &raws); err != nil {
			return nil, errors.New("invalid JSON array: " + err.Error())
		}
		items := make([]inItem, len(raws))
		for i, rm := range raws {
			_ = json.Unmarshal(rm, &items[i]) // a non-object element keeps its slot with nulls
		}
		return items, nil
	case '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(body, &obj) == nil {
			for _, k := range []string{"items", "records", "data", "addresses"} {
				if v, ok := obj[k]; ok && len(v) > 0 && v[0] == '[' {
					return decodeItems(v)
				}
			}
			var it inItem
			_ = json.Unmarshal(body, &it)
			return []inItem{it}, nil
		}
		var items []inItem
		for _, line := range bytes.Split(body, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			var it inItem
			_ = json.Unmarshal(line, &it)
			items = append(items, it)
		}
		return items, nil
	}
	return nil, errors.New("expected a JSON array of {id, raw_address}")
}
