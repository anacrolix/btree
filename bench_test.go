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

import (
	"cmp"
	"fmt"
	"testing"
)

var benchSizes = []int{100, 1_000, 10_000, 100_000}

func BenchmarkGetBySize(b *testing.B) {
	for _, n := range benchSizes {
		m := MakeMap[int, int](cmp.Compare)
		for i := range n {
			m.Upsert(i, i)
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for i := range n {
					_, _ = m.Get(i)
				}
			}
		})
	}
}

// BenchmarkUpsertBySize measures insert throughput. In erigon, trees are
// rebuilt on each block (txpool pending set, dirty file lists in db/state) so
// construction cost matters as much as steady-state lookup cost.
func BenchmarkUpsertBySize(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				m := MakeMap[int, int](cmp.Compare)
				for i := range n {
					m.Upsert(i, i)
				}
			}
		})
	}
}

// BenchmarkDeleteBySize measures deletion cost. The txpool removes confirmed
// transactions after each block, which can touch O(txns-per-block) entries in
// BySenderAndNonce. Deletions use Clone first to isolate the mutation from
// concurrent readers, mirroring the actual access pattern.
func BenchmarkDeleteBySize(b *testing.B) {
	for _, n := range benchSizes {
		m := MakeMap[int, int](cmp.Compare)
		for i := range n {
			m.Upsert(i, i)
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				mc := m.Clone()
				for i := range n {
					mc.Delete(i)
				}
			}
		})
	}
}

// BenchmarkIterateAscBySize measures full ascending traversal. This is the
// dominant access pattern in db/state (dirty file lists, merge decisions) and
// in versionmap (scanning all write versions for a key). Allocation-free
// iteration is important because these scans occur on the critical path of
// block execution.
func BenchmarkIterateAscBySize(b *testing.B) {
	for _, n := range benchSizes {
		m := MakeMap[int, int](cmp.Compare)
		for i := range n {
			m.Upsert(i, i)
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				it := m.Iterator()
				for it.First(); it.Valid(); it.Next() {
					_ = it.Cur()
					_ = it.Value()
				}
			}
		})
	}
}

// BenchmarkIterateDescBySize measures full descending traversal. The txpool
// uses Prev() when backtracking past a seek overshoot (SeekGE followed by
// Prev), and body/header download structures walk backwards to evict the
// oldest entries first.
func BenchmarkIterateDescBySize(b *testing.B) {
	for _, n := range benchSizes {
		m := MakeMap[int, int](cmp.Compare)
		for i := range n {
			m.Upsert(i, i)
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				it := m.Iterator()
				for it.Last(); it.Valid(); it.Prev() {
					_ = it.Cur()
					_ = it.Value()
				}
			}
		})
	}
}

// BenchmarkSeekGEBySize measures point-seek cost with a simple integer key.
// In versionmap (execution/state), SeekGE is used to find the nearest write
// version for a given txn index, and in kvcache each cache lookup does a seek
// to locate the closest entry for a storage key.
func BenchmarkSeekGEBySize(b *testing.B) {
	for _, n := range benchSizes {
		m := MakeMap[int, int](cmp.Compare)
		for i := range n {
			m.Upsert(i, i)
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				it := m.Iterator()
				for i := range n {
					it.SeekGE(i)
					_ = it.Cur()
				}
			}
		})
	}
}

// BenchmarkCloneBySize measures the cost of a clone followed by one mutation,
// which is the minimum CoW unit. In erigon, db/state clones dirty-file maps
// before each merge pass so the original view remains visible to concurrent
// readers. The clone itself is O(1), but the first write forces a node copy;
// this benchmark captures that combined cost.
func BenchmarkCloneBySize(b *testing.B) {
	for _, n := range benchSizes {
		m := MakeMap[int, int](cmp.Compare)
		for i := range n {
			m.Upsert(i, i)
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				c := m.Clone()
				c.Upsert(n, n) // trigger copy-on-write
			}
		})
	}
}

// txnKey models the key used in txpool's BySenderAndNonce map
// (Map[*metaTxn, *metaTxn]). Transactions are ordered first by sender address
// ID then by nonce so that all pending txns for a sender form a contiguous
// range that can be walked with a single seek + forward scan.
type txnKey struct {
	sender uint64
	nonce  uint64
}

func txnKeyCmp(a, b txnKey) int {
	if c := cmp.Compare(a.sender, b.sender); c != 0 {
		return c
	}
	return cmp.Compare(a.nonce, b.nonce)
}

// BenchmarkStructKeyUpsertBySize measures inserts with a two-field struct key
// and a custom multi-field comparator, matching the txpool pattern. The
// comparator has a branch on the first field, which is representative of real
// comparison functions in erigon and stresses the inlining budget differently
// than a single cmp.Compare call.
func BenchmarkStructKeyUpsertBySize(b *testing.B) {
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				m := MakeMap[txnKey, struct{}](txnKeyCmp)
				for i := range n {
					m.Upsert(txnKey{sender: uint64(i % 100), nonce: uint64(i)}, struct{}{})
				}
			}
		})
	}
}

// BenchmarkStructKeySeekGEBySize reflects the txpool's per-block processing
// loop: for each sender, seek to its lowest-nonce pending txn and walk forward
// through all of its txns in nonce order. The 100-sender distribution matches
// a realistic mempool where many senders each have a handful of queued txns.
func BenchmarkStructKeySeekGEBySize(b *testing.B) {
	for _, n := range benchSizes {
		m := MakeMap[txnKey, struct{}](txnKeyCmp)
		for i := range n {
			m.Upsert(txnKey{sender: uint64(i % 100), nonce: uint64(i)}, struct{}{})
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				it := m.Iterator()
				// seek to first txn for each of the 100 senders
				for s := range uint64(100) {
					it.SeekGE(txnKey{sender: s, nonce: 0})
					for it.Valid() && it.Cur().sender == s {
						it.Next()
					}
				}
			}
		})
	}
}
