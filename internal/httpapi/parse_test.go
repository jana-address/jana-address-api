package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var wantKeys = []string{"id", "raw_address", "province", "city", "district", "living_area", "street_type",
	"street_name", "house", "apartment", "entrance", "floor", "landmark", "confidence"}

// keysInOrder decodes an array of objects and returns the key order of each object.
func keysInOrder(t *testing.T, body []byte) [][]string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	var out [][]string
	if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
		t.Fatalf("expected array, got %v %v", tok, err)
	}
	for dec.More() {
		var keys []string
		if tok, _ := dec.Token(); tok != json.Delim('{') {
			t.Fatalf("expected object")
		}
		for dec.More() {
			k, _ := dec.Token()
			keys = append(keys, k.(string))
			var v json.RawMessage
			if err := dec.Decode(&v); err != nil {
				t.Fatal(err)
			}
		}
		_, _ = dec.Token()
		out = append(out, keys)
	}
	return out
}

func TestParseBatchContract(t *testing.T) {
	h := New(fakeDB{}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	in := `[{"id":"rec_00417","raw_address":"город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70"},
	        {"id":42,"raw_address":"Кулманова 133"},
	        {"id":"bad","raw_address":null},
	        "not an object"]`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/parse/batch", strings.NewReader("\xEF\xBB\xBF"+in)))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.Bytes()
	objs := keysInOrder(t, body)
	if len(objs) != 4 {
		t.Fatalf("records = %d, want 4", len(objs))
	}
	for i, keys := range objs {
		if strings.Join(keys, ",") != strings.Join(wantKeys, ",") {
			t.Fatalf("record %d keys = %v", i, keys)
		}
	}
	var res []map[string]any
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatal(err)
	}
	if res[0]["raw_address"] != "город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70" || res[0]["street_name"] != "Аксай 3а" ||
		res[0]["district"] != "Ауэзовский" || res[0]["house"] != "59" || res[0]["apartment"] != "70" {
		t.Fatalf("record 0 = %v", res[0])
	}
	if res[1]["id"] != float64(42) || res[1]["house"] != "133" {
		t.Fatalf("record 1 = %v", res[1])
	}
	if res[2]["street_name"] != nil || res[2]["confidence"] != float64(0) {
		t.Fatalf("record 2 = %v", res[2])
	}
}

func TestParseBatchNDJSON(t *testing.T) {
	h := New(fakeDB{}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	in := "{\"id\":\"a\",\"raw_address\":\"жуалы 19\"}\n{\"id\":\"b\",\"raw_address\":\"Ул .Курильская 9 кв 1\"}\n"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/parse/batch", strings.NewReader(in)))
	var res []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || len(res) != 2 || res[1]["apartment"] != "1" {
		t.Fatalf("ndjson: %v %v", res, err)
	}
}

func TestParseBatchInputErrors(t *testing.T) {
	h := New(fakeDB{}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/parse/batch", strings.NewReader(body)))
		return rec
	}
	// a broken body is an error, not one null record per line
	for _, in := range []string{
		"{\"id\":\"a\",\"raw_address\":\"жуалы 19\"}\n}\n",
		"{\n  \"items\": [\n    {\"id\": 1, \"raw_address\": \"жуалы 19\"},\n",
		`[{"id":1,"raw_address":"жуалы 19"},`,
		`[{"id":1,"raw_address":"жуалы 19"}] x`,
	} {
		if rec := post(in); rec.Code != http.StatusBadRequest {
			t.Errorf("%q: code = %d, want 400: %s", in, rec.Code, rec.Body.String())
		}
	}
	// pretty-printed objects one after another are records, not lines
	rec := post("{\n  \"id\": \"a\",\n  \"raw_address\": \"жуалы 19\"\n}\n{\n  \"id\": \"b\",\n  \"raw_address\": \"Кулманова 133\"\n}\n")
	var res []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || len(res) != 2 || res[1]["house"] != "133" {
		t.Fatalf("pretty objects: %d %v %v", rec.Code, res, err)
	}
	// "[{},{},…]" must not expand into millions of results
	if rec := post("[" + strings.Repeat("{},", maxRecords) + "{}]"); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too many records: code = %d", rec.Code)
	}
}

// A client that sends the body a byte now and then is cut off instead of holding the connection for minutes.
func TestParseBatchSlowBody(t *testing.T) {
	defer func(g time.Duration) { bodyGrace = g }(bodyGrace)
	bodyGrace = 200 * time.Millisecond
	srv := httptest.NewServer(New(fakeDB{}, "test", slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer srv.Close()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	start := time.Now()
	fmt.Fprint(conn, "POST /api/v1/parse/batch HTTP/1.1\r\nHost: x\r\nContent-Length: 100000\r\n\r\n[")
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("no response after %v: %v", time.Since(start), err)
	}
	if res.StatusCode != http.StatusBadRequest || time.Since(start) > 2*time.Second {
		t.Fatalf("status %d after %v", res.StatusCode, time.Since(start))
	}
}

func TestParseOne(t *testing.T) {
	h := New(fakeDB{}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		strings.NewReader(`{"raw_address":"Туран 55/6 магазин береке 4подьезд 9этаж"}`)))
	var resp struct {
		Result  map[string]any    `json:"result"`
		Spans   []map[string]any  `json:"spans"`
		Sources map[string]string `json:"sources"`
		Status  string            `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result["house"] != "55/6" || resp.Result["landmark"] != "магазин береке" || len(resp.Spans) == 0 ||
		resp.Sources["house"] != "string" || resp.Sources["district"] != "absent" || resp.Status == "" {
		t.Fatalf("resp = %+v", resp)
	}
}
