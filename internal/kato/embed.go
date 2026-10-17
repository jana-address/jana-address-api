package kato

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
)

// The snapshot is regenerated with `go run ./cmd/kato fetch`.
//
//go:embed snapshot/kato.csv.gz snapshot/manifest.json
var snapshotFS embed.FS

type embedded struct {
	recs []Record
	man  Manifest
}

var loadEmbedded = sync.OnceValues(func() (embedded, error) {
	var e embedded
	raw, err := snapshotFS.ReadFile("snapshot/" + ManifestFile)
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(raw, &e.man); err != nil {
		return e, fmt.Errorf("kato: embedded manifest: %w", err)
	}
	raw, err = snapshotFS.ReadFile("snapshot/" + SnapshotFile)
	if err != nil {
		return e, err
	}
	if e.recs, err = ReadSnapshot(bytes.NewReader(raw)); err != nil {
		return e, err
	}
	if len(e.recs) != e.man.Rows {
		return e, fmt.Errorf("kato: embedded snapshot has %d rows, manifest says %d", len(e.recs), e.man.Rows)
	}
	return e, nil
})

// Embedded returns the classifier snapshot compiled into the binary. The
// slice is a copy and may be modified by the caller.
func Embedded() ([]Record, Manifest, error) {
	e, err := loadEmbedded()
	if err != nil {
		return nil, Manifest{}, err
	}
	return slices.Clone(e.recs), e.man, nil
}
