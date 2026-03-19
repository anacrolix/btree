package bench_test

import "testing"

// BenchmarkTidwall is ported from the tidwall/btree benchmark suite and run
// against all three implementations. It focuses on sequential-insert patterns
// and pivot (seek + short scan) operations at large tree sizes.
func BenchmarkTidwall(b *testing.B) {
	// InsertSeq inserts items in ascending sorted order.
	b.Run("InsertSeq", func(b *testing.B) {
		insertSeq := rang(benchmarkTreeSizeLarge)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				i := 0
				for i < b.N {
					tr := impl.new()
					for _, item := range insertSeq {
						tr.Insert(item)
						i++
						if i >= b.N {
							return
						}
					}
				}
			})
		}
	})

	// GetSeq populates with random order then looks up keys in sequential order.
	b.Run("GetSeq", func(b *testing.B) {
		const size = benchmarkTreeSizeLarge
		lookupSeq := rang(size)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				i := 0
				for i < b.N {
					b.StopTimer()
					tr := impl.new()
					for _, v := range perm(size) {
						tr.Insert(v)
					}
					b.StartTimer()
					for _, item := range lookupSeq {
						tr.Get(item)
						i++
						if i >= b.N {
							return
						}
					}
				}
			})
		}
	})

	// InsertAfterClone clones a pre-populated tree then inserts new random items.
	b.Run("InsertAfterClone", func(b *testing.B) {
		base := perm(benchmarkTreeSize)
		extra := perm(benchmarkTreeSize)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				src := impl.new()
				for _, item := range base {
					src.Insert(item)
				}
				b.ResetTimer()
				i := 0
				for b.Loop() {
					tr := src.Clone()
					tr.Insert(extra[i%benchmarkTreeSize])
					i++
				}
			})
		}
	})

	// PivotAscend seeks to a random position and iterates 10 items ascending.
	b.Run("PivotAscend", func(b *testing.B) {
		const size = benchmarkTreeSizeLarge
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, item := range perm(size) {
					tr.Insert(item)
				}
				b.ResetTimer()
				i := 0
				for b.Loop() {
					n := 0
					tr.AscendFrom(i%size, func(_ int) bool {
						n++
						return n < 10
					})
					i++
				}
			})
		}
	})

	// PivotDescend seeks to a random position and iterates 10 items descending.
	b.Run("PivotDescend", func(b *testing.B) {
		const size = benchmarkTreeSizeLarge
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, item := range perm(size) {
					tr.Insert(item)
				}
				b.ResetTimer()
				i := 0
				for b.Loop() {
					n := 0
					tr.DescendFrom(i%size, func(_ int) bool {
						n++
						return n < 10
					})
					i++
				}
			})
		}
	})
}
