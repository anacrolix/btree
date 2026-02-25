// Copyright 2018 The Cockroach Authors.
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

package abstract

// Iterator is responsible for search and traversal within a AugBTree.
type Iterator[K, V, A any] struct {
	r *Map[K, V, A]
	iterFrame[K, V, A]
	s iterStack[K, V, A]
}

func (i *Iterator[K, V, A]) lowLevel() *LowLevelIterator[K, V, A] {
	return (*LowLevelIterator[K, V, A])(i)
}

// Compare compares two keys using the same comparison function as the map.
func (i *Iterator[K, V, A]) Compare(a, b K) int {
	return i.r.cfg.cmp(a, b)
}

// Reset marks the iterator as invalid and clears any state.
func (i *Iterator[K, V, A]) Reset() {
	i.node = i.r.root
	i.pos = -1
	i.s.reset()
}

// SeekGE seeks to the first key greater-than or equal to the provided key.
//
// It uses a lazy descent: ancestor frames are not pushed to the stack during
// the walk, making SeekGE as fast as a bare pointer-chase lookup.  The cost
// of building the ancestor stack is deferred to the first leaf-boundary
// crossing in Next/Prev, keeping the hot path (iterations within a single
// leaf) completely free of extra overhead.
func (i *Iterator[K, V, A]) SeekGE(key K) {
	i.s.setLazy()
	i.node = i.r.root
	i.pos = -1
	if i.node == nil {
		return
	}
	for {
		pos, found := i.node.find(i.r.cfg.cmp, key)
		i.pos = int16(pos)
		if found {
			return
		}
		if i.node.IsLeaf() {
			if i.pos == i.node.count {
				// The key is greater than every key in this leaf; we need to
				// ascend to the parent, which requires the ancestor stack.
				// Fall back to the full seek for this rare edge case and
				// build the stack eagerly.
				i.seekGEFull(key)
			}
			return
		}
		// Lazy descent: follow the child without recording a frame.
		i.node = i.node.children[i.pos]
	}
}

// seekGEFull is SeekGE with eager ancestor-stack building.  It is used as a
// fallback for the rare edge case where SeekGE must ascend past a leaf and as
// the rebuild routine when Next/Prev first crosses a leaf boundary.
func (i *Iterator[K, V, A]) seekGEFull(key K) {
	i.node = i.r.root
	i.pos = -1
	i.s.reset() // clears the lazy flag
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	for {
		pos, found := i.node.find(i.r.cfg.cmp, key)
		i.pos = int16(pos)
		if found {
			return
		}
		if i.node.IsLeaf() {
			if i.pos == i.node.count {
				i.nextCore()
			}
			return
		}
		ll.Descend()
	}
}

// SeekLT seeks to the last key less-than the provided key.
//
// Uses a lazy descent for the common cases where the predecessor can be
// located without ascending the tree: when it is within the same leaf
// (pos > 0) or in the rightmost leaf of the left subtree at an internal
// match.  Falls back to a full-stack seek only when pos == 0 at a leaf,
// requiring an ascent.
func (i *Iterator[K, V, A]) SeekLT(key K) {
	i.s.setLazy()
	i.node = i.r.root
	i.pos = -1
	if i.node == nil {
		return
	}
	for {
		pos, found := i.node.find(i.r.cfg.cmp, key)
		i.pos = int16(pos)
		if found {
			if i.node.IsLeaf() {
				i.pos = int16(pos) - 1
				if i.pos < 0 {
					// Need to ascend; fall back to full seek.
					i.seekLTFull(key)
				}
				return
			}
			// Internal node match: predecessor is rightmost of child[pos].
			// Walk into that subtree without recording any frames; the stack
			// stays lazy for the subsequent Prev path.
			i.node = i.node.children[pos]
			for !i.node.IsLeaf() {
				i.node = i.node.children[i.node.count]
			}
			i.pos = i.node.count - 1
			return
		}
		if i.node.IsLeaf() {
			i.pos = int16(pos) - 1
			if i.pos < 0 {
				i.seekLTFull(key)
			}
			return
		}
		// Lazy descent.
		i.node = i.node.children[pos]
	}
}

// seekLTFull is SeekLT with eager ancestor-stack building.
func (i *Iterator[K, V, A]) seekLTFull(key K) {
	i.node = i.r.root
	i.pos = -1
	i.s.reset()
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	for {
		pos, found := i.node.find(i.r.cfg.cmp, key)
		i.pos = int16(pos)
		if found || i.node.IsLeaf() {
			i.prevCore()
			return
		}
		ll.Descend()
	}
}

// First seeks to the first key in the AugBTree.
func (i *Iterator[K, V, A]) First() {
	i.Reset()
	i.pos = 0
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	for !i.node.IsLeaf() {
		ll.Descend()
	}
	i.pos = 0
}

// Last seeks to the last key in the AugBTree.
func (i *Iterator[K, V, A]) Last() {
	i.Reset()
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	for !i.node.IsLeaf() {
		i.pos = i.node.count
		ll.Descend()
	}
	i.pos = i.node.count - 1
}

// Next positions the Iterator to the key immediately following its current
// position.
//
// When the iterator is in lazy mode (after SeekGE/SeekLT) and the next
// element lies within the same leaf node, no ancestor-stack work is needed.
// The stack is built only on the first leaf-boundary crossing, keeping the
// hot path (movements within a single leaf) free of any extra overhead.
func (i *Iterator[K, V, A]) Next() {
	if i.node == nil {
		return
	}
	if i.node.IsLeaf() {
		i.pos++
		if i.pos < i.node.count {
			return // hot path: still inside the same leaf, no stack needed
		}
		// We have walked off the end of this leaf and need to ascend.
		if i.s.lazy {
			// Ancestor stack was not built.  Re-seek to the last key in this
			// leaf to reconstruct the full stack, then advance past it.
			i.seekGEFull(i.node.keys[i.node.count-1])
			i.nextCore()
			return
		}
		ll := i.lowLevel()
		for i.s.len() > 0 && i.pos >= i.node.count {
			ll.Ascend()
		}
		return
	}
	// At an internal node (SeekGE found an exact match there).
	if i.s.lazy {
		i.seekGEFull(i.node.keys[i.pos])
		i.nextCore()
		return
	}
	i.nextCore()
}

// nextCore is the inner Next step; it assumes the ancestor stack is valid.
func (i *Iterator[K, V, A]) nextCore() {
	ll := i.lowLevel()
	if i.node.IsLeaf() {
		i.pos++
		if i.pos < i.node.count {
			return
		}
		for i.s.len() > 0 && i.pos >= i.node.count {
			ll.Ascend()
		}
		return
	}
	i.pos++
	ll.Descend()
	for !i.node.IsLeaf() {
		i.pos = 0
		ll.Descend()
	}
	i.pos = 0
}

// Prev positions the Iterator to the key immediately preceding its current
// position.
//
// Like Next, the lazy-mode check is only on the cold path (leaf-boundary
// crossing), keeping the hot path (within a single leaf) free of overhead.
func (i *Iterator[K, V, A]) Prev() {
	if i.node == nil {
		return
	}
	if i.node.IsLeaf() {
		i.pos--
		if i.pos >= 0 {
			return // hot path: still inside the same leaf
		}
		// Walked off the start of this leaf.
		if i.s.lazy {
			// Rebuild stack by re-seeking to the first key in this leaf, then
			// call prevCore which will ascend past it.
			i.seekGEFull(i.node.keys[0])
			i.prevCore()
			return
		}
		ll := i.lowLevel()
		for i.s.len() > 0 && i.pos < 0 {
			ll.Ascend()
			i.pos--
		}
		return
	}
	// At an internal node.
	if i.s.lazy {
		i.seekGEFull(i.node.keys[i.pos])
		i.prevCore()
		return
	}
	i.prevCore()
}

// prevCore is the inner Prev step; it assumes the ancestor stack is valid.
func (i *Iterator[K, V, A]) prevCore() {
	ll := i.lowLevel()
	if i.node.IsLeaf() {
		i.pos--
		if i.pos >= 0 {
			return
		}
		for i.s.len() > 0 && i.pos < 0 {
			ll.Ascend()
			i.pos--
		}
		return
	}
	ll.Descend()
	for !i.node.IsLeaf() {
		i.pos = i.node.count
		ll.Descend()
	}
	i.pos = i.node.count - 1
}

// Valid returns whether the Iterator is positioned at a valid position.
func (i *Iterator[K, V, A]) Valid() bool {
	return i.node != nil && i.pos >= 0 && i.pos < i.node.count
}

// Cur returns the key at the Iterator's current position. It is illegal
// to call Key if the Iterator is not valid.
func (i *Iterator[K, V, A]) Cur() K {
	return i.node.keys[i.pos]
}

// Value returns the value at the Iterator's current position. It is illegal
// to call Value if the Iterator is not valid.
func (i *Iterator[K, V, A]) Value() V {
	return i.node.values[i.pos]
}
