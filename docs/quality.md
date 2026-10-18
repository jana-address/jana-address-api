# Quality

## Method

- **Unit tests:** all examples from the case brief and hundreds of input forms
  (`internal/parser`).
- **Evaluator** that follows the metric rules of the case brief (`internal/eval`): lower case,
  no punctuation, weights 60/25/15, reference fields only where they are present in the line.
  - Two sets of 100 lines each are labelled by hand.
  - The fast set generator `cmd/synth`: typos, glued words, Latin script, Kazakh, ЖК,
    villages. The set is deterministic for a seed: the test generates 10 000 lines (seed 1)
    on the fly; they are not stored in the repository. CI keeps the metric from falling below
    the baseline, and the hash `synth_10000.sha256` keeps the set from changing silently
    together with the generator.
  - The generator does not produce forms whose fields cannot be read from the line itself: a
    typo at the border of two glued words (in «алматыфтолеби» the extra letter may belong to the
    city or to the street) and a four-letter name cut to three letters («асу» for Аксу).

## Current numbers

From `go test ./internal/eval -run TestFastTypeSet -v`:

| Set | Total | Address (60%) | Details (25%) | Reference (15%) |
|---|---|---|---|---|
| fasttype, 100 lines labelled by hand | 1.000 | 1.000 | 1.000 | 1.000 |
| holdout, 100 lines labelled by hand | 1.000 | 1.000 | 1.000 | 1.000 |
| synthetic, 10 000 lines | 1.000 | 1.000 | 1.000 | 0.998 |

The team labelled the manual sets, and the synthetic set is built on the same lexicon as the
parser. So these numbers are an upper bound; the hidden jury set will be stricter. The
restoration of the district from the reference data is checked separately, on OSM houses
held out from the build (see [reference-data.md](reference-data.md#holdout-check)).

Speed: 100 000 records in one request ≈ 0.5 s, 1 000 000 ≈ 5 s on a 32-thread machine; one address takes about 45 µs on one core (the case limit is 5 minutes per 1000).

## Running the evaluation

```bash
go test ./internal/eval -run TestFastTypeSet -v                  # 100 + 100 + 10 000
go run ./cmd/synth -n 10000 -seed 1 -format jury -o testdata/synth/synth_10000_input.json  # file for the UI (without Go: server synth, see below)
go run ./cmd/synth -n 100000 -seed 2 -o testdata/synth/synth_100000.jsonl
EVAL_EXTRA=synth/synth_100000 go test ./internal/eval -run TestFastTypeSet -v
```

Without Go, the 10 000-line set is built from the same image; the file also works for upload
in the UI:

```bash
docker compose run --rm --no-deps -T api synth > testdata/synth/synth_10000_input.json
```
