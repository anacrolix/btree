package bench_test

import (
	"reflect"
	"sync"
	"testing"
)

func TestBTreeFull(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for i := 0; i < 10; i++ {
			if _, ok := tr.Min(); ok {
				t.Fatal("empty tree should have no min")
			}
			if _, ok := tr.Max(); ok {
				t.Fatal("empty tree should have no max")
			}
			for _, item := range perm(treeSize) {
				if tr.Upsert(item) {
					t.Fatal("insert found existing item", item)
				}
			}
			for _, item := range perm(treeSize) {
				if !tr.Get(item) {
					t.Fatal("get did not find item", item)
				}
			}
			for _, item := range perm(treeSize) {
				if !tr.Upsert(item) {
					t.Fatal("upsert of existing item should report overwrote", item)
				}
			}
			if min, ok := tr.Min(); !ok || min != 0 {
				t.Fatalf("min: want 0, got %v (ok=%v)", min, ok)
			}
			if max, ok := tr.Max(); !ok || max != treeSize-1 {
				t.Fatalf("max: want %v, got %v (ok=%v)", treeSize-1, max, ok)
			}
			if got, want := all(tr), rang(treeSize); !reflect.DeepEqual(got, want) {
				t.Fatalf("ascending mismatch:\n got: %v\nwant: %v", got, want)
			}
			if got, want := allrev(tr), rangrev(treeSize); !reflect.DeepEqual(got, want) {
				t.Fatalf("descending mismatch:\n got: %v\nwant: %v", got, want)
			}
			for _, item := range perm(treeSize) {
				if !tr.Delete(item) {
					t.Fatalf("delete did not find %v", item)
				}
			}
			if got := all(tr); len(got) > 0 {
				t.Fatalf("tree not empty after deleting all: %v", got)
			}
		}
	})
}

func TestDeleteMin(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range perm(100) {
			tr.Insert(v)
		}
		var got []int
		for {
			v, ok := tr.DeleteMin()
			if !ok {
				break
			}
			got = append(got, v)
		}
		if want := rang(100); !reflect.DeepEqual(got, want) {
			t.Fatalf("deletemin:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestDeleteMax(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range perm(100) {
			tr.Insert(v)
		}
		var got []int
		for {
			v, ok := tr.DeleteMax()
			if !ok {
				break
			}
			got = append(got, v)
		}
		for i, j := 0, len(got)-1; i < j; i, j = i+1, j-1 {
			got[i], got[j] = got[j], got[i]
		}
		if want := rang(100); !reflect.DeepEqual(got, want) {
			t.Fatalf("deletemax:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestAscendRange(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range perm(100) {
			tr.Insert(v)
		}
		var got []int
		ascendRange(tr, 40, 60, func(a int) bool { got = append(got, a); return true })
		if want := rang(100)[40:60]; !reflect.DeepEqual(got, want) {
			t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
		}
		got = got[:0]
		ascendRange(tr, 40, 60, func(a int) bool {
			if a > 50 {
				return false
			}
			got = append(got, a)
			return true
		})
		if want := rang(100)[40:51]; !reflect.DeepEqual(got, want) {
			t.Fatalf("ascendrange early-stop:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestDescendRange(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range perm(100) {
			tr.Insert(v)
		}
		var got []int
		descendRange(tr, 60, 40, func(a int) bool { got = append(got, a); return true })
		if want := rangrev(100)[39:59]; !reflect.DeepEqual(got, want) {
			t.Fatalf("descendrange:\n got: %v\nwant: %v", got, want)
		}
		got = got[:0]
		descendRange(tr, 60, 40, func(a int) bool {
			if a < 50 {
				return false
			}
			got = append(got, a)
			return true
		})
		if want := rangrev(100)[39:50]; !reflect.DeepEqual(got, want) {
			t.Fatalf("descendrange early-stop:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestAscendLessThan(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range perm(100) {
			tr.Insert(v)
		}
		var got []int
		ascendLessThan(tr, 60, func(a int) bool { got = append(got, a); return true })
		if want := rang(100)[:60]; !reflect.DeepEqual(got, want) {
			t.Fatalf("ascendlessthan:\n got: %v\nwant: %v", got, want)
		}
		got = got[:0]
		ascendLessThan(tr, 60, func(a int) bool {
			if a > 50 {
				return false
			}
			got = append(got, a)
			return true
		})
		if want := rang(100)[:51]; !reflect.DeepEqual(got, want) {
			t.Fatalf("ascendlessthan early-stop:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestDescendLessOrEqual(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range perm(100) {
			tr.Insert(v)
		}
		var got []int
		descendLessOrEqual(tr, 40, func(a int) bool { got = append(got, a); return true })
		if want := rangrev(100)[59:]; !reflect.DeepEqual(got, want) {
			t.Fatalf("descendlessorequal:\n got: %v\nwant: %v", got, want)
		}
		got = got[:0]
		descendLessOrEqual(tr, 60, func(a int) bool {
			if a < 50 {
				return false
			}
			got = append(got, a)
			return true
		})
		if want := rangrev(100)[39:50]; !reflect.DeepEqual(got, want) {
			t.Fatalf("descendlessorequal early-stop:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestAscendGreaterOrEqual(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range perm(100) {
			tr.Insert(v)
		}
		var got []int
		ascendGreaterOrEqual(tr, 40, func(a int) bool { got = append(got, a); return true })
		if want := rang(100)[40:]; !reflect.DeepEqual(got, want) {
			t.Fatalf("ascendgreaterorequal:\n got: %v\nwant: %v", got, want)
		}
		got = got[:0]
		ascendGreaterOrEqual(tr, 40, func(a int) bool {
			if a > 50 {
				return false
			}
			got = append(got, a)
			return true
		})
		if want := rang(100)[40:51]; !reflect.DeepEqual(got, want) {
			t.Fatalf("ascendgreaterorequal early-stop:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestDescendGreaterThan(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range perm(100) {
			tr.Insert(v)
		}
		var got []int
		descendGreaterThan(tr, 40, func(a int) bool { got = append(got, a); return true })
		if want := rangrev(100)[:59]; !reflect.DeepEqual(got, want) {
			t.Fatalf("descendgreaterthan:\n got: %v\nwant: %v", got, want)
		}
		got = got[:0]
		descendGreaterThan(tr, 40, func(a int) bool {
			if a < 50 {
				return false
			}
			got = append(got, a)
			return true
		})
		if want := rangrev(100)[:50]; !reflect.DeepEqual(got, want) {
			t.Fatalf("descendgreaterthan early-stop:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestEmptyTree(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		if _, ok := tr.Min(); ok {
			t.Error("min of empty tree")
		}
		if _, ok := tr.Max(); ok {
			t.Error("max of empty tree")
		}
		if _, ok := tr.DeleteMin(); ok {
			t.Error("deleteMin of empty tree")
		}
		if _, ok := tr.DeleteMax(); ok {
			t.Error("deleteMax of empty tree")
		}
		if tr.Get(0) {
			t.Error("get on empty tree")
		}
		var got []int
		ascendRange(tr, 0, 10, func(a int) bool { got = append(got, a); return true })
		if len(got) != 0 {
			t.Errorf("ascendRange on empty tree: %v", got)
		}
	})
}

func TestSingleElement(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		tr.Insert(42)
		if v, ok := tr.Min(); !ok || v != 42 {
			t.Errorf("min: got %v,%v", v, ok)
		}
		if v, ok := tr.Max(); !ok || v != 42 {
			t.Errorf("max: got %v,%v", v, ok)
		}
		if !tr.Get(42) {
			t.Error("get(42) = false")
		}
		if tr.Get(0) {
			t.Error("get(0) on single-elem tree = true")
		}
		if v, ok := tr.DeleteMin(); !ok || v != 42 {
			t.Errorf("deleteMin: got %v,%v", v, ok)
		}
		if tr.Len() != 0 {
			t.Errorf("len after deleteMin: %d", tr.Len())
		}
	})
}

func TestDescendRangeGaps(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for i := 0; i < 100; i += 2 {
			tr.Insert(i)
		}
		var got []int
		descendRange(tr, 55, 40, func(a int) bool { got = append(got, a); return true })
		want := []int{54, 52, 50, 48, 46, 44, 42}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("descendRange gaps:\n got: %v\nwant: %v", got, want)
		}
	})
}

func TestReset(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for i := 0; i < 100; i++ {
			tr.Insert(i)
		}
		if tr.Len() != 100 {
			t.Fatalf("want 100, got %d", tr.Len())
		}
		tr.Reset()
		if tr.Len() != 0 {
			t.Fatalf("after Reset: want 0, got %d", tr.Len())
		}
		for i := 0; i < 100; i++ {
			tr.Insert(i)
		}
		if tr.Len() != 100 {
			t.Fatalf("after repopulate: want 100, got %d", tr.Len())
		}
		if got := all(tr); !reflect.DeepEqual(got, rang(100)) {
			t.Fatalf("order after repopulate: got %v…", got[:10])
		}
	})
}

func TestSeekBoundaries(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range rang(10) {
			tr.Insert(v)
		}
		it := tr.NewCursor()

		it.SeekGE(100)
		if it.Valid() {
			t.Errorf("SeekGE(100) should be invalid, got %v", it.Cur())
		}
		it.SeekLT(0)
		if it.Valid() {
			t.Errorf("SeekLT(0) should be invalid, got %v", it.Cur())
		}
		it.SeekGE(0)
		if !it.Valid() || it.Cur() != 0 {
			t.Errorf("SeekGE(0): valid=%v cur=%v", it.Valid(), it.Cur())
		}
		it.SeekGE(9)
		if !it.Valid() || it.Cur() != 9 {
			t.Errorf("SeekGE(9): valid=%v cur=%v", it.Valid(), it.Cur())
		}
		it.SeekLT(10)
		if !it.Valid() || it.Cur() != 9 {
			t.Errorf("SeekLT(10): valid=%v cur=%v", it.Valid(), it.Cur())
		}
		it.SeekLT(1)
		if !it.Valid() || it.Cur() != 0 {
			t.Errorf("SeekLT(1): valid=%v cur=%v", it.Valid(), it.Cur())
		}
	})
}

func TestIteratorBidirectional(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for _, v := range rang(10) {
			tr.Insert(v)
		}
		it := tr.NewCursor()
		it.First()
		for range 5 {
			it.Next()
		}
		if it.Cur() != 5 {
			t.Fatalf("after 5 Next: want 5, got %v", it.Cur())
		}
		for range 3 {
			it.Prev()
		}
		if it.Cur() != 2 {
			t.Fatalf("after 3 Prev: want 2, got %v", it.Cur())
		}
	})
}

func TestSeekMissingKey(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		for i := range 10000 {
			tr.Insert(i * 2) // 0, 2, 4, …, 19998
		}
		it := tr.NewCursor()

		it.SeekGE(501)
		if !it.Valid() || it.Cur() != 502 {
			t.Fatalf("SeekGE(501): want 502, got valid=%v cur=%v", it.Valid(), it.Cur())
		}
		it.Next()
		if !it.Valid() || it.Cur() != 504 {
			t.Fatalf("SeekGE(501)+Next: want 504, got %v", it.Cur())
		}

		it.SeekGE(501)
		if !it.Valid() || it.Cur() != 502 {
			t.Fatalf("SeekGE(501): want 502, got %v", it.Cur())
		}
		it.Prev()
		if !it.Valid() || it.Cur() != 500 {
			t.Fatalf("SeekGE(501)+Prev: want 500, got %v", it.Cur())
		}

		it.SeekLT(501)
		if !it.Valid() || it.Cur() != 500 {
			t.Fatalf("SeekLT(501): want 500, got valid=%v cur=%v", it.Valid(), it.Cur())
		}
		it.Prev()
		if !it.Valid() || it.Cur() != 498 {
			t.Fatalf("SeekLT(501)+Prev: want 498, got %v", it.Cur())
		}
	})
}

func TestIteratorFullScan(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		const N = 1000
		sorted := rang(N)
		for _, v := range perm(N) {
			tr.Insert(v)
		}
		it := tr.NewCursor()

		var got []int
		for it.First(); it.Valid(); it.Next() {
			got = append(got, it.Cur())
		}
		if !reflect.DeepEqual(got, sorted) {
			t.Fatalf("forward scan mismatch at len %d", len(got))
		}

		got = got[:0]
		for it.Last(); it.Valid(); it.Prev() {
			got = append(got, it.Cur())
		}
		want := rangrev(N)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("backward scan mismatch at len %d", len(got))
		}

		var fwd, bwd []int
		for it.First(); it.Valid(); it.Next() {
			fwd = append(fwd, it.Cur())
			if it.Cur() == N/2 {
				for it.Prev(); it.Valid(); it.Prev() {
					bwd = append(bwd, it.Cur())
				}
				break
			}
		}
		if len(fwd) != N/2+1 {
			t.Fatalf("forward half: want %d elems, got %d", N/2+1, len(fwd))
		}
		if len(bwd) != N/2 {
			t.Fatalf("backward half: want %d elems, got %d", N/2, len(bwd))
		}
		for i, v := range bwd {
			if v != N/2-1-i {
				t.Fatalf("backward[%d]: want %d, got %d", i, N/2-1-i, v)
			}
		}
	})
}

func TestRandomStress(t *testing.T) {
	forEachImpl(t, func(t *testing.T, tr BTree) {
		const N = 10000
		keys := perm(N)
		for _, k := range keys {
			tr.Insert(k)
		}
		if tr.Len() != N {
			t.Fatalf("len after insert: want %d, got %d", N, tr.Len())
		}
		prev := -1
		count := 0
		tr.Ascend(func(v int) bool {
			if v <= prev {
				t.Fatalf("order violation: %d after %d", v, prev)
			}
			prev = v
			count++
			return true
		})
		if count != N {
			t.Fatalf("scan count: want %d, got %d", N, count)
		}
		for _, k := range keys {
			if !tr.Delete(k) {
				t.Fatalf("Delete(%d) = false, expected true", k)
			}
		}
		if tr.Len() != 0 {
			t.Fatalf("len after delete all: want 0, got %d", tr.Len())
		}
		for _, k := range keys[:10] {
			if tr.Delete(k) {
				t.Fatalf("double-Delete(%d) returned true", k)
			}
		}
	})
}

// TestCloneConcurrentOperations verifies that cloned trees are independent.
// ajwerner uses O(1) lazy cloning; tidwall and google perform a full copy.
func TestCloneConcurrentOperations(t *testing.T) {
	for _, impl := range impls {
		t.Run(impl.name, func(t *testing.T) {
			tr := impl.new()
			var trees []BTree
			p := perm(cloneTestSize)
			var wg sync.WaitGroup
			var mu sync.Mutex
			wg.Add(1)
			go cloneTestHelper(t, tr, 0, p, &wg, &trees, &mu)

			wg.Wait()
			want := rang(cloneTestSize)
			t.Logf("Checking %d trees", len(trees))
			for i, tree := range trees {
				if !reflect.DeepEqual(want, all(tree)) {
					t.Errorf("tree %v mismatch", i)
				}
			}
			toRemove := rang(cloneTestSize)[cloneTestSize/2:]
			for i := 0; i < len(trees)/2; i++ {
				tree := trees[i]
				wg.Add(1)
				go func() {
					for _, item := range toRemove {
						tree.Delete(item)
					}
					wg.Done()
				}()
			}
			wg.Wait()
			for i, tree := range trees {
				wantpart := want
				if i < len(trees)/2 {
					wantpart = want[:cloneTestSize/2]
				}
				if got := all(tree); !reflect.DeepEqual(wantpart, got) {
					t.Errorf("tree %v after removal: want len %v got len %v", i, len(wantpart), len(got))
				}
			}
		})
	}
}

func cloneTestHelper(t *testing.T, tr BTree, start int, p []int, wg *sync.WaitGroup, trees *[]BTree, mu *sync.Mutex) {
	t.Logf("Starting clone at %v", start)
	mu.Lock()
	*trees = append(*trees, tr)
	mu.Unlock()
	for i := start; i < cloneTestSize; i++ {
		tr.Insert(p[i])
		if i%(cloneTestSize/5) == 0 {
			clone := tr.Clone()
			wg.Add(1)
			go cloneTestHelper(t, clone, i+1, p, wg, trees, mu)
		}
	}
	wg.Done()
}
