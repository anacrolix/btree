// benchchart reads Go benchmark output from stdin and writes a Chart.js HTML
// file to stdout. Each top-level benchmark group (e.g. BenchmarkGoogle,
// BenchmarkLocal, BenchmarkTidwall) becomes its own chart section. Within each
// section, bars are grouped by operation with one bar per implementation.
//
// Usage:
//
//	go test -bench=. -benchmem -count=6 . | go run ./cmd/benchchart > results/chart.html
//	go run ./cmd/benchchart < results/bench.txt > results/chart.html
package main

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const chartJSURL = "https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js"

// fetchChartJS downloads the Chart.js bundle and returns the raw JS for inlining.
// On failure it returns an empty string and sets cdnURL so the caller can fall back.
func fetchChartJS() (raw template.JS, cdnURL string) {
	resp, err := http.Get(chartJSURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not fetch Chart.js (%v); using CDN src instead\n", err)
		return "", chartJSURL
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "warning: could not read Chart.js (%v); using CDN src instead\n", err)
		return "", chartJSURL
	}
	return template.JS(body), ""
}

type result struct {
	group    string // e.g. "Google"
	op       string // e.g. "Insert"
	impl     string // e.g. "ajwerner"
	nsOp     float64
	bOp      float64
	allocsOp float64
}

// parseLine parses a single benchmark output line.
// Expected format: BenchmarkGroup/Op/impl-N \t iters \t X ns/op ...
func parseLine(line string) (result, bool) {
	if !strings.HasPrefix(line, "Benchmark") {
		return result{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return result{}, false
	}
	name := fields[0]

	// Strip trailing -N (CPU count)
	if idx := strings.LastIndex(name, "-"); idx != -1 {
		if _, err := strconv.Atoi(name[idx+1:]); err == nil {
			name = name[:idx]
		}
	}

	parts := strings.Split(name, "/")
	if len(parts) < 3 {
		return result{}, false
	}
	group := strings.TrimPrefix(parts[0], "Benchmark")
	impl := parts[len(parts)-1]
	op := strings.Join(parts[1:len(parts)-1], "/") // e.g. "UpsertBySize/n=100"

	// Find ns/op, B/op, and allocs/op values
	var nsOp, bOp, allocsOp float64
	for i := 0; i+1 < len(fields); i++ {
		switch fields[i+1] {
		case "ns/op":
			if v, err := strconv.ParseFloat(fields[i], 64); err == nil {
				nsOp = v
			}
		case "B/op":
			if v, err := strconv.ParseFloat(fields[i], 64); err == nil {
				bOp = v
			}
		case "allocs/op":
			if v, err := strconv.ParseFloat(fields[i], 64); err == nil {
				allocsOp = v
			}
		}
	}
	if nsOp == 0 {
		return result{}, false
	}

	return result{group: group, op: op, impl: impl, nsOp: nsOp, bOp: bOp, allocsOp: allocsOp}, true
}

type chartData struct {
	Group          string
	Ops            []string
	Impls          []string
	Datasets       map[string]map[string][]float64 // [impl][op] = []ns/op samples
	BOpDatasets    map[string]map[string][]float64 // [impl][op] = []B/op samples
	AllocsDatasets map[string]map[string][]float64 // [impl][op] = []allocs/op samples
}

func main() {
	// nsAll[group][op][impl]     = []ns/op samples
	// bAll[group][op][impl]      = []B/op samples
	// allocsAll[group][op][impl] = []allocs/op samples
	all := map[string]map[string]map[string][]float64{}
	bAll := map[string]map[string]map[string][]float64{}
	allocsAll := map[string]map[string]map[string][]float64{}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		r, ok := parseLine(scanner.Text())
		if !ok {
			continue
		}
		if all[r.group] == nil {
			all[r.group] = map[string]map[string][]float64{}
			bAll[r.group] = map[string]map[string][]float64{}
			allocsAll[r.group] = map[string]map[string][]float64{}
		}
		if all[r.group][r.op] == nil {
			all[r.group][r.op] = map[string][]float64{}
			bAll[r.group][r.op] = map[string][]float64{}
			allocsAll[r.group][r.op] = map[string][]float64{}
		}
		all[r.group][r.op][r.impl] = append(all[r.group][r.op][r.impl], r.nsOp)
		bAll[r.group][r.op][r.impl] = append(bAll[r.group][r.op][r.impl], r.bOp)
		allocsAll[r.group][r.op][r.impl] = append(allocsAll[r.group][r.op][r.impl], r.allocsOp)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "reading stdin: %v\n", err)
		os.Exit(1)
	}
	if len(all) == 0 {
		fmt.Fprintln(os.Stderr, "no benchmark data found")
		os.Exit(1)
	}

	// Determine stable ordering: groups, ops, impls
	groupOrder := []string{"Google", "Erigon", "Tidwall", "Local"}
	var groups []chartData
	for _, g := range groupOrder {
		ops, ok := all[g]
		if !ok {
			continue
		}
		// Collect ops in encounter order isn't stable; sort for reproducibility.
		opList := stableKeys(ops)
		implSet := map[string]struct{}{}
		for _, opMap := range ops {
			for impl := range opMap {
				implSet[impl] = struct{}{}
			}
		}
		implList := []string{"ajwerner", "tidwall", "google"}
		// Include any impls not in our expected list
		for impl := range implSet {
			found := false
			for _, i := range implList {
				if i == impl {
					found = true
					break
				}
			}
			if !found {
				implList = append(implList, impl)
			}
		}
		// Filter to only impls that actually appear
		var activeImpls []string
		for _, impl := range implList {
			if _, ok := implSet[impl]; ok {
				activeImpls = append(activeImpls, impl)
			}
		}
		datasets := map[string]map[string][]float64{}
		bopDatasets := map[string]map[string][]float64{}
		allocsDatasets := map[string]map[string][]float64{}
		for _, impl := range activeImpls {
			datasets[impl] = map[string][]float64{}
			bopDatasets[impl] = map[string][]float64{}
			allocsDatasets[impl] = map[string][]float64{}
			for _, op := range opList {
				datasets[impl][op] = all[g][op][impl]
				bopDatasets[impl][op] = bAll[g][op][impl]
				allocsDatasets[impl][op] = allocsAll[g][op][impl]
			}
		}
		groups = append(groups, chartData{
			Group:          g,
			Ops:            opList,
			Impls:          activeImpls,
			Datasets:       datasets,
			BOpDatasets:    bopDatasets,
			AllocsDatasets: allocsDatasets,
		})
	}

	chartJSInline, chartJSSrc := fetchChartJS()
	data := struct {
		Groups        []chartData
		ChartJSInline template.JS
		ChartJSSrc    string
	}{
		Groups:        groups,
		ChartJSInline: chartJSInline,
		ChartJSSrc:    chartJSSrc,
	}
	if err := tmpl.Execute(os.Stdout, data); err != nil {
		fmt.Fprintf(os.Stderr, "rendering template: %v\n", err)
		os.Exit(1)
	}
}

// stableKeys returns map keys in insertion-stable order by iterating once.
// Since Go maps are unordered we preserve the order ops appear alphabetically.
func stableKeys(m map[string]map[string][]float64) []string {
	// Use a slice to preserve insertion order via sorted approach
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Sort for reproducibility
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// median returns the median of a sorted (or unsorted) sample slice.
func median(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	// simple selection sort for small slices
	s := make([]float64, len(samples))
	copy(s, samples)
	for i := range s {
		min := i
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[min] {
				min = j
			}
		}
		s[i], s[min] = s[min], s[i]
	}
	n := len(s)
	if n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[n/2]
}

func formatNs(ns float64) string {
	switch {
	case ns >= 1e9:
		return fmt.Sprintf("%.2fs", ns/1e9)
	case ns >= 1e6:
		return fmt.Sprintf("%.2fms", ns/1e6)
	case ns >= 1e3:
		return fmt.Sprintf("%.2fµs", ns/1e3)
	default:
		return fmt.Sprintf("%.2fns", ns)
	}
}

var funcs = template.FuncMap{
	"median":   median,
	"formatNs": formatNs,
	"json": func(v interface{}) template.JS {
		switch t := v.(type) {
		case []string:
			parts := make([]string, len(t))
			for i, s := range t {
				parts[i] = `"` + s + `"`
			}
			return template.JS("[" + strings.Join(parts, ",") + "]")
		case []float64:
			parts := make([]string, len(t))
			for i, f := range t {
				if math.IsNaN(f) || math.IsInf(f, 0) {
					parts[i] = "0"
				} else {
					parts[i] = strconv.FormatFloat(f, 'f', 2, 64)
				}
			}
			return template.JS("[" + strings.Join(parts, ",") + "]")
		}
		return template.JS("null")
	},
	// bopForOp returns absolute median B/op per impl for a single op.
	"bopForOp": func(bopDatasets map[string]map[string][]float64, impls []string, op string) []float64 {
		out := make([]float64, len(impls))
		for i, impl := range impls {
			out[i] = median(bopDatasets[impl][op])
		}
		return out
	},
	// allocsForOp returns absolute median allocs/op per impl for a single op.
	"allocsForOp": func(allocsDatasets map[string]map[string][]float64, impls []string, op string) []float64 {
		out := make([]float64, len(impls))
		for i, impl := range impls {
			out[i] = median(allocsDatasets[impl][op])
		}
		return out
	},
	"relativeForOp": func(datasets map[string]map[string][]float64, impls []string, op string) []float64 {
		base := median(datasets["ajwerner"][op])
		out := make([]float64, len(impls))
		for i, impl := range impls {
			val := median(datasets[impl][op])
			if base == 0 {
				out[i] = 1
			} else {
				out[i] = val / base
			}
		}
		return out
	},
}

var tmpl = template.Must(template.New("chart").Funcs(funcs).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>B-tree Implementation Comparison</title>
<style>
  body { font-family: system-ui, sans-serif; background: #f5f5f5; margin: 0; padding: 20px; }
  h1 { text-align: center; color: #333; }
  .section { background: white; border-radius: 8px; box-shadow: 0 1px 4px rgba(0,0,0,.15);
             margin: 24px auto; max-width: 1100px; padding: 24px; }
  h2 { margin-top: 0; color: #444; border-bottom: 1px solid #eee; padding-bottom: 8px; }
  h2 a { color: inherit; text-decoration: none; }
  h2 a:hover { text-decoration: underline; }
  .suite-table { display: grid; grid-template-columns: 13rem 1fr 1fr 1fr; gap: 6px 12px;
                 align-items: center; }
  .col-header { font-size: 0.75rem; font-weight: 600; color: #888; text-align: center;
                text-transform: uppercase; letter-spacing: .05em; padding: 4px 0 8px; }
  .row-label { font-size: 0.85rem; color: #444; text-align: right; padding-right: 8px;
               font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .chart-cell { background: #fafafa; border-radius: 4px; padding: 6px; min-width: 0; }
</style>
</head>
<body>
<h1>B-tree Implementation Comparison</h1>
<p style="text-align:center;color:#666;margin-top:-8px">Runtime is relative to ajwerner; memory in B/KiB/MiB and allocs are absolute. Lower is better in all cases.</p>
{{range $gi, $g := .Groups}}
<div class="section" id="{{$g.Group}}">
  <h2><a href="#{{$g.Group}}">{{$g.Group}} test suite</a></h2>
  <div class="suite-table">
    <div></div>
    <div class="col-header">Runtime</div>
    <div class="col-header">B/op</div>
    <div class="col-header">allocs/op</div>
  {{range $oi, $op := $g.Ops}}
    <div class="row-label">{{$op}}</div>
    <div class="chart-cell"><canvas id="c{{$gi}}_{{$oi}}"></canvas></div>
    <div class="chart-cell"><canvas id="m{{$gi}}_{{$oi}}"></canvas></div>
    <div class="chart-cell"><canvas id="a{{$gi}}_{{$oi}}"></canvas></div>
  {{end}}
  </div>
</div>
{{end}}
{{if .ChartJSSrc}}<script src="{{.ChartJSSrc}}"></script>{{end}}
<script>
{{if .ChartJSInline}}{{.ChartJSInline}}{{end}}
{{range $gi, $g := .Groups}}
(function(){
  const impls  = {{json $g.Impls}};
  const colors = {
    "ajwerner": "rgba(54,162,235,0.8)",
    "tidwall":  "rgba(255,99,132,0.8)",
    "google":   "rgba(75,192,100,0.8)",
  };
  const bg = impl => colors[impl] || "rgba(153,102,255,0.8)";
  function fmtBytes(v) {
    if (v >= 1048576) return (v/1048576).toFixed(2)+" MiB";
    if (v >= 1024)    return (v/1024).toFixed(1)+" KiB";
    return v.toFixed(0)+" B";
  }
  const chartOpts = (fmtTick, fmtTip) => ({
    responsive: true,
    plugins: {
      legend: { display: false },
      tooltip: { callbacks: { label: ctx => fmtTip(ctx) } },
    },
    scales: { y: { ticks: { callback: fmtTick } } },
  });
  {{range $oi, $op := $g.Ops}}
  new Chart(document.getElementById("c{{$gi}}_{{$oi}}"), {
    type: "bar",
    data: { labels: impls, datasets: [{ data: {{json (relativeForOp $g.Datasets $g.Impls $op)}}, backgroundColor: impls.map(bg) }] },
    options: chartOpts(v => (v*100).toFixed(0)+"%", ctx => (ctx.parsed.y*100).toFixed(1)+"%"),
  });
  new Chart(document.getElementById("m{{$gi}}_{{$oi}}"), {
    type: "bar",
    data: { labels: impls, datasets: [{ data: {{json (bopForOp $g.BOpDatasets $g.Impls $op)}}, backgroundColor: impls.map(bg) }] },
    options: chartOpts(fmtBytes, ctx => ctx.label+": "+fmtBytes(ctx.parsed.y)),
  });
  new Chart(document.getElementById("a{{$gi}}_{{$oi}}"), {
    type: "bar",
    data: { labels: impls, datasets: [{ data: {{json (allocsForOp $g.AllocsDatasets $g.Impls $op)}}, backgroundColor: impls.map(bg) }] },
    options: chartOpts(v => v.toFixed(0), ctx => ctx.label+": "+ctx.parsed.y.toFixed(0)+" allocs"),
  });
  {{end}}
})();
{{end}}
</script>
</body>
</html>
`))
