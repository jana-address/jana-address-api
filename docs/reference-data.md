# Reference data

All reference data is built into the binary. The parser does not need a network or the
database to use it. Sources and licences: [ATTRIBUTION](../ATTRIBUTION).

## KATO

KATO (the classifier of administrative-territorial objects, stat.gov.kz, version 18.09.2026)
has 15 586 records: 20 regions, 236 districts and city administrations, 87 cities and 9 645
villages, with names in Russian and Kazakh. With it the parser recognises cities and villages,
rural districts and the districts of Актобе and Караганда, and restores the province and
district of a settlement.

- **Snapshot in the image.** `internal/kato/snapshot/` (`kato.csv.gz` + `manifest.json` with
  the sha256 of the source xlsx and the date) is embedded in the binary. The API parses
  addresses with it right after start, without waiting for the database or the network.
- **Versions in the database, refresh, migrations:** see
  [operations.md](operations.md#kato-in-the-database-and-migrations).
- **Update the snapshot in the repository:** `go run ./cmd/kato fetch`.

How the parser uses KATO villages: [parser.md](parser.md#villages-from-kato).

## Street directory (OpenStreetMap)

KATO has no streets, so the district of a city address with no district in the line comes
from OpenStreetMap. The directory covers 21 districts of Алматы, Астана, Шымкент and Тараз,
and 43 more cities with their own akimat (Караганда, Актобе, Павлодар and others; the list is
in `cmd/osmref/main.go`). These cities are taken whole, without division into districts. They
give no district, but their streets and microdistricts split «мкр Достык Академика Маргулана
47» into a microdistrict and a street and give the street type. Rural districts are not
taken: their streets belong to many villages, and without coordinates the village cannot be
determined.

The directory is in `internal/parser/dict/osm/` (`streets.tsv.gz`, `houses.tsv.gz`,
`areas.tsv.gz`, `manifest.json`). It weighs 1.0 MB (55 678 streets, 196 887 houses on streets
that cross district borders, 2 687 ЖК and microdistricts; OSM export of 17–18.10.2026) and is
embedded in the binary. It needs no network, and loading it at start takes ~85 ms.

### How it is built

The build is reproducible (`cmd/osmref`):

```bash
go run ./cmd/osmref fetch   # Overpass API: for each district or city polygon, the streets, the houses with an address, the ЖК and microdistricts
go run ./cmd/osmref build   # data/raw/osm/*.json -> internal/parser/dict/osm/
```

- **Votes.** Every street segment and every building with an address votes for the district
  whose polygon it lies in.
- **One street under different names.** The Russian and Kazakh names of one road («проспект
  Абая» and «Абай даңғылы») are merged into one street. «Улица Абая» and «проспект Абая» are
  two different streets, as the case brief requires. The key is built by the function
  `dict.StreetKey`, shared by the build and the parser: the street type is removed, letters are
  folded, initials are dropped. A corner building with two streets in one tag («улица
  Комиссарова;проспект Бухар-жырау», number «37;66») does not merge them: they are different
  streets, and each gets its own number.

### How the parser finds the district

In order of accuracy:

1. **house** (`house`): the number is in OSM for a street that crosses several districts;
2. **neighbours** (`neighbors`): the house is not in OSM, but the nearest numbers on the same
   side of the street (same parity) lie in one district — by the majority of buildings with
   that number. «Розыбакиева 241» stands between 237 and 247 of Бостандыкский, «Розыбакиева 35»
   between 33 and 37 of Алмалинский (of the three buildings numbered 37, one, 37а deep inside
   the block, is in Бостандыкский). The other side is not counted: a street itself is often
   the border;
3. **street** (`street`): at least 75% of the street's votes are for one district. A street
   split evenly between two districts gives no district;
4. **area** (`area`): a microdistrict or ЖК from OSM. The fallback is a manual table of
   Алматы microdistricts.

### Holdout check

During the build every tenth building (by hash) is held out. Its district is restored the
way the parser does it for a house that is not in the directory: by neighbouring numbers
(without buildings with the same number; «17а» for «17» is removed too), otherwise by street:

| City | Houses held out | District restored | Correct |
|---|---|---|---|
| Алматы | 12 857 | 97.0% | 98.9% |
| Астана | 2 997 | 98.4% | 99.1% |
| Шымкент | 9 216 | 94.6% | 97.4% |
| Тараз | 4 428 | 98.8% | 99.5% |

The rest are houses between numbers of different districts or at the end of a street; no
district is set there. If the house is in OSM, the parser takes its district exactly
(`house`).

The numbers are recomputed by every build and written to `manifest.json`; `GET
/api/v1/health` returns them in the `streets` field.

**Licence.** Data © OpenStreetMap contributors, ODbL 1.0 (file
[ATTRIBUTION](../ATTRIBUTION)).

## Districts of Караганда and Актобе (polling precincts)

The districts of Караганда (Казыбек би and Әлихан Бөкейхан) are not drawn in OSM. There the
district comes from the street, or, if there is no street, from the microdistrict, using the
decision of the akim of Караганда on polling precincts (№ 12/01 of 19.12.2018 as amended by
№ 03 of 14.07.2025, https://zakon.uchet.kz/rus/docs/G25KA00003B): annex 1 is the district
Казыбек би, annex 2 is Әлихан Бөкейхан. About 1 500 street and microdistrict names in Russian
and Kazakh were collected from it (`internal/parser/dict/precincts/karaganda.tsv`); a street
that exists in both districts gives no district. A district written in the line is not
replaced.

The districts of Актобе (Алматы and Астана) work the same way: the decision of the akim of
Актобе on polling precincts (№ 7 of 25.10.2019 as amended by № 4 of 28.05.2026,
https://zakon.uchet.kz/rus/docs/V19CA006434), about 1 200 names in
`internal/parser/dict/precincts/aktobe.tsv`; streets that exist in both districts (Абая, Мира,
Сатпаева and others) give no district.
