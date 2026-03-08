# B-tree Benchmark Comparison

Comparative benchmarks across three B-tree implementations:

| Implementation | Package |
|---|---|
| ajwerner | `github.com/anacrolix/btree` |
| tidwall | `github.com/tidwall/btree v1.7.0` |
| google | `github.com/google/btree v1.1.3` |

## Benchmark groups

Benchmarks are organised into three provenance groups:

| Group | Sub-benchmarks | Source |
|---|---|---|
| `BenchmarkGoogle` | Insert, Seek, DeleteInsert, DeleteInsertCloneOnce, DeleteInsertCloneEachTime, Delete, Get, Ascend, Descend, AscendRange, DescendRange, AscendGreaterOrEqual, DescendLessOrEqual | Ported from `google/btree` test suite |
| `BenchmarkTidwall` | InsertSeq, GetSeq, InsertAfterClone, PivotAscend, PivotDescend | Ported from `tidwall/btree-benchmark` |
| `BenchmarkLocal` | Upsert, CursorSeek, CursorNext, CursorAscend | Original benchmarks for this module |

Each benchmark is named `Benchmark<Group>/<Op>/<impl>`, e.g. `BenchmarkGoogle/Insert/ajwerner`.

## Results

**[Interactive chart](https://htmlpreview.github.io/?https://github.com/anacrolix/btree/blob/main/bench/results/chart.html)** — runtime relative to ajwerner, plus B/op and allocs/op, one row per benchmark.

Latest results (`Apple M4 Max`, `go test -bench=. -benchmem -count=6`):

```
goos: darwin
goarch: arm64
pkg: github.com/anacrolix/btree/bench
cpu: Apple M4 Max
                                             │   sec/op    │
Google/Insert/ajwerner-16                      65.55n ± 0%
Google/Insert/tidwall-16                       53.25n ± 5%
Google/Insert/google-16                        68.99n ± 4%
Google/Seek/ajwerner-16                        51.66n ± 2%
Google/Seek/tidwall-16                         37.16n ± 2%
Google/Seek/google-16                          56.85n ± 1%
Google/DeleteInsert/ajwerner-16                161.2n ± 1%
Google/DeleteInsert/tidwall-16                 121.8n ± 2%
Google/DeleteInsert/google-16                  160.1n ± 0%
Google/DeleteInsertCloneOnce/ajwerner-16       149.4n ± 2%
Google/DeleteInsertCloneOnce/tidwall-16        124.0n ± 1%
Google/DeleteInsertCloneOnce/google-16         173.5n ± 2%
Google/DeleteInsertCloneEachTime/ajwerner-16   753.9n ± 1%
Google/DeleteInsertCloneEachTime/tidwall-16    174.8µ ± 1%
Google/DeleteInsertCloneEachTime/google-16     362.2µ ± 2%
Google/Delete/ajwerner-16                      77.77n ± 1%
Google/Delete/tidwall-16                       60.22n ± 3%
Google/Delete/google-16                        82.35n ± 2%
Google/Get/ajwerner-16                         67.13n ± 3%
Google/Get/tidwall-16                          59.79n ± 5%
Google/Get/google-16                           79.11n ± 2%
Google/Ascend/ajwerner-16                      25.68µ ± 1%
Google/Ascend/tidwall-16                       20.95µ ± 1%
Google/Ascend/google-16                        27.37µ ± 2%
Google/Descend/ajwerner-16                     23.61µ ± 1%
Google/Descend/tidwall-16                      20.48µ ± 1%
Google/Descend/google-16                       28.15µ ± 1%
Google/AscendRange/ajwerner-16                 25.17µ ± 1%
Google/AscendRange/tidwall-16                  21.05µ ± 1%
Google/AscendRange/google-16                   41.07µ ± 1%
Google/DescendRange/ajwerner-16                23.29µ ± 1%
Google/DescendRange/tidwall-16                 20.13µ ± 0%
Google/DescendRange/google-16                  53.26µ ± 1%
Google/AscendGreaterOrEqual/ajwerner-16        25.57µ ± 1%
Google/AscendGreaterOrEqual/tidwall-16         22.63µ ± 3%
Google/AscendGreaterOrEqual/google-16          30.15µ ± 0%
Google/DescendLessOrEqual/ajwerner-16          23.73µ ± 0%
Google/DescendLessOrEqual/tidwall-16           20.89µ ± 1%
Google/DescendLessOrEqual/google-16            41.84µ ± 2%
Local/Upsert/ajwerner-16                       66.03n ± 3%
Local/Upsert/tidwall-16                        108.4n ± 4%
Local/Upsert/google-16                         76.92n ± 1%
Local/CursorSeek/ajwerner-16                   96.20n ± 3%
Local/CursorSeek/tidwall-16                    601.5µ ± 2%
Local/CursorSeek/google-16                     483.0µ ± 2%
Local/CursorNext/ajwerner-16                   2.468n ± 1%
Local/CursorNext/tidwall-16                    1.918n ± 2%
Local/CursorNext/google-16                     1.907n ± 0%
Local/CursorAscend/ajwerner-16                 243.6µ ± 0%
Local/CursorAscend/tidwall-16                  761.4µ ± 1%
Local/CursorAscend/google-16                   672.9µ ± 1%
Tidwall/InsertSeq/ajwerner-16                  44.91n ± 3%
Tidwall/InsertSeq/tidwall-16                   27.32n ± 2%
Tidwall/InsertSeq/google-16                    46.73n ± 1%
Tidwall/GetSeq/ajwerner-16                     57.70n ± 1%
Tidwall/GetSeq/tidwall-16                      39.67n ± 2%
Tidwall/GetSeq/google-16                       63.88n ± 1%
Tidwall/InsertAfterClone/ajwerner-16           662.6n ± 3%
Tidwall/InsertAfterClone/tidwall-16            179.7µ ± 1%
Tidwall/InsertAfterClone/google-16             362.8µ ± 1%
Tidwall/PivotAscend/ajwerner-16                102.9n ± 1%
Tidwall/PivotAscend/tidwall-16                 83.39n ± 1%
Tidwall/PivotAscend/google-16                  106.1n ± 1%
Tidwall/PivotDescend/ajwerner-16               102.0n ± 1%
Tidwall/PivotDescend/tidwall-16                83.25n ± 1%
Tidwall/PivotDescend/google-16                 117.4n ± 4%
geomean                                        1.093µ
```

## Prerequisites

```
go install golang.org/x/perf/cmd/benchstat@latest
```

The chart tool (`cmd/benchchart`) is part of this module and requires no separate installation.

## Usage

```
cd bench

# Run benchmarks (saves to results/bench.txt)
make bench

# Print benchstat comparison table grouped by implementation
make compare

# Generate interactive HTML chart (results/chart.html)
make chart

# Run all of the above
make all

# Remove generated results
make clean
```

## Background

This benchmark suite was created to provide a rigorous multi-implementation comparison beyond the [tidwall/btree-benchmark](https://github.com/tidwall/btree-benchmark) repo, which only covers tidwall vs google.
