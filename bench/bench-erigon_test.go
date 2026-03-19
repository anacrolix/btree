package bench_test

import (
	"fmt"
	"testing"
)

// BenchmarkErigon covers the btree access patterns found in
// github.com/erigontech/erigon. Each sub-benchmark runs across all three
// implementations so that relative performance is directly comparable.
//
// Key differences between implementations that these benchmarks expose:
//   - ajwerner: O(1) lazy clone via reference counting; native bidirectional
//     iterator with zero allocations; SeekGE is a true tree walk.
//   - tidwall/google: Clone is a full O(n) copy; NewCursor snapshots all items
//     into a slice (O(n) alloc), so cursor benchmarks include that cost.
func BenchmarkErigon(b *testing.B) {
	var benchSizes = []int{100, 1_000, 10_000, 100_000}

	// UpsertBySize: full tree construction per iteration.
	// In erigon, trees are rebuilt each block (txpool pending set, dirty-file
	// lists in db/state), so construction cost is as important as lookup cost.
	b.Run("UpsertBySize", func(b *testing.B) {
		for _, n := range benchSizes {
			b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
				for _, impl := range impls {
					b.Run(impl.name, func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							tr := impl.new()
							for i := range n {
								tr.Insert(i)
							}
						}
					})
				}
			})
		}
	})

	// DeleteBySize: Clone then delete all entries.
	// The txpool removes confirmed transactions after each block; it clones
	// BySenderAndNonce first so concurrent readers see a consistent snapshot,
	// then deletes O(txns-per-block) entries from the copy.
	b.Run("DeleteBySize", func(b *testing.B) {
		for _, n := range benchSizes {
			b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
				for _, impl := range impls {
					tr := impl.new()
					for i := range n {
						tr.Insert(i)
					}
					b.Run(impl.name, func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							mc := tr.Clone()
							for i := range n {
								mc.Delete(i)
							}
						}
					})
				}
			})
		}
	})

	// IterateAscBySize: full ascending scan via cursor.
	// Primary access pattern in db/state (dirty file lists, merge decisions)
	// and versionmap (scanning all write versions for a key). For ajwerner the
	// cursor is a live iterator with zero allocations; tidwall and google pay
	// an O(n) snapshot cost at NewCursor time.
	b.Run("IterateAscBySize", func(b *testing.B) {
		for _, n := range benchSizes {
			b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
				for _, impl := range impls {
					tr := impl.new()
					for i := range n {
						tr.Insert(i)
					}
					b.Run(impl.name, func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							c := tr.NewCursor()
							for c.First(); c.Valid(); c.Next() {
								_ = c.Cur()
							}
						}
					})
				}
			})
		}
	})

	// IterateDescBySize: full descending scan via cursor.
	// The txpool walks backwards when a SeekGE overshoots (SeekGE + Prev);
	// body/header download structures evict oldest entries by descending.
	b.Run("IterateDescBySize", func(b *testing.B) {
		for _, n := range benchSizes {
			b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
				for _, impl := range impls {
					tr := impl.new()
					for i := range n {
						tr.Insert(i)
					}
					b.Run(impl.name, func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							c := tr.NewCursor()
							for c.Last(); c.Valid(); c.Prev() {
								_ = c.Cur()
							}
						}
					})
				}
			})
		}
	})

	// SeekGEBySize: n sequential SeekGE calls covering all keys.
	// versionmap (execution/state) does a SeekGE per txn-index lookup;
	// kvcache does a SeekGE per storage-key cache probe. For tidwall and
	// google, NewCursor snapshots the tree so SeekGE is a binary search on
	// a slice; ajwerner walks the live tree.
	b.Run("SeekGEBySize", func(b *testing.B) {
		for _, n := range benchSizes {
			b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
				for _, impl := range impls {
					tr := impl.new()
					for i := range n {
						tr.Insert(i)
					}
					b.Run(impl.name, func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							c := tr.NewCursor()
							for i := range n {
								c.SeekGE(i)
								_ = c.Cur()
							}
						}
					})
				}
			})
		}
	})

	// CloneBySize: O(1) clone (ajwerner) vs full copy (tidwall, google),
	// followed by one insert to trigger copy-on-write for ajwerner.
	// db/state clones dirty-file maps before each merge pass so the original
	// remains visible to concurrent readers. This is the most direct benchmark
	// of ajwerner's key structural advantage over the other two.
	b.Run("CloneBySize", func(b *testing.B) {
		for _, n := range benchSizes {
			b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
				for _, impl := range impls {
					tr := impl.new()
					for i := range n {
						tr.Insert(i)
					}
					b.Run(impl.name, func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							c := tr.Clone()
							c.Insert(n) // trigger copy-on-write for ajwerner
						}
					})
				}
			})
		}
	})
}
