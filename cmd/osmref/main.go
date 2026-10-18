// Command osmref builds the street and living-area directory of the cities in
// internal/parser/dict/osm from OpenStreetMap: by district for Алматы, Астана, Шымкент and Тараз,
// whole for every other city with its own administration.
//
//	osmref fetch [-raw DIR] [-api URL]
//	osmref build [-raw DIR] [-out DIR]
//
// fetch asks the Overpass API for the named streets, the addressed buildings and the
// named living areas inside every district polygon of Алматы, Астана, Шымкент and Тараз, and
// inside the boundary of every other city with its own administration, and stores the
// answers in -raw, one file per area. build reads them and writes
// streets.tsv.gz, houses.tsv.gz, areas.tsv.gz and manifest.json; the same raw files give
// the same output. A whole city's rows have an empty district: they give the parser the
// street names, types and living areas of the city, not a district. Only build output is
// committed, the raw answers stay in data/raw.
package main

import (
	"fmt"
	"os"
)

const (
	defaultRaw = "data/raw/osm"
	defaultOut = "internal/parser/dict/osm"
)

// districts are the OSM boundary relations (admin_level 6, 7 for Тараз) of the city districts, with the
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
	{18817641, "Тараз", "Әулиеата"}, {18817642, "Тараз", "Жібек жолы"},
}

// cities are the other cities with their own administration ("қалалық әкімдігі", "Г.А."), with the
// names the parser outputs. They are taken whole, without districts: their streets and living areas
// let the parser tell "мкр Достык Академика Маргулана 47" apart. A rural district is not taken:
// its streets belong to many villages, and without coordinates no village can be told.
var cities = []struct {
	Rel  int64
	City string
}{
	{14210662, "Актобе"}, {2068931, "Аркалык"}, {3407442, "Арысь"}, {5501718, "Аксу"}, {16686703, "Аксай"},
	{5515125, "Актау"}, {3389541, "Атырау"}, {5511640, "Абай"}, {5295657, "Байконыр"}, {5513067, "Балхаш"},
	{5515236, "Жанаозен"}, {5513216, "Жезказган"}, {5524713, "Житикара"}, {5512171, "Каражал"}, {3386005, "Караганда"},
	{3390993, "Каскелен"}, {17322798, "Кентау"}, {3388744, "Кокшетау"}, {5518065, "Конаев"}, {3338024, "Костанай"},
	{16717004, "Кульсары"}, {5499667, "Курчатов"}, {3396852, "Кызылорда"}, {2914970, "Лисаковск"}, {5501700, "Павлодар"},
	{2313542, "Петропавловск"}, {2456975, "Приозерск"}, {3389550, "Риддер"}, {2068946, "Рудный"}, {5511047, "Сарань"},
	{5513234, "Сатпаев"}, {17351528, "Семей"}, {3404209, "Степногорск"}, {15405616, "Талгар"}, {3392135, "Талдыкорган"},
	{5518135, "Текели"}, {5511190, "Темиртау"}, {5496366, "Туркестан"}, {3767383, "Уральск"},
	{3389569, "Усть-Каменогорск"}, {5511125, "Шахтинск"}, {21176885, "Щучинск"}, {5502043, "Экибастуз"},
}

// unit is one area to fetch: a city district, or a whole city with an empty District.
type unit struct {
	Rel            int64
	City, District string
}

// units lists the city districts first, then the whole cities.
func units() []unit {
	var out []unit
	for _, d := range districts {
		out = append(out, unit{d.Rel, d.City, d.District})
	}
	for _, c := range cities {
		out = append(out, unit{c.Rel, c.City, ""})
	}
	return out
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
