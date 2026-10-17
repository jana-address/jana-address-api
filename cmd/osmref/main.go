// Command osmref builds the street and living-area directory of the city districts in
// internal/parser/dict/osm from OpenStreetMap.
//
//	osmref fetch [-raw DIR] [-api URL]
//	osmref build [-raw DIR] [-out DIR]
//
// fetch asks the Overpass API for the named streets, the addressed buildings and the
// named living areas inside every district polygon of Алматы, Астана and Шымкент and
// stores the answers in -raw, one file per district. build reads them and writes
// streets.tsv.gz, houses.tsv.gz, areas.tsv.gz and manifest.json; the same raw files give
// the same output. Only build output is committed, the raw answers stay in data/raw.
package main

import (
	"fmt"
	"os"
)

const (
	defaultRaw = "data/raw/osm"
	defaultOut = "internal/parser/dict/osm"
)

// districts are the OSM boundary relations (admin_level 6) of the city districts, with the
// district names the parser outputs.
var districts = []struct {
	Rel      int64
	City     string
	District string
}{
	{3072216, "Алматы", "Алатауский"}, {3072807, "Алматы", "Алмалинский"}, {3072808, "Алматы", "Ауэзовский"},
	{3390291, "Алматы", "Бостандыкский"}, {3072130, "Алматы", "Жетысуский"}, {3072217, "Алматы", "Медеуский"},
	{5460063, "Алматы", "Наурызбайский"}, {3072001, "Алматы", "Турксибский"},
	{3482819, "Астана", "Алматы"}, {8593081, "Астана", "Байконур"}, {3479876, "Астана", "Есиль"},
	{20593940, "Астана", "Нура"}, {19733918, "Астана", "Сарайшык"}, {3486954, "Астана", "Сарыарка"},
	{5548210, "Шымкент", "Абайский"}, {5550509, "Шымкент", "Аль-Фарабийский"}, {5551117, "Шымкент", "Енбекшинский"},
	{5551119, "Шымкент", "Каратауский"}, {21588024, "Шымкент", "Туран"},
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "fetch":
		err = fetch(os.Args[2:])
	case "build":
		err = build(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "osmref:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: osmref fetch [-raw DIR] [-api URL]\n       osmref build [-raw DIR] [-out DIR]")
	os.Exit(2)
}
