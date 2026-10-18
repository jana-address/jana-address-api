# API

The full spec is `GET /api/v1/openapi.yaml` (source:
[internal/httpapi/openapi.yaml](../internal/httpapi/openapi.yaml)). The UI renders it at
http://localhost:3000/docs.

## Endpoints

| Method | Path | Response |
|---|---|---|
| GET | `/livez` | `ok` if the process is alive |
| GET | `/readyz` | 200 when the reference data is loaded and the API can parse addresses; the database is not needed for this |
| GET | `/api/v1/health` | `{"status":"ok","version":"0.1.0","db":"ok","directory":{…KATO version…}}` |
| POST | `/api/v1/parse/batch` | the method for the jury: an array of `{id, raw_address}` → an array of 14 fields in the order of the case brief |
| POST | `/api/v1/parse` | one line for the UI: fields, highlighting, sources, suggestion |
| GET | `/api/v1/openapi.yaml` | the OpenAPI spec |

`/api/v1/health` also returns the `streets` field: the OpenStreetMap street directory
summary with the holdout numbers (see [reference-data.md](reference-data.md#holdout-check)).

## Batch parsing: running the test set

```bash
docker compose up -d --wait
curl -s -XPOST http://localhost:8080/api/v1/parse/batch \
  --data-binary @sample_input.json -o result.json
```

- **Input:** a JSON array `[{"id": "...", "raw_address": "..."}]`, like `sample_input.json`.
  NDJSON and `multipart/form-data` with the field `file` are also accepted. The body is at most
  128 MiB and 1 000 000 records; broken JSON gets 400.
- **Output:** an array of the same length and in the same order. Every record has exactly 14
  keys in the order of the case brief (`id`, `raw_address`, `province`, `city`, `district`,
  `living_area`, `street_type`, `street_name`, `house`, `apartment`, `entrance`, `floor`,
  `landmark`, `confidence`).
  - `raw_address` and `id` are returned unchanged;
  - an unknown field is `null`;
  - a record that could not be parsed gets `null` fields and `confidence: 0`; no record is dropped.
- **Speed:** 10 000 records ≈ 60 ms on a laptop (the case limit is 5 minutes per 1000). No
  network is needed.
- **No cache:** the answer is not looked up by `id` or by the whole line; every record is
  parsed again.
- **Your own set of 10 000 lines** is built from the same image; Go is not needed. The file
  also works for upload in the UI:

  ```bash
  docker compose run --rm --no-deps -T api synth > testdata/synth/synth_10000_input.json
  ```

The spec adds details: the batch method also accepts an object with the array in
`items`/`records`/`data`/`addresses`, and a UTF-8 BOM is allowed. The response carries the
`X-Records` header with the number of records. More than 1 000 000 records or a body larger
than 128 MiB gets 413. Errors are JSON: `{"error": "..."}`.

## Output fields

| Field | Rule |
|---|---|
| `id` | as in the input, any JSON value |
| `raw_address` | as in the input, byte for byte |
| `province` | province in direct word order, or a city of republican significance: «Актюбинская область», «Алматы» |
| `city` | name only: «Актобе», «Боралдай» |
| `district` | without the word «район»: «Илийский», «Ауэзовский» |
| `living_area` | microdistrict or residential complex (ЖК), when the street is given separately |
| `street_type` | one of `улица`, `проспект`, `микрорайон`, `переулок`, `квартал`, `бульвар`, `шоссе`, `жилой комплекс`, or `null` |
| `street_name` | without the type, exactly as in the line |
| `house` | with a fraction and a letter: «1/2», «12а» |
| `apartment`, `entrance`, `floor`, `landmark` | as found in the line |
| `confidence` | 0 to 1: ≥ 0.8 confident, 0.5–0.8 review, < 0.5 unparsed |

## One line for the UI

`POST /api/v1/parse` takes `{"raw_address": "..."}` (missing `raw_address` gets 400) and
returns:

- `result` — the 14 fields above;
- `spans` — fragments of the input line per field, offsets in UTF-16;
- `sources` — where each field comes from: `string`, `directory` or `absent`;
- `levels` — for each field restored from the reference data, how it was found: `house`,
  `neighbors`, `street`, `area`, `settlement` or `district` (see
  [parser.md](parser.md#7-restoration-from-the-reference-data));
- `status` — `confident`, `review` or `unparsed`;
- `suggestion` — the clean address for the hint under the input field.
