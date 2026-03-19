package bench_test

import (
	"sort"
	"testing"
)

// BenchmarkGoogle is ported from btree_google_test.go in the parent package
// and run against all three implementations via the BTree interface.
func BenchmarkGoogle(b *testing.B) {
	b.Run("Insert", func(b *testing.B) {
		insertP := perm(benchmarkTreeSize)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				i := 0
				for i < b.N {
					tr := impl.new()
					for _, item := range insertP {
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

	b.Run("Seek", func(b *testing.B) {
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
					tr.Seek(i % size)
					i++
				}
			})
		}
	})

	b.Run("DeleteInsert", func(b *testing.B) {
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
					tr.Delete(insertP[i%benchmarkTreeSize])
					tr.Insert(insertP[i%benchmarkTreeSize])
					i++
				}
			})
		}
	})

	// DeleteInsertCloneOnce clones once then measures steady-state delete+insert.
	// For ajwerner this triggers copy-on-write on first mutation;
	// tidwall and google pay the full copy cost upfront in Clone().
	b.Run("DeleteInsertCloneOnce", func(b *testing.B) {
		insertP := perm(benchmarkTreeSize)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, item := range insertP {
					tr.Insert(item)
				}
				tr = tr.Clone()
				b.ResetTimer()
				i := 0
				for b.Loop() {
					tr.Delete(insertP[i%benchmarkTreeSize])
					tr.Insert(insertP[i%benchmarkTreeSize])
					i++
				}
			})
		}
	})

	// DeleteInsertCloneEachTime clones before every delete+insert,
	// measuring the combined cost of cloning and mutation per operation.
	b.Run("DeleteInsertCloneEachTime", func(b *testing.B) {
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
					tr = tr.Clone()
					tr.Delete(insertP[i%benchmarkTreeSize])
					tr.Insert(insertP[i%benchmarkTreeSize])
					i++
				}
			})
		}
	})

	b.Run("Delete", func(b *testing.B) {
		insertP := perm(benchmarkTreeSize)
		removeP := perm(benchmarkTreeSize)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				i := 0
				for i < b.N {
					b.StopTimer()
					tr := impl.new()
					for _, v := range insertP {
						tr.Insert(v)
					}
					b.StartTimer()
					for _, item := range removeP {
						tr.Delete(item)
						i++
						if i >= b.N {
							return
						}
					}
				}
			})
		}
	})

	b.Run("Get", func(b *testing.B) {
		insertP := perm(benchmarkTreeSize)
		lookupP := perm(benchmarkTreeSize)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				i := 0
				for i < b.N {
					b.StopTimer()
					tr := impl.new()
					for _, v := range insertP {
						tr.Insert(v)
					}
					b.StartTimer()
					for _, item := range lookupP {
						tr.Seek(item)
						i++
						if i >= b.N {
							return
						}
					}
				}
			})
		}
	})

	b.Run("Ascend", func(b *testing.B) {
		arr := perm(benchmarkTreeSize)
		sort.Ints(arr)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, v := range arr {
					tr.Insert(v)
				}
				b.ResetTimer()
				for b.Loop() {
					j := 0
					tr.Ascend(func(k int) bool {
						if k != arr[j] {
							b.Fatalf("mismatch: want %v got %v", arr[j], k)
						}
						j++
						return true
					})
				}
			})
		}
	})

	b.Run("Descend", func(b *testing.B) {
		arr := perm(benchmarkTreeSize)
		sort.Ints(arr)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, v := range arr {
					tr.Insert(v)
				}
				b.ResetTimer()
				for b.Loop() {
					j := len(arr) - 1
					tr.Descend(func(k int) bool {
						if k != arr[j] {
							b.Fatalf("mismatch: want %v got %v", arr[j], k)
						}
						j--
						return true
					})
				}
			})
		}
	})

	b.Run("AscendRange", func(b *testing.B) {
		arr := perm(benchmarkTreeSize)
		sort.Ints(arr)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, v := range arr {
					tr.Insert(v)
				}
				b.ResetTimer()
				for b.Loop() {
					j := 100
					hi := arr[len(arr)-100]
					tr.AscendRange(100, hi, func(k int) bool {
						if k != arr[j] {
							b.Fatalf("mismatch: want %v got %v", arr[j], k)
						}
						j++
						return true
					})
					if j != len(arr)-100 {
						b.Fatalf("j: want %v got %v", len(arr)-100, j)
					}
				}
			})
		}
	})

	b.Run("DescendRange", func(b *testing.B) {
		arr := perm(benchmarkTreeSize)
		sort.Ints(arr)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, v := range arr {
					tr.Insert(v)
				}
				b.ResetTimer()
				for b.Loop() {
					j := len(arr) - 100
					pivot := arr[len(arr)-100]
					tr.DescendRange(pivot, 100, func(k int) bool {
						if k != arr[j] {
							b.Fatalf("mismatch: want %v got %v", arr[j], k)
						}
						j--
						return true
					})
					if j != 100 {
						b.Fatalf("j: want %v got %v", 100, j)
					}
				}
			})
		}
	})

	b.Run("AscendGreaterOrEqual", func(b *testing.B) {
		arr := perm(benchmarkTreeSize)
		sort.Ints(arr)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, v := range arr {
					tr.Insert(v)
				}
				b.ResetTimer()
				for b.Loop() {
					j, k := 100, 0
					tr.AscendFrom(100, func(item int) bool {
						if item != arr[j] {
							b.Fatalf("mismatch: want %v got %v", arr[j], item)
						}
						j++
						k++
						return true
					})
					if j != len(arr) {
						b.Fatalf("j: want %v got %v", len(arr), j)
					}
					if k != len(arr)-100 {
						b.Fatalf("k: want %v got %v", len(arr)-100, k)
					}
				}
			})
		}
	})

	b.Run("DescendLessOrEqual", func(b *testing.B) {
		arr := perm(benchmarkTreeSize)
		sort.Ints(arr)
		for _, impl := range impls {
			b.Run(impl.name, func(b *testing.B) {
				tr := impl.new()
				for _, v := range arr {
					tr.Insert(v)
				}
				b.ResetTimer()
				for b.Loop() {
					j := len(arr) - 100
					k := len(arr)
					pivot := arr[len(arr)-100]
					tr.DescendFrom(pivot, func(item int) bool {
						if item != arr[j] {
							b.Fatalf("mismatch: want %v got %v", arr[j], item)
						}
						j--
						k--
						return true
					})
					if j != -1 {
						b.Fatalf("j: want -1 got %v", j)
					}
					if k != 99 {
						b.Fatalf("k: want 99 got %v", k)
					}
				}
			})
		}
	})
}
