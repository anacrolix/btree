// Copyright 2021 Andrew Werner.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Stack-based iterator benchmarks.
//
// The Iterator is a value type backed by an iterStack that uses a fixed-size
// inline array ([iterStackDepth]iterFrame) so that, for any tree whose height
// is at most iterStackDepth (= 10), the iterator never touches the heap.
//
// With degree 32 and depth 10 that covers up to 32^10 ≈ 1.1×10^15 elements —
// far beyond any practical use-case — so heap allocation is effectively never
// triggered.
//
// The design is analogous to the approach described in
// https://github.com/RoaringBitmap/roaring/pull/354: expose an iterator as a
// plain value (not *Iterator or an interface), so the caller controls the
// allocation and the compiler can stack-allocate it when it does not escape.

package btree

import (
	"cmp"
	"math/rand"
	"testing"
)

const iterBenchSize = 10000

func buildBenchTree(n int) Set[int] {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range rand.Perm(n) {
		tr.Upsert(v)
	}
	return tr
}

// TestIteratorZeroAllocs verifies that all iterator operations complete
// without any heap allocations for a practical-sized tree.
func TestIteratorZeroAllocs(t *testing.T) {
	tr := buildBenchTree(iterBenchSize)

	check := func(name string, fn func()) {
		t.Helper()
		allocs := testing.AllocsPerRun(100, fn)
		if allocs != 0 {
			t.Errorf("%s: got %.1f allocs/op, want 0", name, allocs)
		}
	}

	check("First+Next full scan", func() {
		it := tr.Iterator()
		for it.First(); it.Valid(); it.Next() {
			_ = it.Cur()
		}
	})

	check("Last+Prev full scan", func() {
		it := tr.Iterator()
		for it.Last(); it.Valid(); it.Prev() {
			_ = it.Cur()
		}
	})

	check("SeekGE", func() {
		it := tr.Iterator()
		it.SeekGE(iterBenchSize / 2)
		_ = it.Valid()
	})

	check("SeekLT", func() {
		it := tr.Iterator()
		it.SeekLT(iterBenchSize / 2)
		_ = it.Valid()
	})

	check("SeekGE+Next partial scan", func() {
		it := tr.Iterator()
		it.SeekGE(iterBenchSize / 4)
		for n := 0; n < 100 && it.Valid(); n++ {
			_ = it.Cur()
			it.Next()
		}
	})

	check("bidirectional Next/Prev", func() {
		it := tr.Iterator()
		it.First()
		for i := 0; i < 50; i++ {
			it.Next()
		}
		for i := 0; i < 25; i++ {
			it.Prev()
		}
		_ = it.Cur()
	})
}

// BenchmarkIterFirst measures the cost of seeking to the first element.
func BenchmarkIterFirst(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		it.First()
		_ = it.Cur()
	}
}

// BenchmarkIterLast measures the cost of seeking to the last element.
func BenchmarkIterLast(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		it.Last()
		_ = it.Cur()
	}
}

// BenchmarkIterSeekGE measures a point-seek forward.
func BenchmarkIterSeekGE(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		it.SeekGE(i % iterBenchSize)
		_ = it.Valid()
	}
}

// BenchmarkIterSeekLT measures a point-seek backward.
func BenchmarkIterSeekLT(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		it.SeekLT(i % iterBenchSize)
		_ = it.Valid()
	}
}

// BenchmarkIterNext measures the amortised cost of a single Next step
// during a full ascending scan.
func BenchmarkIterNext(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	it := tr.Iterator()
	for i := 0; i < b.N; i++ {
		if !it.Valid() {
			it.First()
		}
		_ = it.Cur()
		it.Next()
	}
}

// BenchmarkIterPrev measures the amortised cost of a single Prev step
// during a full descending scan.
func BenchmarkIterPrev(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	it := tr.Iterator()
	for i := 0; i < b.N; i++ {
		if !it.Valid() {
			it.Last()
		}
		_ = it.Cur()
		it.Prev()
	}
}

// BenchmarkIterFullAscend measures a complete ascending scan.
func BenchmarkIterFullAscend(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		for it.First(); it.Valid(); it.Next() {
			_ = it.Cur()
		}
	}
}

// BenchmarkIterFullDescend measures a complete descending scan.
func BenchmarkIterFullDescend(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		for it.Last(); it.Valid(); it.Prev() {
			_ = it.Cur()
		}
	}
}

// BenchmarkIterSeekAndScan measures SeekGE followed by a 100-element scan.
func BenchmarkIterSeekAndScan(b *testing.B) {
	tr := buildBenchTree(iterBenchSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		it.SeekGE((i * 97) % iterBenchSize)
		for n := 0; n < 100 && it.Valid(); n++ {
			_ = it.Cur()
			it.Next()
		}
	}
}
