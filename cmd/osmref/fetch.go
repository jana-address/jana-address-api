package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const defaultAPI = "https://overpass-api.de/api/interpreter"

// query returns tags only, no geometry: the district is known from the area the element is in.
// An element on a district border appears in both districts, which only lowers the street share.
const query = `[out:json][timeout:600];
rel(%d);map_to_area->.d;
way(area.d)["highway"]["name"];
out tags;
nwr(area.d)["addr:street"]["addr:housenumber"];
out tags;
(nwr(area.d)["place"~"^(suburb|neighbourhood|quarter|city_block|isolated_dwelling)$"]["name"];
 nwr(area.d)["landuse"="residential"]["name"];);
out tags;`

func fetch(args []string) error {
	fl := flag.NewFlagSet("fetch", flag.ExitOnError)
	raw := fl.String("raw", defaultRaw, "directory for the Overpass answers")
	api := fl.String("api", defaultAPI, "Overpass API interpreter URL")
	fl.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := os.MkdirAll(*raw, 0o755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 15 * time.Minute}
	for _, d := range districts {
		path := filepath.Join(*raw, fmt.Sprintf("%d.json", d.Rel))
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "%s %s: have %s\n", d.City, d.District, path)
			continue
		}
		var body []byte
		var err error
		for attempt := 1; attempt <= 4; attempt++ {
			body, err = post(ctx, client, *api, fmt.Sprintf(query, d.Rel))
			if err == nil {
				break
			}
			fmt.Fprintf(os.Stderr, "%s %s: attempt %d: %v\n", d.City, d.District, attempt, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt*30) * time.Second):
			}
		}
		if err != nil {
			return fmt.Errorf("%s %s: %w", d.City, d.District, err)
		}
		var ans answer
		if err := json.Unmarshal(body, &ans); err != nil {
			return fmt.Errorf("%s %s: bad answer: %w", d.City, d.District, err)
		}
		if ans.Remark != "" {
			return fmt.Errorf("%s %s: overpass: %s", d.City, d.District, ans.Remark)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "%s %s: %d elements, %s\n", d.City, d.District, len(ans.Elements), ans.OSM3S.Timestamp)
		time.Sleep(5 * time.Second) // be polite to the public instance
	}
	return nil
}

func post(ctx context.Context, client *http.Client, api, q string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api, strings.NewReader(url.Values{"data": {q}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "jana-address-osmref/1.0 (+https://github.com/jana-address)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %.200s", resp.StatusCode, body)
	}
	return body, nil
}

// answer is the part of an Overpass JSON answer the tool reads.
type answer struct {
	OSM3S struct {
		Timestamp string `json:"timestamp_osm_base"`
	} `json:"osm3s"`
	Remark   string    `json:"remark"`
	Elements []element `json:"elements"`
}

type element struct {
	Type string            `json:"type"`
	ID   int64             `json:"id"`
	Tags map[string]string `json:"tags"`
}
