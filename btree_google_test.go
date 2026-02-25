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

func all(t *Set[int]) []int {
	var out []int
	it := t.Iterator()
	for it.First(); it.Valid(); it.Next() {
		out = append(out, it.Cur())
	}
	return out
}

func allrev(t *Set[int]) []int {
	var out []int
	it := t.Iterator()
	for it.Last(); it.Valid(); it.Prev() {
		out = append(out, it.Cur())
	}
	return out
}

func treeHas(t *Set[int], item int) bool {
	it := t.Iterator()
	it.SeekGE(item)
	return it.Valid() && it.Cur() == item
}

func treeMin(t *Set[int]) (int, bool) {
	it := t.Iterator()
	it.First()
	if !it.Valid() {
		return 0, false
	}
	return it.Cur(), true
}

func treeMax(t *Set[int]) (int, bool) {
	it := t.Iterator()
	it.Last()
	if !it.Valid() {
		return 0, false
	}
	return it.Cur(), true
}

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

// descendRange iterates (to, from] descending (mirrors google/btree DescendRange semantics).
func descendRange(t *Set[int], from, to int, fn func(int) bool) {
	it := t.Iterator()
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

func ascendLessThan(t *Set[int], to int, fn func(int) bool) {
	it := t.Iterator()
	for it.First(); it.Valid() && it.Cur() < to; it.Next() {
		if !fn(it.Cur()) {
			return
		}
	}
}

func descendLessOrEqual(t *Set[int], to int, fn func(int) bool) {
	it := t.Iterator()
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

func ascendGreaterOrEqual(t *Set[int], from int, fn func(int) bool) {
	it := t.Iterator()
	for it.SeekGE(from); it.Valid(); it.Next() {
		if !fn(it.Cur()) {
			return
		}
	}
}

func descendGreaterThan(t *Set[int], from int, fn func(int) bool) {
	it := t.Iterator()
	for it.Last(); it.Valid() && it.Cur() > from; it.Prev() {
		if !fn(it.Cur()) {
			return
		}
	}
}

func newSet() Set[int] { return MakeSet(cmp.Compare[int]) }

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
			if _, overwrote := tr.Upsert(item); overwrote {
				t.Fatal("insert found existing item", item)
			}
		}
		for _, item := range perm(treeSize) {
			if !treeHas(&tr, item) {
				t.Fatal("has did not find item", item)
			}
		}
		for _, item := range perm(treeSize) {
			if _, overwrote := tr.Upsert(item); !overwrote {
				t.Fatal("upsert of existing item should report overwrote", item)
			}
		}
		if min, ok := treeMin(&tr); !ok || min != 0 {
			t.Fatalf("min: want 0, got %v (ok=%v)", min, ok)
		}
		if max, ok := treeMax(&tr); !ok || max != treeSize-1 {
			t.Fatalf("max: want %v, got %v (ok=%v)", treeSize-1, max, ok)
		}
		if got, want := all(&tr), rang(treeSize); !reflect.DeepEqual(got, want) {
			t.Fatalf("ascending mismatch:\n got: %v\nwant: %v", got, want)
		}
		if got, want := allrev(&tr), rangrev(treeSize); !reflect.DeepEqual(got, want) {
			t.Fatalf("descending mismatch:\n got: %v\nwant: %v", got, want)
		}
		for _, item := range perm(treeSize) {
			if !tr.Delete(item) {
				t.Fatalf("delete did not find %v", item)
			}
		}
		if got := all(&tr); len(got) > 0 {
			t.Fatalf("tree not empty after deleting all: %v", got)
		}
	}
}

func TestDeleteMin(t *testing.T) {
	tr := newSet()
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
	tr := newSet()
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	for v, ok := deleteMax(&tr); ok; v, ok = deleteMax(&tr) {
		got = append(got, v)
	}
	for i, j := 0, len(got)-1; i < j; i, j = i+1, j-1 {
		got[i], got[j] = got[j], got[i]
	}
	if want := rang(100); !reflect.DeepEqual(got, want) {
		t.Fatalf("deletemax:\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendRange(t *testing.T) {
	tr := newSet()
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	ascendRange(&tr, 40, 60, func(a int) bool { got = append(got, a); return true })
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
		t.Fatalf("ascendrange early-stop:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendRange(t *testing.T) {
	tr := newSet()
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	descendRange(&tr, 60, 40, func(a int) bool { got = append(got, a); return true })
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
		t.Fatalf("descendrange early-stop:\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendLessThan(t *testing.T) {
	tr := newSet()
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	ascendLessThan(&tr, 60, func(a int) bool { got = append(got, a); return true })
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
		t.Fatalf("ascendlessthan early-stop:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendLessOrEqual(t *testing.T) {
	tr := newSet()
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	descendLessOrEqual(&tr, 40, func(a int) bool { got = append(got, a); return true })
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
		t.Fatalf("descendlessorequal early-stop:\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendGreaterOrEqual(t *testing.T) {
	tr := newSet()
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	ascendGreaterOrEqual(&tr, 40, func(a int) bool { got = append(got, a); return true })
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
		t.Fatalf("ascendgreaterorequal early-stop:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendGreaterThan(t *testing.T) {
	tr := newSet()
	for _, v := range perm(100) {
		tr.Upsert(v)
	}
	var got []int
	descendGreaterThan(&tr, 40, func(a int) bool { got = append(got, a); return true })
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
		t.Fatalf("descendgreaterthan early-stop:\n got: %v\nwant: %v", got, want)
	}
}

const cloneTestSize = 10000

func cloneTest(t *testing.T, tr *Set[int], start int, p []int, wg *sync.WaitGroup, trees *[]*Set[int], mu *sync.Mutex) {
	t.Logf("Starting clone at %v", start)
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
}

// --- Corner cases ---

func TestEmptyTree(t *testing.T) {
	tr := newSet()
	if _, ok := treeMin(&tr); ok {
		t.Error("min of empty tree")
	}
	if _, ok := treeMax(&tr); ok {
		t.Error("max of empty tree")
	}
	if _, ok := deleteMin(&tr); ok {
		t.Error("deleteMin of empty tree")
	}
	if _, ok := deleteMax(&tr); ok {
		t.Error("deleteMax of empty tree")
	}
	if treeHas(&tr, 0) {
		t.Error("has on empty tree")
	}
	var got []int
	ascendRange(&tr, 0, 10, func(a int) bool { got = append(got, a); return true })
	if len(got) != 0 {
		t.Errorf("ascendRange on empty tree: %v", got)
	}
}

func TestSingleElement(t *testing.T) {
	tr := newSet()
	tr.Upsert(42)
	if v, ok := treeMin(&tr); !ok || v != 42 {
		t.Errorf("min: got %v,%v", v, ok)
	}
	if v, ok := treeMax(&tr); !ok || v != 42 {
		t.Errorf("max: got %v,%v", v, ok)
	}
	if !treeHas(&tr, 42) {
		t.Error("has(42) = false")
	}
	if treeHas(&tr, 0) {
		t.Error("has(0) on single-elem tree = true")
	}
	if v, ok := deleteMin(&tr); !ok || v != 42 {
		t.Errorf("deleteMin: got %v,%v", v, ok)
	}
	if tr.Len() != 0 {
		t.Errorf("len after deleteMin: %d", tr.Len())
	}
}

func TestSeekBoundaries(t *testing.T) {
	tr := newSet()
	for _, v := range rang(10) {
		tr.Upsert(v)
	}
	it := tr.Iterator()

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
}

func TestIteratorBidirectional(t *testing.T) {
	tr := newSet()
	for _, v := range rang(10) {
		tr.Upsert(v)
	}
	it := tr.Iterator()
	it.First()
	for i := 0; i < 5; i++ {
		it.Next()
	}
	if it.Cur() != 5 {
		t.Fatalf("after 5 Next: want 5, got %v", it.Cur())
	}
	for i := 0; i < 3; i++ {
		it.Prev()
	}
	if it.Cur() != 2 {
		t.Fatalf("after 3 Prev: want 2, got %v", it.Cur())
	}
}

// TestDescendRangeGaps checks descent when neither endpoint is in the tree.
func TestDescendRangeGaps(t *testing.T) {
	tr := newSet()
	for i := 0; i < 100; i += 2 {
		tr.Upsert(i)
	}
	var got []int
	descendRange(&tr, 55, 40, func(a int) bool { got = append(got, a); return true })
	want := []int{54, 52, 50, 48, 46, 44, 42}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descendRange gaps:\n got: %v\nwant: %v", got, want)
	}
}

// --- Benchmarks (all from google/btree) ---

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
	const size = 100000
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
		for it.First(); it.Valid(); it.Next() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: want %v got %v", arr[j], it.Cur())
			}
			j++
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
		for it.Last(); it.Valid(); it.Prev() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: want %v got %v", arr[j], it.Cur())
			}
			j--
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
		hi := arr[len(arr)-100]
		it := tr.Iterator()
		for it.SeekGE(100); it.Valid() && it.Cur() < hi; it.Next() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: want %v got %v", arr[j], it.Cur())
			}
			j++
		}
		if j != len(arr)-100 {
			b.Fatalf("j: want %v got %v", len(arr)-100, j)
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
		pivot := arr[len(arr)-100]
		it := tr.Iterator()
		it.SeekGE(pivot)
		if !it.Valid() {
			it.Last()
		} else if it.Cur() > pivot {
			it.Prev()
		}
		for ; it.Valid() && it.Cur() > 100; it.Prev() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: want %v got %v", arr[j], it.Cur())
			}
			j--
		}
		if j != 100 {
			b.Fatalf("j: want %v got %v", 100, j)
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
		j, k := 100, 0
		it := tr.Iterator()
		for it.SeekGE(100); it.Valid(); it.Next() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: want %v got %v", arr[j], it.Cur())
			}
			j++
			k++
		}
		if j != len(arr) {
			b.Fatalf("j: want %v got %v", len(arr), j)
		}
		if k != len(arr)-100 {
			b.Fatalf("k: want %v got %v", len(arr)-100, k)
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
		for ; it.Valid(); it.Prev() {
			if it.Cur() != arr[j] {
				b.Fatalf("mismatch: want %v got %v", arr[j], it.Cur())
			}
			j--
			k--
		}
		if j != -1 {
			b.Fatalf("j: want -1 got %v", j)
		}
		if k != 99 {
			b.Fatalf("k: want 99 got %v", k)
		}
	}
}
