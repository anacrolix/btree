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

package btree

import "github.com/anacrolix/btree/internal/abstract"

// Map is a ordered map from K to V.
type Map[K, V any] struct {
	abstract.Map[K, V, struct{}]
}

// MakeMap constructs a new Map with the provided comparison function.
func MakeMap[K, V any](cmp func(K, K) int) Map[K, V] {
	return Map[K, V]{
		Map: abstract.MakeMap[K, V, struct{}](cmp, nil),
	}
}

// Clone clones the Map, lazily. It does so in constant time.
func (m *Map[K, V]) Clone() Map[K, V] {
	return Map[K, V]{Map: m.Map.Clone()}
}

// Set is an ordered set of items of type T.
type Set[T any] Map[T, struct{}]

// MakeSet constructs a new Set with the provided comparison function.
func MakeSet[T any](cmp func(T, T) int) Set[T] {
	return (Set[T])(MakeMap[T, struct{}](cmp))
}

// Clone clones the Set, lazily. It does so in constant time.
func (t *Set[T]) Clone() Set[T] {
	return (Set[T])((*Map[T, struct{}])(t).Clone())
}

// Upsert inserts or updates the provided item. It returns
// the overwritten item if a previous value existed for the key.
func (t *Set[T]) Upsert(item T) (replaced T, overwrote bool) {
	replaced, _, overwrote = t.Map.Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the value with the provided key. It returns true if the
// item existed in the set.
func (t *Set[K]) Delete(item K) (removed bool) {
	_, _, removed = t.Map.Delete(item)
	return removed
}

// Ascend calls fn for each key-value pair in ascending order.
// Iteration stops when fn returns false.
func (m *Map[K, V]) Ascend(fn func(K, V) bool) { m.Map.Ascend(fn) }

// AscendFrom calls fn for each key-value pair with key >= ge, in ascending order.
// Iteration stops when fn returns false.
func (m *Map[K, V]) AscendFrom(ge K, fn func(K, V) bool) { m.Map.AscendFrom(ge, fn) }

// AscendRange calls fn for each key-value pair with ge <= key < lt, in ascending order.
// Iteration stops when fn returns false.
func (m *Map[K, V]) AscendRange(ge, lt K, fn func(K, V) bool) { m.Map.AscendRange(ge, lt, fn) }

// Descend calls fn for each key-value pair in descending order.
// Iteration stops when fn returns false.
func (m *Map[K, V]) Descend(fn func(K, V) bool) { m.Map.Descend(fn) }

// DescendFrom calls fn for each key-value pair with key <= le, in descending order.
// Iteration stops when fn returns false.
func (m *Map[K, V]) DescendFrom(le K, fn func(K, V) bool) { m.Map.DescendFrom(le, fn) }

// DescendRange calls fn for each key-value pair with gt < key <= le, in descending order.
// Iteration stops when fn returns false.
func (m *Map[K, V]) DescendRange(le, gt K, fn func(K, V) bool) { m.Map.DescendRange(le, gt, fn) }

// Min returns the minimum key-value pair in the map, if any.
func (m *Map[K, V]) Min() (K, V, bool) { return m.Map.Min() }

// Max returns the maximum key-value pair in the map, if any.
func (m *Map[K, V]) Max() (K, V, bool) { return m.Map.Max() }

// Ascend calls fn for each item in ascending order.
// Iteration stops when fn returns false.
func (t *Set[T]) Ascend(fn func(T) bool) {
	(*Map[T, struct{}])(t).Ascend(func(k T, _ struct{}) bool { return fn(k) })
}

// Min returns the minimum item in the set, if any.
func (t *Set[T]) Min() (T, bool) {
	k, _, ok := (*Map[T, struct{}])(t).Min()
	return k, ok
}

// Max returns the maximum item in the set, if any.
func (t *Set[T]) Max() (T, bool) {
	k, _, ok := (*Map[T, struct{}])(t).Max()
	return k, ok
}

// MapIterator is an iterator for a Map.
type MapIterator[K, V any] = abstract.Iterator[K, V, struct{}]

// SetIterator is an iterator for a Set.
type SetIterator[T any] = MapIterator[T, struct{}]
