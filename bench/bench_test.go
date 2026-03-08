// Package bench_test provides comparative tests and benchmarks across:
//   - github.com/anacrolix/btree (ajwerner's btree)
//   - github.com/tidwall/btree
//   - github.com/google/btree
//
// Tests and benchmarks are ported from btree_google_test.go in the parent
// package and run against all three implementations via a common interface.
//
// Run benchmarks with:
//
//	go test -bench=. -benchmem -count=6
package bench_test

import (
	"cmp"
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"testing"

	ajwernerbtree "github.com/anacrolix/btree"
	googlebtree "github.com/google/btree"
	tidwallbtree "github.com/tidwall/btree"
)

const (
	treeSize          = 10000
	cloneTestSize     = 10000
	benchmarkTreeSize = 10000
)

// BTree is the common interface satisfied by all three adapters.
type BTree interface {
	Insert(k int)
	Delete(k int) bool
	Get(k int) bool
	Len() int
	Reset()
	Clone() BTree // ajwerner uses O(1) lazy clone; others perform a full copy
	Seek(k int) bool // true if any item >= k exists
	Ascend(fn func(k int) bool)
	Descend(fn func(k int) bool)
	AscendFrom(ge int, fn func(k int) bool)
	DescendFrom(le int, fn func(k int) bool) // start at largest item <= le
	AscendRange(ge, lt int, fn func(k int) bool)
	DescendRange(le, gt int, fn func(k int) bool) // (gt, le] descending
	Min() (int, bool)
	Max() (int, bool)
	DeleteMin() (int, bool)
	DeleteMax() (int, bool)
}

// ===== ajwerner/btree adapter =====

type ajwernerAdapter struct {
	m ajwernerbtree.Map[int, struct{}]
}

func newAjwerner() BTree {
	return &ajwernerAdapter{m: ajwernerbtree.MakeMap[int, struct{}](cmp.Compare[int])}
}

func (a *ajwernerAdapter) Insert(k int) { a.m.Upsert(k, struct{}{}) }

func (a *ajwernerAdapter) Delete(k int) bool {
	_, _, ok := a.m.Delete(k)
	return ok
}

func (a *ajwernerAdapter) Get(k int) bool { _, ok := a.m.Get(k); return ok }
func (a *ajwernerAdapter) Len() int        { return a.m.Len() }
func (a *ajwernerAdapter) Reset()          { a.m.Reset() }

func (a *ajwernerAdapter) Clone() BTree {
	return &ajwernerAdapter{m: a.m.Clone()}
}

func (a *ajwernerAdapter) Seek(k int) bool {
	it := a.m.Iterator()
	it.SeekGE(k)
	return it.Valid()
}

func (a *ajwernerAdapter) Ascend(fn func(k int) bool) {
	it := a.m.Iterator()
	for it.First(); it.Valid(); it.Next() {
		if !fn(it.Cur()) {
			return
		}
	}
}

func (a *ajwernerAdapter) Descend(fn func(k int) bool) {
	it := a.m.Iterator()
	for it.Last(); it.Valid(); it.Prev() {
		if !fn(it.Cur()) {
			return
		}
	}
}

func (a *ajwernerAdapter) AscendFrom(ge int, fn func(k int) bool) {
	it := a.m.Iterator()
	for it.SeekGE(ge); it.Valid(); it.Next() {
		if !fn(it.Cur()) {
			return
		}
	}
}

func (a *ajwernerAdapter) DescendFrom(le int, fn func(k int) bool) {
	it := a.m.Iterator()
	it.SeekGE(le)
	if !it.Valid() {
		it.Last()
	} else if it.Cur() > le {
		it.Prev()
	}
	for it.Valid() {
		if !fn(it.Cur()) {
			return
		}
		it.Prev()
	}
}

func (a *ajwernerAdapter) AscendRange(ge, lt int, fn func(k int) bool) {
	it := a.m.Iterator()
	for it.SeekGE(ge); it.Valid() && it.Cur() < lt; it.Next() {
		if !fn(it.Cur()) {
			return
		}
	}
}

func (a *ajwernerAdapter) DescendRange(le, gt int, fn func(k int) bool) {
	it := a.m.Iterator()
	it.SeekGE(le)
	if !it.Valid() {
		it.Last()
	} else if it.Cur() > le {
		it.Prev()
	}
	for it.Valid() && it.Cur() > gt {
		if !fn(it.Cur()) {
			return
		}
		it.Prev()
	}
}

func (a *ajwernerAdapter) Min() (int, bool) {
	it := a.m.Iterator()
	it.First()
	if !it.Valid() {
		return 0, false
	}
	return it.Cur(), true
}

func (a *ajwernerAdapter) Max() (int, bool) {
	it := a.m.Iterator()
	it.Last()
	if !it.Valid() {
		return 0, false
	}
	return it.Cur(), true
}

func (a *ajwernerAdapter) DeleteMin() (int, bool) {
	it := a.m.Iterator()
	it.First()
	if !it.Valid() {
		return 0, false
	}
	v := it.Cur()
	a.m.Delete(v)
	return v, true
}

func (a *ajwernerAdapter) DeleteMax() (int, bool) {
	it := a.m.Iterator()
	it.Last()
	if !it.Valid() {
		return 0, false
	}
	v := it.Cur()
	a.m.Delete(v)
	return v, true
}

// ===== tidwall/btree adapter =====

type tidwallAdapter struct {
	m tidwallbtree.Map[int, struct{}]
}

func newTidwall() BTree { return &tidwallAdapter{} }

func (a *tidwallAdapter) Insert(k int) { a.m.Set(k, struct{}{}) }

func (a *tidwallAdapter) Delete(k int) bool {
	_, ok := a.m.Delete(k)
	return ok
}

func (a *tidwallAdapter) Get(k int) bool { _, ok := a.m.Get(k); return ok }
func (a *tidwallAdapter) Len() int        { return a.m.Len() }
func (a *tidwallAdapter) Reset()          { a.m = tidwallbtree.Map[int, struct{}]{} }

func (a *tidwallAdapter) Seek(k int) bool {
	var found bool
	a.m.Ascend(k, func(key int, _ struct{}) bool { found = true; return false })
	return found
}

func (a *tidwallAdapter) Ascend(fn func(k int) bool) {
	a.m.Scan(func(k int, _ struct{}) bool { return fn(k) })
}

func (a *tidwallAdapter) Descend(fn func(k int) bool) {
	a.m.Reverse(func(k int, _ struct{}) bool { return fn(k) })
}

func (a *tidwallAdapter) AscendFrom(ge int, fn func(k int) bool) {
	a.m.Ascend(ge, func(k int, _ struct{}) bool { return fn(k) })
}

func (a *tidwallAdapter) DescendFrom(le int, fn func(k int) bool) {
	a.m.Descend(le, func(k int, _ struct{}) bool { return fn(k) })
}

func (a *tidwallAdapter) AscendRange(ge, lt int, fn func(k int) bool) {
	a.m.Ascend(ge, func(k int, _ struct{}) bool {
		if k >= lt {
			return false
		}
		return fn(k)
	})
}

func (a *tidwallAdapter) DescendRange(le, gt int, fn func(k int) bool) {
	a.m.Descend(le, func(k int, _ struct{}) bool {
		if k <= gt {
			return false
		}
		return fn(k)
	})
}

func (a *tidwallAdapter) Min() (int, bool) { k, _, ok := a.m.Min(); return k, ok }
func (a *tidwallAdapter) Max() (int, bool) { k, _, ok := a.m.Max(); return k, ok }

func (a *tidwallAdapter) DeleteMin() (int, bool) { k, _, ok := a.m.PopMin(); return k, ok }
func (a *tidwallAdapter) DeleteMax() (int, bool) { k, _, ok := a.m.PopMax(); return k, ok }

func (a *tidwallAdapter) Clone() BTree {
	clone := &tidwallAdapter{}
	a.m.Scan(func(k int, v struct{}) bool { clone.m.Set(k, v); return true })
	return clone
}

// ===== google/btree adapter =====

type googleAdapter struct {
	t    *googlebtree.BTreeG[int]
	less func(a, b int) bool
}

func newGoogle() BTree {
	less := func(a, b int) bool { return a < b }
	return &googleAdapter{t: googlebtree.NewG[int](32, less), less: less}
}

func (a *googleAdapter) Insert(k int) { a.t.ReplaceOrInsert(k) }

func (a *googleAdapter) Delete(k int) bool {
	_, ok := a.t.Delete(k)
	return ok
}

func (a *googleAdapter) Get(k int) bool { _, ok := a.t.Get(k); return ok }
func (a *googleAdapter) Len() int        { return a.t.Len() }
func (a *googleAdapter) Reset()          { a.t = googlebtree.NewG[int](32, a.less) }

func (a *googleAdapter) Seek(k int) bool {
	var found bool
	a.t.AscendGreaterOrEqual(k, func(item int) bool { found = true; return false })
	return found
}

func (a *googleAdapter) Ascend(fn func(k int) bool) {
	a.t.Ascend(func(k int) bool { return fn(k) })
}

func (a *googleAdapter) Descend(fn func(k int) bool) {
	a.t.Descend(func(k int) bool { return fn(k) })
}

func (a *googleAdapter) AscendFrom(ge int, fn func(k int) bool) {
	a.t.AscendGreaterOrEqual(ge, func(k int) bool { return fn(k) })
}

func (a *googleAdapter) DescendFrom(le int, fn func(k int) bool) {
	a.t.DescendLessOrEqual(le, func(k int) bool { return fn(k) })
}

func (a *googleAdapter) AscendRange(ge, lt int, fn func(k int) bool) {
	a.t.AscendRange(ge, lt, func(k int) bool { return fn(k) })
}

func (a *googleAdapter) DescendRange(le, gt int, fn func(k int) bool) {
	a.t.DescendRange(le, gt, func(k int) bool { return fn(k) })
}

func (a *googleAdapter) Min() (int, bool) { return a.t.Min() }
func (a *googleAdapter) Max() (int, bool) { return a.t.Max() }

func (a *googleAdapter) DeleteMin() (int, bool) { return a.t.DeleteMin() }
func (a *googleAdapter) DeleteMax() (int, bool) { return a.t.DeleteMax() }

func (a *googleAdapter) Clone() BTree {
	clone := &googleAdapter{t: googlebtree.NewG[int](32, a.less), less: a.less}
	a.t.Ascend(func(k int) bool { clone.t.ReplaceOrInsert(k); return true })
	return clone
}

// ===== registry =====

var impls = []struct {
	name string
	new  func() BTree
}{
	{"ajwerner", newAjwerner},
	{"tidwall", newTidwall},
	{"google", newGoogle},
}

// ===== helpers (mirror btree_google_test.go, using BTree interface) =====

func perm(n int) []int { return rand.Perm(n) }

func rang(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func rangrev(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = n - 1 - i
	}
	return out
}

func all(t BTree) []int {
	var out []int
	t.Ascend(func(k int) bool { out = append(out, k); return true })
	return out
}

func allrev(t BTree) []int {
	var out []int
	t.Descend(func(k int) bool { out = append(out, k); return true })
	return out
}

func ascendRange(t BTree, from, to int, fn func(int) bool) { t.AscendRange(from, to, fn) }
func descendRange(t BTree, from, to int, fn func(int) bool) { t.DescendRange(from, to, fn) }

func ascendLessThan(t BTree, to int, fn func(int) bool) {
	t.Ascend(func(k int) bool {
		if k >= to {
			return false
		}
		return fn(k)
	})
}

func descendLessOrEqual(t BTree, to int, fn func(int) bool) { t.DescendFrom(to, fn) }
func ascendGreaterOrEqual(t BTree, from int, fn func(int) bool) { t.AscendFrom(from, fn) }

func descendGreaterThan(t BTree, from int, fn func(int) bool) {
	t.Descend(func(k int) bool {
		if k <= from {
			return false
		}
		return fn(k)
	})
}

// ===== tests =====

func forEachImpl(t *testing.T, fn func(t *testing.T, tr BTree)) {
	t.Helper()
	for _, impl := range impls {
		t.Run(impl.name, func(t *testing.T) { fn(t, impl.new()) })
	}
}

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
				tr.Insert(item)
			}
			for _, item := range perm(treeSize) {
				if !tr.Get(item) {
					t.Fatal("get did not find item", item)
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

// ===== benchmarks (ported from btree_google_test.go) =====

func BenchmarkInsert(b *testing.B) {
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
}

func BenchmarkSeek(b *testing.B) {
	const size = 100000
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, item := range perm(size) {
				tr.Insert(item)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tr.Seek(i % size)
			}
		})
	}
}

func BenchmarkDeleteInsert(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, item := range insertP {
				tr.Insert(item)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tr.Delete(insertP[i%benchmarkTreeSize])
				tr.Insert(insertP[i%benchmarkTreeSize])
			}
		})
	}
}

// BenchmarkDeleteInsertCloneOnce clones once then measures steady-state
// delete+insert. For ajwerner this triggers copy-on-write on first mutation;
// tidwall and google pay the full copy cost upfront in Clone().
func BenchmarkDeleteInsertCloneOnce(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, item := range insertP {
				tr.Insert(item)
			}
			tr = tr.Clone()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tr.Delete(insertP[i%benchmarkTreeSize])
				tr.Insert(insertP[i%benchmarkTreeSize])
			}
		})
	}
}

// BenchmarkDeleteInsertCloneEachTime clones before every delete+insert,
// measuring the combined cost of cloning and mutation per operation.
func BenchmarkDeleteInsertCloneEachTime(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, item := range insertP {
				tr.Insert(item)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tr = tr.Clone()
				tr.Delete(insertP[i%benchmarkTreeSize])
				tr.Insert(insertP[i%benchmarkTreeSize])
			}
		})
	}
}

func BenchmarkDelete(b *testing.B) {
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
}

func BenchmarkGet(b *testing.B) {
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
}

func BenchmarkAscend(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	sort.Ints(arr)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, v := range arr {
				tr.Insert(v)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
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
}

func BenchmarkDescend(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	sort.Ints(arr)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, v := range arr {
				tr.Insert(v)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
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
}

func BenchmarkAscendRange(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	sort.Ints(arr)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, v := range arr {
				tr.Insert(v)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
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
}

func BenchmarkDescendRange(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	sort.Ints(arr)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, v := range arr {
				tr.Insert(v)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
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
}

func BenchmarkAscendGreaterOrEqual(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	sort.Ints(arr)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, v := range arr {
				tr.Insert(v)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
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
}

func BenchmarkDescendLessOrEqual(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	sort.Ints(arr)
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			tr := impl.new()
			for _, v := range arr {
				tr.Insert(v)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
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
}
