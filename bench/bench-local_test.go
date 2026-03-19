package bench_test

import "testing"

// BenchmarkLocal contains original benchmarks focused on cursor and upsert
// performance across all three implementations.
func BenchmarkLocal(b *testing.B) {
	b.Run("Upsert", func(b *testing.B) {
		insertP := perm(benchmarkTreeSize)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, item := range insertP {
					tr.Insert(item)
				}
				b.ResetTimer()
				i := 0
				for b.Loop() {
					tr.Upsert(insertP[i%benchmarkTreeSize])
					i++
				}
			})
		}
	})

	// CursorSeek measures the cost of creating a cursor and seeking to a key.
	b.Run("CursorSeek", func(b *testing.B) {
		const size = 100000
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, item := range perm(size) {
					tr.Insert(item)
				}
				b.ResetTimer()
				i := 0
				for b.Loop() {
					c := tr.NewCursor()
					c.SeekGE(i % size)
					i++
				}
			})
		}
	})

	// CursorNext measures the cost of a single Next step on a live cursor.
	b.Run("CursorNext", func(b *testing.B) {
		const size = 100000
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, item := range perm(size) {
					tr.Insert(item)
				}
				b.ResetTimer()
				c := tr.NewCursor()
				c.First()
				for b.Loop() {
					if !c.Valid() {
						c.First()
					}
					c.Next()
				}
			})
		}
	})

	// CursorAscend measures full forward iteration via a cursor.
	b.Run("CursorAscend", func(b *testing.B) {
		const size = 100000
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, item := range perm(size) {
					tr.Insert(item)
				}
				b.ResetTimer()
				for b.Loop() {
					c := tr.NewCursor()
					for c.First(); c.Valid(); c.Next() {
					}
				}
			})
		}
	})
}
