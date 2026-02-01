package rope

import (
	"iter"
	"math/rand/v2"
	"reflect"
	"testing"
)

// --- Helper types for Sizer interface ---

type SizedString string

func (s SizedString) Len() int { return len(s) }

type SizedEmpty int // We'll use the int to store the intended length

func (s SizedEmpty) Len() int { return int(s) }

// ---------------------------------------

const (
	benchOps     = 100_000
	deleteOddsOf = 20
)

var (
	internalNextId = 0
)

func nextId() int {
	internalNextId++
	return internalNextId
}

func BenchmarkRope(b *testing.B) {
	ops := benchOps * (deleteOddsOf - 1) / deleteOddsOf
	ids := make([]int, 0, ops)

	for b.Loop() {
		ids = ids[:0]
		ids = append(ids, 0)
		r := New[int, SizedEmpty]() // Using SizedEmpty instead of struct{}

		for j := 0; j < benchOps; j++ {
			if len(ids) <= 2 || rand.IntN(deleteOddsOf) != 0 {
				choice := rand.IntN(len(ids))
				afterId := ids[choice]
				newId := nextId()

				// Length is now inferred from the SizedEmpty value
				r.Insert(afterId, newId, SizedEmpty(rand.IntN(16)))
				ids = append(ids, newId)
			} else {
				choice := 1 + rand.IntN(len(ids)-2)
				deleteId := ids[choice]
				last := ids[len(ids)-1]
				ids = ids[:len(ids)-1]
				ids[choice] = last

				info := r.Info(deleteId)
				r.Delete(info.Prev, deleteId)
			}
		}
	}
}

func BenchmarkCompare(b *testing.B) {
	r := New[int, SizedEmpty]()
	ids := []int{0}

	for range 100_000 {
		choice := rand.IntN(len(ids))
		afterId := ids[choice]
		newId := nextId()

		// Splice now takes afterId by value, and length is gone
		_, err := r.Splice(afterId, nil, &newId, SizedEmpty(rand.IntN(16)))
		if err != nil {
			b.Errorf("couldn't insert: %v", err)
		}
		ids = append(ids, newId)
	}

	for b.Loop() {
		a := ids[rand.IntN(len(ids))]
		c := ids[rand.IntN(len(ids))]
		r.Less(a, c)
	}
}

func TestRope(t *testing.T) {
	for i := 0; i < 50; i++ {
		if t.Failed() {
			return
		}

		r := New[int, SizedString]()

		helloId := nextId()
		zero := 0
		_, err := r.Splice(zero, nil, &helloId, SizedString("hello"))
		if err != nil {
			t.Errorf("couldn't insert hello: %v", err)
		}

		if r.Count() != 1 {
			t.Errorf("expected count=1")
		}
		if r.Len() != 5 {
			t.Errorf("expected len=5, was=%v", r.Len())
		}
		helloAt := r.Find(helloId)
		if helloAt != 5 {
			t.Errorf("expected helloAt=5, was=%v", helloAt)
		}

		thereId := nextId()
		_, err = r.Splice(helloId, nil, &thereId, SizedString(" there"))
		if err != nil {
			t.Errorf("couldn't insert there: %v", err)
		}
		if r.Len() != 11 {
			t.Errorf("expected len=11, was=%v", r.Len())
		}

		thereLookup := r.Info(thereId)
		if !reflect.DeepEqual(thereLookup, Info[int, SizedString]{
			Id:   thereId,
			Next: 0,
			Prev: helloId,
			DataLen: DataLen[SizedString]{Data: " there", Len: 6},
		}) {
			t.Errorf("bad lookup=%+v", thereLookup)
		}

		// Positional lookups
		if id, offset := r.ByPosition(5, false); id != helloId || offset != 0 {
			t.Errorf("bad byPosition: id=%d (wanted=%d), offset=%d", id, helloId, offset)
		}
		if id, offset := r.ByPosition(5, true); id != thereId || offset != 6 {
			t.Errorf("bad byPosition: id=%d (wanted=%d), offset=%d", id, thereId, offset)
		}

		// Deletion
		removed, err := r.Splice(zero, &helloId, nil, "")
		if err != nil {
			t.Errorf("delete failed: %v", err)
		}
		if len(removed) != 1 {
			t.Errorf("expected deleted one, was: %v", len(removed))
		}
		if r.Len() != 6 {
			t.Errorf("didn't reduce by hello size: wanted=%d, got=%d", 6, r.Len())
		}
	}
}

func TestRandomRope(t *testing.T) {
	ids := make([]int, 0, 51)

	for i := 0; i < 100; i++ {
		r := New[int, SizedString]()
		ids = ids[:0]
		ids = append(ids, 0)

		for j := 0; j < 50; j++ {
			choice := rand.IntN(len(ids))
			parent := ids[choice]

			length := rand.IntN(4) + 1
			var s string
			for range length {
				s += string(rune('a' + rand.IntN(26)))
			}

			newId := nextId()
			_, err := r.Splice(parent, nil, &newId, SizedString(s))
			if err != nil {
				t.Errorf("couldn't insert: %v", err)
			}
			ids = append(ids, newId)
		}
	}
}

func TestIter(t *testing.T) {
	r := New[int, SizedString]()

	zero := 0
	one := nextId()
	two := nextId()
	three := nextId()

	r.Splice(zero, nil, &one, SizedString("hello"))
	r.Splice(one, nil, &two, SizedString(" there"))
	r.Splice(two, nil, &three, SizedString(" bob"))

	i := r.Iter(0)
	next, stop := iter.Pull2(i)
	defer stop()

	id, value, _ := next()
	if id != one || value.Data != "hello" {
		t.Errorf("bad first next")
	}

	// same afterId and deleteUntilId should delete nothing
	removed, _ := r.Splice(one, &one, nil, "")
	if len(removed) != 0 {
		t.Errorf("should not delete any with same values, got: %d", len(removed))
	}

	// delete one (after zero, until one)
	r.Splice(zero, &one, nil, "")

	id, value, _ = next()
	if id != two || value.Data != " there" {
		t.Errorf("bad additional next, got: id=%d value=%v", id, value)
	}

	// delete from two until three
	r.Splice(two, &three, nil, "")
	_, _, ok := next()
	if ok {
		t.Errorf("should not get more values: last deleted")
	}
}
