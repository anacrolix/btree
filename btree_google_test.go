// Copyright 2014 Google Inc.
// Portions Copyright 2021 Andrew Werner.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Ported from github.com/google/btree, adapted for the cursor-based iterator
// API and Go generics used by this package.

package btree

import (
	"cmp"
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"testing"
)

// perm returns a random permutation of n ints in the range [0, n).
func perm(n int) []int {
	return rand.Perm(n)
}

// rang returns an ordered list of ints in the range [0, n).
func rang(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// rangrev returns a reversed ordered list of ints in the range [0, n).
func rangrev(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = n - 1 - i
	}
	return out
}

// all extracts all items from a set in ascending order.
func all(t *Set[int]) []int {
	var out []int
	it := t.Iterator()
	it.First()
	for it.Valid() {
		out = append(out, it.Cur())
		it.Next()
	}
	return out
}

// allrev extracts all items from a set in descending order.
func allrev(t *Set[int]) []int {
	var out []int
	it := t.Iterator()
	it.Last()
	for it.Valid() {
		out = append(out, it.Cur())
		it.Prev()
	}
	return out
}

// treeHas returns whether item is present in the set.
func treeHas(t *Set[int], item int) bool {
	it := t.Iterator()
	it.SeekGE(item)
	return it.Valid() && it.Cur() == item
}

// treeMin returns the minimum item and true, or zero and false if empty.
func treeMin(t *Set[int]) (int, bool) {
	it := t.Iterator()
	it.First()
	if !it.Valid() {
		return 0, false
	}
	return it.Cur(), true
}

// treeMax returns the maximum item and true, or zero and false if empty.
func treeMax(t *Set[int]) (int, bool) {
	it := t.Iterator()
	it.Last()
	if !it.Valid() {
		return 0, false
	}
	return it.Cur(), true
}

// deleteMin removes and returns the minimum item.
func deleteMin(t *Set[int]) (int, bool) {
	it := t.Iterator()
	it.First()
	if !it.Valid() {
		return 0, false
	}
	v := it.Cur()
	t.Delete(v)
	return v, true
}

// deleteMax removes and returns the maximum item.
func deleteMax(t *Set[int]) (int, bool) {
	it := t.Iterator()
	it.Last()
	if !it.Valid() {
		return 0, false
	}
	v := it.Cur()
	t.Delete(v)
	return v, true
}

// ascendRange iterates over items in [from, to), calling fn for each.
func ascendRange(t *Set[int], from, to int, fn func(int) bool) {
	it := t.Iterator()
	it.SeekGE(from)
	for it.Valid() && it.Cur() < to {
		if !fn(it.Cur()) {
			return
		}
		it.Next()
	}
}

// descendRange iterates over items in (to, from] in descending order, calling fn for each.
func descendRange(t *Set[int], from, to int, fn func(int) bool) {
	it := t.Iterator()
	// Seek to last item <= from.
	it.SeekGE(from)
	if !it.Valid() {
		it.Last()
	} else if it.Cur() > from {
		it.Prev()
	}
	for it.Valid() && it.Cur() > to {
		if !fn(it.Cur()) {
			return
		}
		it.Prev()
	}
}

// ascendLessThan iterates over items in [min, to), calling fn for each.
func ascendLessThan(t *Set[int], to int, fn func(int) bool) {
	it := t.Iterator()
	it.First()
	for it.Valid() && it.Cur() < to {
		if !fn(it.Cur()) {
			return
		}
		it.Next()
	}
}

// descendLessOrEqual iterates over items in (-inf, to] in descending order.
func descendLessOrEqual(t *Set[int], to int, fn func(int) bool) {
	it := t.Iterator()
	// Seek to last item <= to.
	it.SeekGE(to)
	if !it.Valid() {
		it.Last()
	} else if it.Cur() > to {
		it.Prev()
	}
	for it.Valid() {
		if !fn(it.Cur()) {
			return
		}
		it.Prev()
	}
}

// ascendGreaterOrEqual iterates over items in [from, max], calling fn for each.
func ascendGreaterOrEqual(t *Set[int], from int, fn func(int) bool) {
	it := t.Iterator()
	it.SeekGE(from)
	for it.Valid() {
		if !fn(it.Cur()) {
			return
		}
		it.Next()
	}
}

// descendGreaterThan iterates over items in (from, max] in descending order.
func descendGreaterThan(t *Set[int], from int, fn func(int) bool) {
	it := t.Iterator()
	it.Last()
	for it.Valid() && it.Cur() > from {
		if !fn(it.Cur()) {
			return
		}
		it.Prev()
	}
}

func newSet() Set[int] {
	return MakeSet(cmp.Compare[int])
}

const treeSize = 10000

func TestBTreeFull(t *testing.T) {
	tr := newSet()
	for i := 0; i < 10; i++ {
		if _, ok := treeMin(&tr); ok {
			t.Fatal("empty tree should have no min")
		}
		if _, ok := treeMax(&tr); ok {
			t.Fatal("empty tree should have no max")
		}
		for _, item := range perm(treeSize) {
			_, overwrote := tr.Upsert(item)
			if overwrote {
				t.Fatal("insert found existing item", item)
			}
		}
		for _, item := range perm(treeSize) {
			if !treeHas(&tr, item) {
				t.Fatal("has did not find item", item)
			}
		}
		for _, item := range perm(treeSize) {
			_, overwrote := tr.Upsert(item)
			if !overwrote {
				t.Fatal("upsert of existing item should report overwrote", item)
			}
		}
		if min, ok := treeMin(&tr); !ok || min != 0 {
			t.Fatalf("min: want %v, got %v (ok=%v)", 0, min, ok)
		}
		if max, ok := treeMax(&tr); !ok || max != treeSize-1 {
			t.Fatalf("max: want %v, got %v (ok=%v)", treeSize-1, max, ok)
		}
		got := all(&tr)
		want := rang(treeSize)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ascending mismatch:\n got: %v\nwant: %v", got, want)
		}
		gotrev := allrev(&tr)
		wantrev := rangrev(treeSize)
		if !reflect.DeepEqual(gotrev, wantrev) {
			t.Fatalf("descending mismatch:\n got: %v\nwant: %v", gotrev, wantrev)
		}
		for _, item := range perm(treeSize) {
			if !tr.Delete(item) {
				t.Fatalf("delete did not find %v", item)
			}
		}
		if got = all(&tr); len(got) > 0 {
			t.Fatalf("tree not empty after deleting all: %v", got)
		}
	}
}

func TestDeleteMin(t *testing.T) {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	for v, ok := deleteMin(&tr); ok; v, ok = deleteMin(&tr) {
		got = append(got, v)
	}
	if want := rang(100); !reflect.DeepEqual(got, want) {
		t.Fatalf("deletemin:\n got: %v\nwant: %v", got, want)
	}
}

func TestDeleteMax(t *testing.T) {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	for v, ok := deleteMax(&tr); ok; v, ok = deleteMax(&tr) {
		got = append(got, v)
	}
	// deleteMax extracts in descending order; reverse to compare with rang.
	for i, j := 0, len(got)-1; i < j; i, j = i+1, j-1 {
		got[i], got[j] = got[j], got[i]
	}
	if want := rang(100); !reflect.DeepEqual(got, want) {
		t.Fatalf("deletemax:\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendRange(t *testing.T) {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	ascendRange(&tr, 40, 60, func(a int) bool {
		got = append(got, a)
		return true
	})
	if want := rang(100)[40:60]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	ascendRange(&tr, 40, 60, func(a int) bool {
		if a > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rang(100)[40:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange (early stop):\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendRange(t *testing.T) {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	descendRange(&tr, 60, 40, func(a int) bool {
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[39:59]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	descendRange(&tr, 60, 40, func(a int) bool {
		if a < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[39:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendrange (early stop):\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendLessThan(t *testing.T) {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	ascendLessThan(&tr, 60, func(a int) bool {
		got = append(got, a)
		return true
	})
	if want := rang(100)[:60]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendlessthan:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	ascendLessThan(&tr, 60, func(a int) bool {
		if a > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rang(100)[:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendlessthan (early stop):\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendLessOrEqual(t *testing.T) {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	descendLessOrEqual(&tr, 40, func(a int) bool {
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[59:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendlessorequal:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	descendLessOrEqual(&tr, 60, func(a int) bool {
		if a < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[39:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendlessorequal (early stop):\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendGreaterOrEqual(t *testing.T) {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	ascendGreaterOrEqual(&tr, 40, func(a int) bool {
		got = append(got, a)
		return true
	})
	if want := rang(100)[40:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendgreaterorequal:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	ascendGreaterOrEqual(&tr, 40, func(a int) bool {
		if a > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rang(100)[40:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendgreaterorequal (early stop):\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendGreaterThan(t *testing.T) {
	tr := MakeSet(cmp.Compare[int])
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	descendGreaterThan(&tr, 40, func(a int) bool {
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[:59]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendgreaterthan:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	descendGreaterThan(&tr, 40, func(a int) bool {
		if a < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendgreaterthan (early stop):\n got: %v\nwant: %v", got, want)
	}
}

const cloneTestSize = 10000

func cloneTest(t *testing.T, tr *Set[int], start int, p []int, wg *sync.WaitGroup, trees *[]*Set[int], mu *sync.Mutex) {
	t.Logf("Starting new clone at %v", start)
	mu.Lock()
	*trees = append(*trees, tr)
	mu.Unlock()
	for i := start; i < cloneTestSize; i++ {
		tr.Upsert(p[i])
		if i%(cloneTestSize/5) == 0 {
			clone := tr.Clone()
			wg.Add(1)
			go cloneTest(t, &clone, i+1, p, wg, trees, mu)
		}
	}
	wg.Done()
}

func TestCloneConcurrentOperations(t *testing.T) {
	tr := newSet()
	var trees []*Set[int]
	p := perm(cloneTestSize)
	var wg sync.WaitGroup
	wg.Add(1)
	go cloneTest(t, &tr, 0, p, &wg, &trees, &sync.Mutex{})
	wg.Wait()
	want := rang(cloneTestSize)
	t.Logf("Starting equality checks on %d trees", len(trees))
	for i, tree := range trees {
		if !reflect.DeepEqual(want, all(tree)) {
			t.Errorf("tree %v mismatch", i)
		}
	}
	t.Log("Removing half from first half of trees")
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
	t.Log("Checking all values again")
	for i, tree := range trees {
		var wantpart []int
		if i < len(trees)/2 {
			wantpart = want[:cloneTestSize/2]
		} else {
			wantpart = want
		}
		if got := all(tree); !reflect.DeepEqual(wantpart, got) {
			t.Errorf("tree %v mismatch, want %v got %v", i, len(wantpart), len(got))
		}
	}
}

// Corner case: operations on empty tree.
func TestEmptyTree(t *testing.T) {
	tr := newSet()
	if _, ok := treeMin(&tr); ok {
		t.Error("min of empty tree should return false")
	}
	if _, ok := treeMax(&tr); ok {
		t.Error("max of empty tree should return false")
	}
	if _, ok := deleteMin(&tr); ok {
		t.Error("deleteMin of empty tree should return false")
	}
	if _, ok := deleteMax(&tr); ok {
		t.Error("deleteMax of empty tree should return false")
	}
	if treeHas(&tr, 0) {
		t.Error("has on empty tree should return false")
	}
	var got []int
	ascendRange(&tr, 0, 10, func(a int) bool {
		got = append(got, a)
		return true
	})
	if len(got) != 0 {
		t.Errorf("ascendRange on empty tree returned %v", got)
	}
}

// Corner case: single-element tree.
func TestSingleElement(t *testing.T) {
	tr := newSet()
	tr.Upsert(42)
	if v, ok := treeMin(&tr); !ok || v != 42 {
		t.Errorf("min: got %v,%v want 42,true", v, ok)
	}
	if v, ok := treeMax(&tr); !ok || v != 42 {
		t.Errorf("max: got %v,%v want 42,true", v, ok)
	}
	if !treeHas(&tr, 42) {
		t.Error("has(42) returned false")
	}
	if treeHas(&tr, 0) {
		t.Error("has(0) on single-elem tree returned true")
	}
	if v, ok := deleteMin(&tr); !ok || v != 42 {
		t.Errorf("deleteMin: got %v,%v want 42,true", v, ok)
	}
	if tr.Len() != 0 {
		t.Errorf("tree should be empty after deleteMin, len=%d", tr.Len())
	}
}

// Corner case: seek to non-existent keys at boundaries.
func TestSeekBoundaries(t *testing.T) {
	tr := newSet()
	for _, v := range rang(10) {
		tr.Upsert(v)
	}
	// SeekGE past the end.
	it := tr.Iterator()
	it.SeekGE(100)
	if it.Valid() {
		t.Errorf("SeekGE(100) on [0..9] should be invalid, got %v", it.Cur())
	}
	// SeekLT before the beginning.
	it.SeekLT(0)
	if it.Valid() {
		t.Errorf("SeekLT(0) on [0..9] should be invalid, got %v", it.Cur())
	}
	// SeekGE at exact boundary.
	it.SeekGE(0)
	if !it.Valid() || it.Cur() != 0 {
		t.Errorf("SeekGE(0): got valid=%v cur=%v, want valid=true cur=0", it.Valid(), it.Cur())
	}
	it.SeekGE(9)
	if !it.Valid() || it.Cur() != 9 {
		t.Errorf("SeekGE(9): got valid=%v cur=%v, want valid=true cur=9", it.Valid(), it.Cur())
	}
	// SeekLT at exact boundary.
	it.SeekLT(10)
	if !it.Valid() || it.Cur() != 9 {
		t.Errorf("SeekLT(10): got valid=%v cur=%v, want valid=true cur=9", it.Valid(), it.Cur())
	}
	it.SeekLT(1)
	if !it.Valid() || it.Cur() != 0 {
		t.Errorf("SeekLT(1): got valid=%v cur=%v, want valid=true cur=0", it.Valid(), it.Cur())
	}
}

// Corner case: iterator bidirectional traversal.
func TestIteratorBidirectional(t *testing.T) {
	tr := newSet()
	for _, v := range rang(10) {
		tr.Upsert(v)
	}
	it := tr.Iterator()
	it.First()
	// Go forward 5 steps.
	for i := 0; i < 5; i++ {
		it.Next()
	}
	if it.Cur() != 5 {
		t.Fatalf("expected 5 after 5 Next, got %v", it.Cur())
	}
	// Go back 3 steps.
	for i := 0; i < 3; i++ {
		it.Prev()
	}
	if it.Cur() != 2 {
		t.Fatalf("expected 2 after 3 Prev, got %v", it.Cur())
	}
}

// Corner case: descend with missing keys (keys between inserts).
func TestDescendRangeGaps(t *testing.T) {
	tr := newSet()
	// Insert only even numbers 0..98.
	for i := 0; i < 100; i += 2 {
		tr.Upsert(i)
	}
	var got []int
	// descendRange from 55 (not present) to 40 (not present), exclusive.
	descendRange(&tr, 55, 40, func(a int) bool {
		got = append(got, a)
		return true
	})
	want := []int{54, 52, 50, 48, 46, 44, 42}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descendRange with gaps:\n got: %v\nwant: %v", got, want)
	}
}

const benchmarkTreeSize = 10000

func BenchmarkInsert(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	b.ResetTimer()
	i := 0
	for i < b.N {
		tr := newSet()
		for _, item := range insertP {
			tr.Upsert(item)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

func BenchmarkSeek(b *testing.B) {
	size := 100000
	tr := newSet()
	for _, item := range perm(size) {
		tr.Upsert(item)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		it.SeekGE(i % size)
		_ = it.Valid()
	}
}

func BenchmarkDeleteInsert(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	tr := newSet()
	for _, item := range insertP {
		tr.Upsert(item)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Delete(insertP[i%benchmarkTreeSize])
		tr.Upsert(insertP[i%benchmarkTreeSize])
	}
}

func BenchmarkDeleteInsertCloneOnce(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	tr := newSet()
	for _, item := range insertP {
		tr.Upsert(item)
	}
	tr = tr.Clone()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Delete(insertP[i%benchmarkTreeSize])
		tr.Upsert(insertP[i%benchmarkTreeSize])
	}
}

func BenchmarkDeleteInsertCloneEachTime(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	tr := newSet()
	for _, item := range insertP {
		tr.Upsert(item)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr = tr.Clone()
		tr.Delete(insertP[i%benchmarkTreeSize])
		tr.Upsert(insertP[i%benchmarkTreeSize])
	}
}

func BenchmarkDelete(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	removeP := perm(benchmarkTreeSize)
	i := 0
	for i < b.N {
		b.StopTimer()
		tr := newSet()
		for _, v := range insertP {
			tr.Upsert(v)
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
}

func BenchmarkGet(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	removeP := perm(benchmarkTreeSize)
	i := 0
	for i < b.N {
		b.StopTimer()
		tr := newSet()
		for _, v := range insertP {
			tr.Upsert(v)
		}
		b.StartTimer()
		for _, item := range removeP {
			treeHas(&tr, item)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

func BenchmarkAscend(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := newSet()
	for _, v := range arr {
		tr.Upsert(v)
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := 0
		it := tr.Iterator()
		it.First()
		for it.Valid() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: expected %v, got %v", arr[j], it.Cur())
			}
			j++
			it.Next()
		}
	}
}

func BenchmarkDescend(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := newSet()
	for _, v := range arr {
		tr.Upsert(v)
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := len(arr) - 1
		it := tr.Iterator()
		it.Last()
		for it.Valid() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: expected %v, got %v", arr[j], it.Cur())
			}
			j--
			it.Prev()
		}
	}
}

func BenchmarkAscendRange(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := newSet()
	for _, v := range arr {
		tr.Upsert(v)
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := 100
		it := tr.Iterator()
		it.SeekGE(100)
		for it.Valid() && it.Cur() < arr[len(arr)-100] {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: expected %v, got %v", arr[j], it.Cur())
			}
			j++
			it.Next()
		}
		if j != len(arr)-100 {
			b.Fatalf("expected %v, got %v", len(arr)-100, j)
		}
	}
}

func BenchmarkDescendRange(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := newSet()
	for _, v := range arr {
		tr.Upsert(v)
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := len(arr) - 100
		it := tr.Iterator()
		it.SeekGE(arr[len(arr)-100])
		if !it.Valid() {
			it.Last()
		} else if it.Cur() > arr[len(arr)-100] {
			it.Prev()
		}
		for it.Valid() && it.Cur() > 100 {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: expected %v, got %v", arr[j], it.Cur())
			}
			j--
			it.Prev()
		}
		if j != 100 {
			b.Fatalf("expected %v, got %v", 100, j)
		}
	}
}

func BenchmarkAscendGreaterOrEqual(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := newSet()
	for _, v := range arr {
		tr.Upsert(v)
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := 100
		k := 0
		it := tr.Iterator()
		it.SeekGE(100)
		for it.Valid() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: expected %v, got %v", arr[j], it.Cur())
			}
			j++
			k++
			it.Next()
		}
		if j != len(arr) {
			b.Fatalf("expected %v, got %v", len(arr), j)
		}
		if k != len(arr)-100 {
			b.Fatalf("expected %v, got %v", len(arr)-100, k)
		}
	}
}

func BenchmarkDescendLessOrEqual(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := newSet()
	for _, v := range arr {
		tr.Upsert(v)
	}
	sort.Ints(arr)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := len(arr) - 100
		k := len(arr)
		pivot := arr[len(arr)-100]
		it := tr.Iterator()
		it.SeekGE(pivot)
		if !it.Valid() {
			it.Last()
		} else if it.Cur() > pivot {
			it.Prev()
		}
		for it.Valid() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: expected %v, got %v", arr[j], it.Cur())
			}
			j--
			k--
			it.Prev()
		}
		if j != -1 {
			b.Fatalf("expected %v, got %v", -1, j)
		}
		if k != 99 {
			b.Fatalf("expected %v, got %v", 99, k)
		}
	}
}