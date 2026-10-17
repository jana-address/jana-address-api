package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
