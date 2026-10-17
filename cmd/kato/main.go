// Command kato refreshes the embedded KATO snapshot in internal/kato/snapshot.
//
//	kato fetch [-page URL] [-out DIR] [-force]
//	kato import -xlsx FILE -src URL [-out DIR] [-force]
//
// fetch downloads the current workbook from stat.gov.kz; import uses a local
// copy. Both parse, validate and write kato.csv.gz and manifest.json. The
// row count may differ from the previous snapshot by at most 15% unless
// -force is given.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/jana-address/jana-address-api/internal/kato"
)

const defaultOut = "internal/kato/snapshot"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "fetch":
		err = fetch(os.Args[2:])
	case "import":
		err = importFile(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kato:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: kato fetch [-page URL] [-out DIR] [-force]\n       kato import -xlsx FILE -src URL [-out DIR] [-force]")
	os.Exit(2)
}

func fetch(args []string) error {
	fl := flag.NewFlagSet("fetch", flag.ExitOnError)
	page := fl.String("page", kato.DefaultPageURL, "classifier page that links the KATO workbook")
	out := fl.String("out", defaultOut, "snapshot directory")
	force := fl.Bool("force", false, "accept any change in the row count")
	timeout := fl.Duration("timeout", 5*time.Minute, "download timeout")
	fl.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	data, src, err := kato.Fetch(ctx, &http.Client{Timeout: *timeout}, *page)
	if err != nil {
		return err
	}
	fmt.Printf("downloaded %s (%d bytes)\n", src, len(data))
	return build(data, src, *out, *force)
}

func importFile(args []string) error {
	fl := flag.NewFlagSet("import", flag.ExitOnError)
	file := fl.String("xlsx", "", "KATO workbook (required)")
	src := fl.String("src", "", "URL the workbook was downloaded from (required)")
	out := fl.String("out", defaultOut, "snapshot directory")
	force := fl.Bool("force", false, "accept any change in the row count")
	fl.Parse(args)
	if *file == "" || *src == "" {
		fl.Usage()
		os.Exit(2)
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	return build(data, *src, *out, *force)
}

func build(data []byte, src, out string, force bool) error {
	recs, err := kato.ParseXLSX(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	prev, err := readManifest(filepath.Join(out, kato.ManifestFile))
	if err != nil {
		return err
	}
	prevRows := prev.Rows
	if force {
		prevRows = 0
	}
	if err := kato.Validate(recs, prevRows); err != nil {
		return err
	}

	var snap bytes.Buffer
	if err := kato.WriteSnapshot(&snap, recs); err != nil {
		return err
	}
	man := kato.NewManifest(src, data, len(recs), time.Now())
	if man.SourceDate == "" {
		fmt.Fprintln(os.Stderr, "warning: no date in the source file name")
	}
	old, _ := os.ReadFile(filepath.Join(out, kato.SnapshotFile))
	if man.SHA256 == prev.SHA256 && man.SourceURL == prev.SourceURL && bytes.Equal(old, snap.Bytes()) {
		fmt.Printf("unchanged: %d rows from %s\n", len(recs), man.SourceDate)
		return nil
	}

	mj, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(out, kato.SnapshotFile), snap.Bytes()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(out, kato.ManifestFile), append(mj, '\n')); err != nil {
		return err
	}
	fmt.Printf("wrote %s: %d rows (previous %d), source date %s, sha256 %s\n",
		out, len(recs), prev.Rows, man.SourceDate, man.SHA256)
	fmt.Println(kindStats(recs))
	return nil
}

// readManifest returns a zero Manifest if the file does not exist yet.
func readManifest(name string) (kato.Manifest, error) {
	var m kato.Manifest
	b, err := os.ReadFile(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return m, nil
	case err != nil:
		return m, err
	case len(b) == 0:
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("%s: %w", name, err)
	}
	return m, nil
}

// writeFile replaces name atomically.
func writeFile(name string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".kato-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), name)
}

func kindStats(recs []kato.Record) string {
	n := map[kato.Kind]int{}
	for _, r := range recs {
		n[r.Kind]++
	}
	var b strings.Builder
	for i, k := range kato.Kinds {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s %d", k, n[k])
	}
	return b.String()
}
