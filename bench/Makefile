RESULTS_DIR := results
BENCH_OUT := $(RESULTS_DIR)/bench.txt
BENCH_COUNT := 10

.PHONY: all bench compare chart clean

all: bench compare chart

$(RESULTS_DIR):
	mkdir -p $(RESULTS_DIR)

bench: $(RESULTS_DIR)
	go test -bench=. -benchmem -count=$(BENCH_COUNT) -timeout=30m . | tee $(BENCH_OUT)

compare: $(BENCH_OUT)
	benchstat -col /1 $(BENCH_OUT)

chart: $(BENCH_OUT)
	go run ./cmd/benchchart/ < $(BENCH_OUT) > $(RESULTS_DIR)/chart.html
	@echo "Chart written to $(RESULTS_DIR)/chart.html"

clean:
	rm -rf $(RESULTS_DIR)
