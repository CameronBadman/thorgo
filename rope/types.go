// package rope implements a skip list.

package rope

import (
	"iter"
)

// Info is a holder for info looked up in a Rope.
type Info[Id comparable, T any] struct {
	Id, Next, Prev Id
	DataLen[T]
}

// DataLen is a pair type.
type DataLen[T any] struct {
	Len  int
	Data T
}

type ropeLevel[Id comparable, T any] struct {
	next        *ropeNode[Id, T] // can be nil
	prev        *ropeNode[Id, T] // always set
	subtreesize int
}

type iterRef[Id comparable, T any] struct {
	count int
	node  *ropeNode[Id, T]
}

type ropeNode[Id comparable, T any] struct {
	id     Id
	dl     DataLen[T]
	levels []ropeLevel[Id, T]

	// if set, an iterator is chilling here for the next value
	iterRef *iterRef[Id, T]
}

type Removed[Id comparable, T any] struct {
	Id   Id
	Len  int
	Data T
}

type ropeImpl[Id comparable, T any] struct {
	head     ropeNode[Id, T]
	len      int
	byId     map[Id]*ropeNode[Id, T]
	height   int // matches len(head.levels)
	nodePool []*ropeNode[Id, T]
	lastId   Id
}

type Sizer interface {
	Len() int
}

// Rope is a skip list.
// It supports zero-length entries.
// It is not goroutine-safe.
// The zero Id is always part of the Rope and has zero length, don't use it to add items.
type Rope[Id comparable, T any] interface {
	DebugPrint()
	// Returns the total sum of the parts of the rope. O(1).
	Len() int
	// Returns the number of parts here. O(1).
	Count() int
	// Finds the position after the given Id.
	// This lookup costs ~O(logn).
	Find(id Id) int
	// Finds info on the given Id.
	// This lookup costs O(1).
	Info(id Id) Info[Id, T]
	// Finds the Id/info at the position in the Rope.
	// Returns the offset from the end of the Id.
	// This costs ~O(logn).
	// Either stops before or skips after zero-length content based on biasAfter.
	// e.g., with 0/false, this will always return the zero Id.
	ByPosition(position int, biasAfter bool) (id Id, offset int)
	// Between returns the distance between _after_ these two nodes.
	// This costs ~O(logn), and is more expensive than Compare.
	Between(afterA, afterB Id) (distance int, ok bool)
	// Compare the position of the two Id in this Rope.
	// Costs ~O(logn).
	Compare(a, b Id) (cmp int, ok bool)
	// Less determines if the first Id in this Rope before the other. For sorting.
	// Costs ~O(logn).
	Less(a, b Id) bool
	// Iter reads from after the given Id.
	// It is safe to use even if the Rope is modified.
	Iter(afterId Id) iter.Seq2[Id, DataLen[T]]
	// Splice performs insert, delete, or replace operations.
	// afterId: anchor point (nil = head/start)
	// deleteUntilId: if non-nil, delete nodes from afterId until this Id
	// newId: if non-nil, insert new node with given data
	// Returns removed nodes for undo support.
	// Costs ~O(logn+m), where m is the number of nodes being deleted.
	Splice(afterId Id, deleteUntilId *Id, insertId *Id, data T) (removed []Removed[Id, T], err error)
	// Insert adds a new entry after afterId. Convenience wrapper around Splice.
	Insert(afterId Id, newId Id, data T) error
	// Delete removes entries from after afterId until untilId. Convenience wrapper around Splice.
	Delete(afterId Id, untilId Id) ([]Removed[Id, T], error)
	// LastId returns the last Id in this rope.
	LastId() Id
}
