# Jana Address

Address parsing for Jana Post: Go API + React UI + PostgreSQL. Terricon Valley hackathon, case 1.

A customer writes an address in one line, in any form: «66квартал 12дом 69квартира»,
«Туран 55/6 магазин береке 4подьезд 9этаж». The service splits it into 14 JSON fields:
province, city, district, living area, street type and name, house, apartment, entrance,
floor and landmark. It fills missing fields from the reference data and marks records
for manual review.

- **REST API** on http://localhost:8080, with the batch method `POST /api/v1/parse/batch`.
- **Operator UI** on http://localhost:3000: check one line with highlighting, upload a file,
  correct uncertain records by hand, download the result.
- **Works offline.** The reference data is built into the image; no network is needed at runtime.

## Quick start

Requirements: Docker Engine ≥ 24 and Docker Compose ≥ 2.20.

```bash
git clone --recurse-submodules https://github.com/jana-address/jana-address-api.git
cd jana-address-api
docker compose up
```

- UI: http://localhost:3000
- API: http://localhost:8080
- Health check: `curl http://localhost:8080/api/v1/health` returns
  `{"status":"ok","version":"0.1.0","db":"ok","directory":{…KATO version…}}`.

The frontend lives in the separate repository `jana-address-web` and is a submodule in
`web/`. Clone with `--recurse-submodules`; the GitHub "Download ZIP" button does not work,
because the archive does not include the submodule.

A `.env` file is optional: every variable has a default. To change the ports:
`WEB_PORT=3001 API_PORT=8081 docker compose up` (see `.env.example`).

The network is needed only for the first image build (`docker compose build`). After
that, `docker compose up` works offline.

## API example

```bash
curl -s -XPOST http://localhost:8080/api/v1/parse/batch \
  -H 'Content-Type: application/json' \
  --data-binary '[{"id": "rec_00417", "raw_address": "город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70"}]'
```

```json
[
  {
    "id": "rec_00417",
    "raw_address": "город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70",
    "province": "Алматы",
    "city": "Алматы",
    "district": "Ауэзовский",
    "living_area": null,
    "street_type": "микрорайон",
    "street_name": "Аксай 3а",
    "house": "59",
    "apartment": "70",
    "entrance": null,
    "floor": null,
    "landmark": null,
    "confidence": 0.95
  }
]
```

The output has the same length and order as the input, and every record has exactly 14 keys.
For a whole file: `curl -s -XPOST http://localhost:8080/api/v1/parse/batch --data-binary @sample_input.json -o result.json`.

- OpenAPI spec: `GET /api/v1/openapi.yaml` (source: [internal/httpapi/openapi.yaml](internal/httpapi/openapi.yaml)).
- API reference page in the UI: http://localhost:3000/docs.
- Formats, limits and errors: [docs/api.md](docs/api.md).

## Quality and speed

| Set | Total | Address (60%) | Details (25%) | Reference (15%) |
|---|---|---|---|---|
| fasttype, 100 lines labelled by hand | 1.000 | 1.000 | 1.000 | 1.000 |
| holdout, 100 lines labelled by hand | 1.000 | 1.000 | 1.000 | 1.000 |
| synthetic, 10 000 lines | 0.999 | 0.999 | 1.000 | 0.996 |

The team labelled the manual sets, and the synthetic set uses the same lexicon as the
parser, so these numbers are an upper bound; the hidden jury set will be stricter.
Speed: 100 000 records in one request ≈ 0.5 s, 1 000 000 ≈ 5 s on a 32-thread machine; one address takes about 45 µs on one core (the case limit is 5 minutes per 1000).
Method and commands: [docs/quality.md](docs/quality.md).

## Documentation

| File | Contents |
|---|---|
| [docs/api.md](docs/api.md) | Endpoints, input and output formats, field rules, limits, errors |
| [docs/parser.md](docs/parser.md) | How parsing works, step by step; confidence; villages from KATO |
| [docs/reference-data.md](docs/reference-data.md) | KATO, the OpenStreetMap street directory, the Караганда and Актобе precinct tables, holdout check |
| [docs/quality.md](docs/quality.md) | Evaluation method, metric table, synthetic generator, how to run the evaluation |
| [docs/architecture.md](docs/architecture.md) | Services, images, security, start order, component versions |
| [docs/operations.md](docs/operations.md) | Offline mode, KATO refresh, migrations, environment variables, health checks, troubleshooting |
| [docs/development.md](docs/development.md) | Tests, CI, tools (`osmref`, `kato`, `synth`), working with the submodule |
| [jana-address-web](https://github.com/jana-address/jana-address-web) (`web/README.md`) | The UI: screens, file formats, running and building |

## Licence and data

The reference data in the image comes from KATO (stat.gov.kz), OpenStreetMap
(© OpenStreetMap contributors, ODbL 1.0) and the akimat decisions on polling precincts of
Караганда and Актобе. Sources and terms: [ATTRIBUTION](ATTRIBUTION).

The organizers' files `train.json`, `sample_input.json` and `sample_output.json` are not in
the repository; see [docs/operations.md](docs/operations.md#organizers-data).
