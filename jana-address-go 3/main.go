package main

import (
	_ "embed"
	"encoding/json"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
)

//go:embed web/index.html
var indexHTML []byte

const (
	maxRecords = 10000
	maxBody    = 32 << 20 // 32 МБ
)

// Record — одна запись входного файла. Адрес берётся из raw, address или text.
type Record struct {
	ID      string `json:"id"`
	Raw     string `json:"raw"`
	Address string `json:"address"`
	Text    string `json:"text"`
}

func (r Record) line() string {
	for _, v := range []string{r.Raw, r.Address, r.Text} {
		if v != "" {
			return v
		}
	}
	return ""
}

type Item struct {
	ID  string `json:"id"`
	Raw string `json:"raw"`
	Result
}

type Summary struct {
	Total     int     `json:"total"`
	Confident int     `json:"confident"`
	Review    int     `json:"review"`
	Unparsed  int     `json:"unparsed"`
	Avg       float64 `json:"avg"`
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": "0.1.0"})
	})
	mux.HandleFunc("POST /api/v1/parse", handleParse)
	mux.HandleFunc("POST /api/v1/parse/batch", handleBatch)

	addr := ":" + envOr("PORT", "8080")
	log.Println("Jana Address: http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleParse(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "не удалось прочитать тело запроса")
		return
	}
	var in Record
	if err := json.Unmarshal(body, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "ожидается JSON вида {\"raw\": \"...\"}")
		return
	}
	writeJSON(w, http.StatusOK, Parse(in.line()))
}

func handleBatch(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(w, r)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "файл слишком большой (максимум 32 МБ)")
		return
	}
	// допускаем и массив [...], и объект {"records": [...]}
	var recs []Record
	if err := json.Unmarshal(body, &recs); err != nil {
		var obj struct {
			Records []Record `json:"records"`
		}
		if err := json.Unmarshal(body, &obj); err != nil {
			writeErr(w, http.StatusBadRequest, "неверный JSON: ожидается массив записей или {\"records\": [...]}")
			return
		}
		recs = obj.Records
	}
	if len(recs) == 0 {
		writeErr(w, http.StatusBadRequest, "нет записей")
		return
	}
	if len(recs) > maxRecords {
		writeErr(w, http.StatusRequestEntityTooLarge, "максимум 10 000 записей за один запрос")
		return
	}

	items := make([]Item, 0, len(recs))
	sum := Summary{Total: len(recs)}
	var total float64
	for i, rec := range recs {
		id := rec.ID
		if id == "" {
			id = idFor(i)
		}
		res := Parse(rec.line())
		items = append(items, Item{ID: id, Raw: rec.line(), Result: res})
		total += res.Confidence
		switch res.Status {
		case "confident":
			sum.Confident++
		case "unparsed":
			sum.Unparsed++
		default:
			sum.Review++
		}
	}
	sum.Avg = math.Round(total/float64(len(recs))*100) / 100
	writeJSON(w, http.StatusOK, map[string]any{"results": items, "summary": sum})
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	return io.ReadAll(r.Body)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
