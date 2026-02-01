package rope

import (
	"errors"
	"fmt"
	"iter"
	"log"
	"strings"
)

const (
	poolSize  = 8
	maxHeight = 32
)

// NewRoot builds a new Rope[Id, T] with a given root value for the zero ID.
func NewRoot[Id comparable, T any](root T) Rope[Id, T] {
	out := &ropeImpl[Id, T]{
		byId:     map[Id]*ropeNode[Id, T]{},
		height:   1,
		nodePool: make([]*ropeNode[Id, T], 0, poolSize),
	}
	out.head.dl.Data = root

	var zeroId Id
	out.byId[zeroId] = &out.head
	out.head.levels = make([]ropeLevel[Id, T], 1, maxHeight) // never alloc again
	out.head.levels[0] = ropeLevel[Id, T]{prev: &out.head}
	return out
}

var (
		ErrBadAnchor = errors.New("invalid anchor id")
		ErrIdExists  = errors.New("id already exists")
		ErrNegativeLength = errors.New("length must be positive")	
)

// New builds a new Rope[Id, T].
func New[Id comparable, T Sizer]() Rope[Id, T] {
	var root T
	return NewRoot[Id](root)
}



func (r *ropeImpl[Id, T]) DebugPrint() {
	log.Printf("> rope len=%d heads=%d", r.len, r.height)
	const pipePart = "|     "
	const blankPart = "      "

	curr := &r.head
	renderHeight := r.height

	for {
		var parts []string

		// add level parts
		for i, l := range curr.levels {
			key := "+"
			if l.next == nil {
				key = "*"
				renderHeight = min(i, renderHeight)
			}

			s := fmt.Sprintf("%s%-5d", key, l.subtreesize)
			parts = append(parts, s)
		}

		// add blank/pipe parts
		for j := len(curr.levels); j < r.height; j++ {
			part := pipePart
			if j >= renderHeight {
				part = blankPart
			}
			parts = append(parts, part)
		}

		// add actual data
		parts = append(parts, fmt.Sprintf("id=%v", curr.id))
    parts = append(parts, fmt.Sprintf("%v", curr.dl.Data))

		// render
		log.Printf("- %s", strings.Join(parts, ""))

		// move to next
		curr = curr.levels[0].next
		if curr == nil {
			break
		}

		// render lines to break up the entries
		var lineParts []string
		for range renderHeight {
			lineParts = append(lineParts, pipePart)
		}
		log.Printf("  %s", strings.Join(lineParts, ""))

	}
}


func (r *ropeImpl[Id, T]) Len() int {
	return r.len
}

func (r *ropeImpl[Id, T]) Count() int {
	return len(r.byId) - 1
}

func (r *ropeImpl[Id, T]) Find(id Id) int {
	e := r.byId[id]
	if e == nil {
		return -1
	}

	node := e
	var pos int

	for node != &r.head {
		link := len(node.levels) - 1
		node = node.levels[link].prev
		pos += node.levels[link].subtreesize
	}

	return pos + e.dl.Len
}

func (r *ropeImpl[Id, T]) Info(id Id) (out Info[Id, T]) {
	node := r.byId[id]
	if node == nil {
		return
	}

	out.DataLen = node.dl
	out.Id = node.id

	ol := &node.levels[0]
	out.Prev = ol.prev.id // we always have prev
	if ol.next != nil {
		out.Next = ol.next.id
	}
	return out
}

func (r *ropeImpl[Id, T]) ByPosition(position int, biasAfter bool) (id Id, offset int) {
	if position < 0 || (!biasAfter && position == 0) {
		return
	} else if position > r.len || (biasAfter && position == r.len) {
		return r.lastId, 0
	}

	e := &r.head
outer:
	for h := r.height - 1; h >= 0; h-- {
		// traverse this height while we can
		for position > e.levels[h].subtreesize {
			position -= e.levels[h].subtreesize

			next := e.levels[h].next
			if next == nil {
				continue outer
			}
			e = next
		}

		// if we bias to end, move as far forward as possible (even zero)
		for biasAfter && position >= e.levels[h].subtreesize && e.levels[h].next != nil {
			position -= e.levels[h].subtreesize
			e = e.levels[h].next
		}
	}

	return e.id, e.dl.Len - position

	// return e.levels[0].next.id, e.dl.Len - position
}

func (r *ropeImpl[Id, T]) Insert(afterId Id, newId Id, data T) error {
	_, err := r.Splice(afterId, nil, &newId, data)
	return err
}

func (r *ropeImpl[Id, T]) Delete(afterId Id, untilId Id) ([]Removed[Id, T], error) {
	// We pass nil for insertId and a zero-value/empty T for data
	return r.Splice(afterId, &untilId, nil, *new(T))
}

func (r *ropeImpl[Id, T]) Splice(
	afterId Id,
	deleteUntilId *Id,
	insertId *Id,
	data T,
) (removed []Removed[Id, T], err error) {
	afterNode := r.byId[afterId]
	if afterNode == nil {
		var zero Id
		if afterId == zero {
			afterNode = &r.head
		} else {
			return nil, ErrBadAnchor
		}
	}

	doDelete := false
	var deleteUntil Id
	if deleteUntilId != nil {
		// Only perform deletion if deleteUntilId is different from afterId
		// This ensures that Splice(A, &A, nil, data) does not delete anything.
		if *deleteUntilId != afterId {
			doDelete = true
			deleteUntil = *deleteUntilId
		}
	}

	doInsert := insertId != nil
	var length int
	var iid Id

	if doInsert {
		if _, exists := r.byId[*insertId]; exists {
			return nil, ErrIdExists
		}
		iid = *insertId

		if s, ok := any(data).(Sizer); ok {
			length = s.Len()
		}
		// If not a Sizer, length stays 0.

		if length < 0 {
			return nil, ErrNegativeLength
		}
	}

	return r.splice(afterNode, doDelete, deleteUntil, doInsert, iid, length, data)
}




func (r *ropeImpl[Id, T]) splice(after *ropeNode[Id, T], doDelete bool, deleteUntil Id, doInsert bool, insertId Id, length int, data T) (removed []Removed[Id, T], err error) {
	type ropeSeek struct {
		node *ropeNode[Id, T]
		sub  int
	}
	var seekStack [maxHeight]ropeSeek
	seek := seekStack[:r.height]
	cseek := ropeSeek{node: after, sub: after.dl.Len}
	i := 0
	for {
		nl := len(cseek.node.levels)
		for i < nl {
			seek[i] = cseek
			i++
		}
		if cseek.node == &r.head || i == r.height {
			break
		}
		link := i - 1
		cseek.node = cseek.node.levels[link].prev
		cseek.sub += cseek.node.levels[link].subtreesize
	}
	if doDelete {
		for {
			e := after.levels[0].next
			if e == nil {
				r.lastId = after.id
				break
			}
			deletedId := e.id
			
			removed = append(removed, Removed[Id, T]{
				Id:   e.id,
				Len:  e.dl.Len,
				Data: e.dl.Data,
			})
			
			if e.iterRef != nil {
				e.iterRef.node = e.levels[0].prev
			}
			delete(r.byId, e.id)
			r.len -= e.dl.Len
			for j := 0; j < r.height; j++ {
				node := seek[j].node
				nl := &node.levels[j]
				if j >= len(e.levels) {
					nl.subtreesize -= e.dl.Len
					continue
				}
				el := e.levels[j]
				nl.subtreesize += el.subtreesize - e.dl.Len
				next := el.next
				if next != nil {
					next.levels[j].prev = node
				}
				nl.next = next
			}
			r.returnToPool(e)
			if deletedId == deleteUntil {
				break
			}
		}
		if r.byId[r.lastId] == nil {
			r.lastId = after.id
		}
	}
	if doInsert {
		var newNode *ropeNode[Id, T]
		var height int
		if len(r.nodePool) > 0 {
			idx := len(r.nodePool) - 1
			newNode = r.nodePool[idx]
			r.nodePool = r.nodePool[:idx]
			newNode.id = insertId
			newNode.dl = DataLen[T]{Data: data, Len: length}

			height = randomHeight()
			if cap(newNode.levels) < height {
				newNode.levels = make([]ropeLevel[Id, T], height)
			} else {
				newNode.levels = newNode.levels[:height]
			}
		} else {
			height = randomHeight()
			newNode = &ropeNode[Id, T]{
				id:     insertId,
				dl:     DataLen[T]{Data: data, Len: length},
				levels: make([]ropeLevel[Id, T], height),
			}
		}
		r.byId[insertId] = newNode
		for i = 0; i < height; i++ {
			if i < r.height {
				n := seek[i].node
				nl := &n.levels[i]
				next := nl.next
				if next != nil {
					next.levels[i].prev = newNode
				}
				st := seek[i].sub
				newNode.levels[i] = ropeLevel[Id, T]{
					next:        next,
					prev:        n,
					subtreesize: length + nl.subtreesize - st,
				}
				nl.next = newNode
				nl.subtreesize = st
			} else {
				link := len(cseek.node.levels) - 1
				for cseek.node != &r.head {
					cseek.node = cseek.node.levels[link].prev
					cseek.sub += cseek.node.levels[link].subtreesize
				}
				r.head.levels = append(r.head.levels, ropeLevel[Id, T]{
					next:        newNode,
					prev:        &r.head,
					subtreesize: cseek.sub,
				})
				r.height++
				newNode.levels[i] = ropeLevel[Id, T]{
					next:        nil,
					prev:        &r.head,
					subtreesize: r.len - cseek.sub + length,
				}
			}
		}
		for ; i < len(seek); i++ {
			seek[i].node.levels[i].subtreesize += length
		}
		r.len += length
		if after == &r.head {
			if r.len == length {
				r.lastId = insertId
			}
		} else if r.lastId == after.id {
			r.lastId = insertId
		}
	}
	return removed, nil
}

func (r *ropeImpl[Id, T]) DataPtr(id Id) *T {
	node := r.byId[id]
	if node == nil {
		return nil
	}
	return &node.dl.Data
}

func (r *ropeImpl[Id, T]) Less(a, b Id) bool {
	c, _ := r.Compare(a, b)
	return c < 0
}

func (r *ropeImpl[Id, T]) Between(afterA, afterB Id) (distance int, ok bool) {
	posA := r.Find(afterA)
	if posA < 0 {
		return
	}

	posB := r.Find(afterB)
	if posB < 0 {
		return
	}

	return posB - posA, true
}

func (r *ropeImpl[Id, T]) rseekNodes(curr *ropeNode[Id, T], target *[maxHeight]*ropeNode[Id, T]) {
	i := 0
	for {
		ll := len(curr.levels)
		for i < ll {
			target[i] = curr
			i++
			if i == r.height {
				return
			}
		}
		curr = curr.levels[ll-1].prev
	}
}

func (r *ropeImpl[Id, T]) Compare(a, b Id) (cmp int, ok bool) {
	if a == b {
		_, ok = r.byId[a]
		return
	}

	anode := r.byId[a]
	bnode := r.byId[b]

	if anode == nil || bnode == nil {
		return
	}

	// this is about 15% faster than the naïve version (rseekNodes for both)
	// swapping might be a touch faster, maybe negligible

	cmp = 1
	ok = true
	if len(anode.levels) < len(bnode.levels) {
		// swap more levels into anode; seek will be faster
		cmp = -1
		anode, bnode = bnode, anode
	}

	curr := bnode

	var anodes [maxHeight]*ropeNode[Id, T]
	r.rseekNodes(anode, &anodes)

	// walk up the tree
	i := 1
	for {
		ll := len(curr.levels)
		for i < ll {
			// stepped "right" into the previous node tree, so it must be after us
			if curr == anodes[i] {
				return
			}
			i++
		}

		ll--
		curr = curr.levels[ll].prev
		if curr == anodes[ll] {
			// stepped "up" into the previous node tree, so must be before us
			cmp = -cmp
			return
		} else if curr == &r.head {
			// stepped "up" to root, so must be after us (we never saw it in walk)
			return
		}
	}
}

func (r *ropeImpl[Id, T]) returnToPool(e *ropeNode[Id, T]) {
	if len(r.nodePool) == poolSize || e.iterRef != nil {
		return
	}

	var zero ropeLevel[Id, T]
	for i := range e.levels {
		e.levels[i] = zero
	}

	// this just clears stuff in case it's a ptr for GC
	var tmp Id
	e.dl = DataLen[T]{}
	e.id = tmp

	r.nodePool = append(r.nodePool, e)
}

func (r *ropeImpl[Id, T]) Iter(afterId Id) iter.Seq2[Id, DataLen[T]] {
	return func(yield func(Id, DataLen[T]) bool) {
		e := r.byId[afterId]
		if e == nil {
			return
		}

		for {
			next := e.levels[0].next
			if next == nil {
				return
			}

			e = next

			if e.iterRef == nil {
				e.iterRef = &iterRef[Id, T]{node: e, count: 1}
			} else {
				e.iterRef.count++
			}

			shouldContinue := yield(e.id, e.dl)

			// this will probably be ourselves unless we were deleted
			update := e.iterRef.node
			e.iterRef.count--
			if e.iterRef.count == 0 {
				e.iterRef = nil
			}
			e = update

			if !shouldContinue {
				return
			}
		}
	}
}

func (r *ropeImpl[Id, T]) LastId() Id {
	return r.lastId
}
