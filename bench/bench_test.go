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
	"sort"
	"testing"

	ajwernerbtree "github.com/anacrolix/btree"
	googlebtree "github.com/google/btree"
	tidwallbtree "github.com/tidwall/btree"
)

const (
	treeSize               = 10000
	cloneTestSize          = 10000
	benchmarkTreeSize      = 10000
	benchmarkTreeSizeLarge = 1_000_000
)

// BTree is the common interface satisfied by all three adapters.
type BTree interface {
	Insert(k int)
	Upsert(k int) (overwrote bool) // insert or update; reports whether the key already existed
	Delete(k int) bool
	Get(k int) bool
	Len() int
	Reset()
	Clone() BTree     // ajwerner uses O(1) lazy clone; others perform a full copy
	NewCursor() Cursor // ajwerner uses a native iterator; others use a snapshot slice
	Seek(k int) bool   // true if any item >= k exists
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

// Cursor supports bidirectional navigation and seeking within a tree.
// ajwerner's implementation is a live iterator; tidwall's and google's are
// snapshot-backed (populated at NewCursor time) since their APIs are
// callback-based with no native bidirectional cursor.
type Cursor interface {
	First()
	Last()
	SeekGE(k int) // position at first item >= k
	SeekLT(k int) // position at last item < k
	Next()
	Prev()
	Valid() bool
	Cur() int
}

// ===== ajwerner/btree adapter =====

type ajwernerAdapter struct {
	m ajwernerbtree.Map[int, struct{}]
}

func newAjwerner() BTree {
	return &ajwernerAdapter{m: ajwernerbtree.MakeMap[int, struct{}](cmp.Compare[int])}
}

func (a *ajwernerAdapter) Insert(k int) { a.m.Upsert(k, struct{}{}) }

func (a *ajwernerAdapter) Upsert(k int) (overwrote bool) {
	_, _, overwrote = a.m.Upsert(k, struct{}{})
	return overwrote
}

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

func (a *ajwernerAdapter) NewCursor() Cursor {
	it := a.m.Iterator()
	return &ajwernerCursor{it: it}
}

type ajwernerCursor struct {
	it ajwernerbtree.MapIterator[int, struct{}]
}

func (c *ajwernerCursor) First()       { c.it.First() }
func (c *ajwernerCursor) Last()        { c.it.Last() }
func (c *ajwernerCursor) SeekGE(k int) { c.it.SeekGE(k) }
func (c *ajwernerCursor) SeekLT(k int) { c.it.SeekLT(k) }
func (c *ajwernerCursor) Next()        { c.it.Next() }
func (c *ajwernerCursor) Prev()        { c.it.Prev() }
func (c *ajwernerCursor) Valid() bool  { return c.it.Valid() }
func (c *ajwernerCursor) Cur() int     { return c.it.Cur() }

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

// sliceCursor is a snapshot-backed cursor for trees without a native
// bidirectional iterator. Items are collected at construction time.
type sliceCursor struct {
	items []int
	pos   int // valid range: [0, len-1]; -1 = before-first; len = after-last
}

func newSliceCursor(items []int) *sliceCursor {
	return &sliceCursor{items: items, pos: -1}
}

func (c *sliceCursor) First() { c.pos = 0 }
func (c *sliceCursor) Last()  { c.pos = len(c.items) - 1 }

func (c *sliceCursor) SeekGE(k int) {
	c.pos = sort.SearchInts(c.items, k) // first index where items[pos] >= k
}

func (c *sliceCursor) SeekLT(k int) {
	c.pos = sort.SearchInts(c.items, k) - 1 // last index where items[pos] < k
}

func (c *sliceCursor) Next() {
	if c.pos < len(c.items) {
		c.pos++
	}
}

func (c *sliceCursor) Prev() {
	if c.pos > -1 {
		c.pos--
	}
}

func (c *sliceCursor) Valid() bool { return c.pos >= 0 && c.pos < len(c.items) }
func (c *sliceCursor) Cur() int    { return c.items[c.pos] }

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

func (a *tidwallAdapter) Upsert(k int) (overwrote bool) {
	_, overwrote = a.m.Get(k)
	a.m.Set(k, struct{}{})
	return overwrote
}

func (a *tidwallAdapter) NewCursor() Cursor {
	var items []int
	a.m.Scan(func(k int, _ struct{}) bool { items = append(items, k); return true })
	return newSliceCursor(items)
}

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

func (a *googleAdapter) Upsert(k int) (overwrote bool) {
	_, overwrote = a.t.ReplaceOrInsert(k)
	return overwrote
}

func (a *googleAdapter) NewCursor() Cursor {
	var items []int
	a.t.Ascend(func(k int) bool { items = append(items, k); return true })
	return newSliceCursor(items)
}

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

func forEachImpl(t *testing.T, fn func(t *testing.T, tr BTree)) {
	t.Helper()
	for _, impl := range impls {
		t.Run(impl.name, func(t *testing.T) { fn(t, impl.new()) })
	}
}
