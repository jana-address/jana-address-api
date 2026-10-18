# How parsing works

The parser uses rules and reference data. The result is deterministic and explainable, and
every field is marked with its source: "from text" or "directory" in the UI (in Russian
«из строки» or «справочник»). The code is in `internal/parser`.

## 1. Cleanup

Invisible characters, NBSP, different dashes and quotes are normalised. Positions in the
original line are kept for highlighting.

## 2. Tokens

Glued words are split: «66квартал», «Назарбаева8», «кв5под2», «АлматыАбая10»,
«астанакалас…». Fractions and letters are kept: «55/6», «208 / 2» → «208/2», «12 а» → «12а».

## 3. Markers

Markers in Russian, Kazakh and Latin script, with typos: кв/пәтер/kv/кваритра,
подъезд/кіреберіс/подлъезд, ул/көшесі, мкр/шағын аудан/микрарайон, қаласы, ауданы…

## 4. City, district, province

- by markers and the reference data;
- bilingual lines with «/», a repeated city, a district instead of a city
  («Турксибский район» → Алматы);
- typos in names («алматыы», «гролд актобе»).

## 5. Numbers to markers

Glued forms first, then neighbouring tokens. The writing direction («кв 5» or «5 кв») is
detected along the chain.

## 6. Landmark, street, living area

- a street with or without a type;
- the name is taken verbatim, as in the line;
- a microdistrict or residential complex (ЖК) with a separately written street goes to
  `living_area`, also without the word «мкр» («Караганда, Пришахтинск, ул. Ленина 5»);
- a microdistrict and a street without a separator are split by the city's street directory
  if it knows both parts: «Павлодар мкр Достык Академика Маргулана 47» → `living_area`
  «Достык», street «Академика Маргулана». One typo in one of the parts is allowed if the city
  has no second name that close; a street is also recognised by the surname alone
  («Бектурова» — «Академика Бектурова»);
- a name before the marker («Самал шағын ауданы», «Хайвил жк Сыганак 18», «32 мкр Абылай
  хан көшесі 48») and a name in quotes («ЖК «Асыл Тау» Маметовой 18») also go to
  `living_area`, and the words after them go to the street;
- if a microdistrict is followed by a street with the type at the end («мкр Коктем Кабанбай
  батыра улица»), the reference data sets the boundary. When it knows neither part, the
  microdistrict is one word, and the street before a ЖК is one word, or two if the second is a
  title («батыра», «би», «хана»).

## 7. Restoration from the reference data

Only empty fields are filled; what is written in the line is never changed.

- province by city; province and district by village (KATO);
- city and province by district; city by living area («Пришахтинск» → Караганда);
- the district of Алматы, Астана, Шымкент and Тараз by house, by neighbouring numbers, by
  street or by microdistrict (OpenStreetMap, see
  [reference-data.md](reference-data.md#street-directory-openstreetmap));
- the street type, if all streets with this name in the city have the same type
  («Сатпаева» → улица).

In the `POST /api/v1/parse` response, every restored field has a `levels` entry that says
how it was found: `house`, `neighbors`, `street`, `area`, `settlement` or `district`. The UI
shows it next to the "directory" badge.

## 8. Confidence

| `confidence` | Status | UI label (en / ru) |
|---|---|---|
| ≥ 0.8 | `confident` | Confident / «уверенно» |
| 0.5–0.8 | `review` | Review / «проверить» |
| < 0.5 | `unparsed` | Unparsed / «не разобрано» |

## Villages from KATO

The parser takes a village from KATO only where it is certainly a settlement: after «с.»,
«п.», «ауылы», «г.», or as a separate part next to a written district or province. Hundreds
of villages are called «Абай», «Достык», «Береке», like streets, so a word before a house
number is not taken as a village. If several villages share a name, the written district or
province decides; if neither is written, the larger settlement from a manual table is taken.

See also: [reference-data.md](reference-data.md) for the reference data and
[api.md](api.md#output-fields) for the field rules.
