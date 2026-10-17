package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
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
	// a few batches at a time: each can hold a 64 MiB body and its results in memory
	select {
	case a.batches <- struct{}{}:
		defer func() { <-a.batches }()
	case <-r.Context().Done():
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Minute))
	r.Body = &minRateBody{ReadCloser: r.Body, rc: rc, start: time.Now()}

	body, err := readBody(w, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	items, err := decodeItems(body)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, errTooManyRecords) {
			code = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}

	start := time.Now()
	ctx := r.Context()

	// Workers take chunks of records by a shared counter, parse them and encode each chunk to JSON;
	// the handler writes the chunks strictly in input order as soon as the next one is ready.
	// A worker takes a chunk only when fewer than two per worker wait for the writer, so a client that
	// reads slowly does not make the whole response pile up in memory.
	nchunks := (len(items) + batchChunk - 1) / batchChunk
	parts := make([][]byte, nchunks)
	ready := make([]chan struct{}, nchunks)
	for c := range ready {
		ready[c] = make(chan struct{})
	}
	workers := min(runtime.GOMAXPROCS(0), nchunks)
	ahead := make(chan struct{}, 2*workers)
	var next atomic.Int64
	for range workers {
		go func() {
			var tmp bytes.Buffer
			enc := json.NewEncoder(&tmp)
			enc.SetEscapeHTML(false)
			for {
				select {
				case ahead <- struct{}{}:
				case <-ctx.Done():
					return
				}
				c := int(next.Add(1) - 1)
				if c >= nchunks {
					return
				}
				if ctx.Err() == nil { // the client is still there
					parts[c] = a.encodeChunk(items[c*batchChunk:min((c+1)*batchChunk, len(items))], enc, &tmp)
				}
				close(ready[c])
			}
		}()
	}

	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Records", strconv.Itoa(len(items)))
	h.Set("Trailer", "X-Parse-Ms")
	w.WriteHeader(http.StatusOK)
	bw := bufio.NewWriterSize(w, 64<<10)
	_ = bw.WriteByte('[')
	for c := range nchunks {
		select {
		case <-ready[c]:
		case <-ctx.Done():
			return
		}
		if c > 0 {
			_ = bw.WriteByte(',')
		}
		if _, err := bw.Write(parts[c]); err != nil {
			return
		}
		parts[c] = nil
		<-ahead
	}
	_, _ = bw.WriteString("]\n")
	_ = bw.Flush()
	h.Set("X-Parse-Ms", strconv.FormatInt(time.Since(start).Milliseconds(), 10))
}

// batchChunk is how many records a worker parses and encodes at a time.
const batchChunk = 256

// maxBatches is how many batch requests run at once; the others wait for a slot.
const maxBatches = 4

// A batch body must keep coming: the read deadline is bodyGrace plus one second for every
// bodyMinRate bytes received, at most bodyMaxTime from the start. A client that drips a byte now
// and then is cut off after about 30 s, while 64 MiB on a 1 Mbit/s line still gets through.
var (
	bodyGrace   = 30 * time.Second // variables, so the test can shorten them
	bodyMaxTime = 10 * time.Minute
)

const bodyMinRate = 64 << 10

type minRateBody struct {
	io.ReadCloser
	rc    *http.ResponseController
	start time.Time
	n     int64
}

func (b *minRateBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n += int64(n)
	d := bodyGrace + time.Duration(b.n)*time.Second/bodyMinRate
	_ = b.rc.SetReadDeadline(b.start.Add(min(d, bodyMaxTime)))
	return n, err
}

// encodeChunk parses records and returns them as comma-separated JSON objects (no brackets).
func (a *api) encodeChunk(items []inItem, enc *json.Encoder, tmp *bytes.Buffer) []byte {
	out := make([]byte, 0, len(items)*400)
	for i, it := range items {
		res := a.parseItem(it)
		tmp.Reset()
		if enc.Encode(&res) != nil {
			tmp.Reset()
			_ = enc.Encode(&addr.Result{}) // never break the array: a record that cannot be encoded becomes nulls
		}
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, bytes.TrimSuffix(tmp.Bytes(), []byte("\n"))...)
	}
	return out
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

// maxRecords bounds one batch. A 64 MiB body of real addresses holds about 600 000 records,
// while "[{},{},…]" would expand to tens of millions of null results.
const maxRecords = 1_000_000

var errTooManyRecords = fmt.Errorf("more than %d records in one request", maxRecords)

// decodeItems accepts a JSON array (the jury format), an object wrapping an array, or NDJSON
// (also pretty-printed objects one after another). Records are counted while they are decoded,
// so an oversized batch is rejected before it takes memory.
func decodeItems(body []byte) ([]inItem, error) {
	body = bytes.TrimSpace(bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF")))
	if len(body) == 0 {
		return nil, errors.New("empty body: expected a JSON array of {id, raw_address}")
	}
	switch body[0] {
	case '[':
		dec := json.NewDecoder(bytes.NewReader(body))
		_, _ = dec.Token() // the opening bracket
		var items []inItem
		var err error
		for err == nil && dec.More() {
			items, err = nextItem(dec, items)
		}
		if err == nil {
			_, err = dec.Token() // the closing bracket
		}
		if err == nil && unread(dec, body) {
			err = errors.New("data after the array")
		}
		if err != nil && !errors.Is(err, errTooManyRecords) {
			err = errors.New("invalid JSON array: " + err.Error())
		}
		return items, err
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
		dec := json.NewDecoder(bytes.NewReader(body))
		var items []inItem
		var err error
		for err == nil && dec.More() {
			items, err = nextItem(dec, items)
		}
		if err == nil && unread(dec, body) { // More stops at a stray '}' or ']'
			err = errors.New("unexpected data")
		}
		if err != nil && !errors.Is(err, errTooManyRecords) {
			err = fmt.Errorf("invalid NDJSON at record %d: %v", len(items)+1, err)
		}
		return items, err
	}
	return nil, errors.New("expected a JSON array of {id, raw_address}")
}

// unread reports whether anything but white space follows the last decoded value.
func unread(dec *json.Decoder, body []byte) bool {
	return len(bytes.TrimSpace(body[dec.InputOffset():])) > 0
}

// nextItem decodes one record; a record that is not an object keeps its slot with nulls.
func nextItem(dec *json.Decoder, items []inItem) ([]inItem, error) {
	if len(items) == maxRecords {
		return items, errTooManyRecords
	}
	var v json.RawMessage
	if err := dec.Decode(&v); err != nil {
		return items, err
	}
	var it inItem
	_ = json.Unmarshal(v, &it)
	return append(items, it), nil
}
