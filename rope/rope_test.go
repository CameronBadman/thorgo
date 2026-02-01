package rope

import (
	"iter"
	"math/rand/v2"
	"reflect"
	"testing"
)

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
		r := New[int, struct{}]()

		for j := 0; j < benchOps; j++ {
			if len(ids) <= 2 || rand.IntN(deleteOddsOf) != 0 {
				choice := rand.IntN(len(ids))
				afterId := ids[choice]
				newId := nextId()

				r.Insert(afterId, newId, rand.IntN(16), struct{}{})
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
	r := New[int, struct{}]()
	ids := []int{0}

	for range 100_000 {
		choice := rand.IntN(len(ids))
		afterId := ids[choice]
		newId := nextId()

		_, err := r.Splice(&afterId, nil, &newId, rand.IntN(16), struct{}{})
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

		r := New[int, string]()

		helloId := nextId()
		zero := 0
		_, err := r.Splice(&zero, nil, &helloId, 5, "hello")
		if err != nil {
			t.Errorf("couldn't insert hello: %v", err)
		}

		if r.Count() != 1 {
			t.Errorf("expected count=1")
		}
		if r.Len() != 5 {
			t.Errorf("expected len=5, was=%v", r.Len())
		}
		if helloId == 0 {
			t.Errorf("expected +ve Id, was=%v", helloId)
		}
		helloAt := r.Find(helloId)
		if helloAt != 5 {
			t.Errorf("expected helloAt=5, was=%v", helloAt)
		}

		thereId := nextId()
		_, err = r.Splice(&helloId, nil, &thereId, 6, " there")
		if err != nil {
			t.Errorf("couldn't insert there: %v", err)
		}
		if r.Len() != 11 {
			t.Errorf("expected len=11, was=%v", r.Len())
		}
		if r.Count() != 2 {
			t.Errorf("expected count=2")
		}

		thereLookup := r.Info(thereId)
		thereAt := r.Find(thereId)

		if thereAt != 11 {
			t.Errorf("expected thereAt=11, was=%v", thereAt)
		}
		if !reflect.DeepEqual(thereLookup, Info[int, string]{
			Id:      thereId,
			Next:    0,
			Prev:    helloId,
			DataLen: DataLen[string]{Data: " there", Len: 6},
		}) {
			t.Errorf("bad lookup=%+v", thereLookup)
		}

		if id, offset := r.ByPosition(5, false); id != helloId || offset != 0 {
			t.Errorf("bad byPosition: id=%d (wanted=%d), offset=%d", id, helloId, offset)
		}
		if id, offset := r.ByPosition(5, true); id != thereId || offset != 6 {
			t.Errorf("bad byPosition: id=%d (wanted=%d), offset=%d", id, thereId, offset)
		}
		if id, offset := r.ByPosition(0, false); id != 0 || offset != 0 {
			t.Errorf("bad byPosition: id=%d (wanted=%d), offset=%d", id, 0, offset)
		}
		if id, offset := r.ByPosition(0, true); id != helloId || offset != 5 {
			t.Errorf("bad byPosition: id=%d (wanted=%d), offset=%d", id, helloId, offset)
		}

		var cmp int
		var ok bool
		cmp, ok = r.Compare(helloId, thereId)
		if !ok || cmp >= 0 {
			t.Errorf("bad cmp for ids (should be -1, hello before there): %v", cmp)
		}
		cmp, ok = r.Compare(thereId, helloId)
		if !ok || cmp <= 0 {
			t.Errorf("bad cmp for ids (should be +1, there not before hello): %v", cmp)
		}
		cmp, ok = r.Compare(thereId, thereId)
		if !ok || cmp != 0 {
			t.Errorf("bad cmp for ids: %v", cmp)
		}
		cmp, ok = r.Compare(thereId, -1)
		if ok || cmp != 0 {
			t.Errorf("bad cmp for ids: %v", cmp)
		}

		var out []int
		for id := range r.Iter(0) {
			out = append(out, id)
		}
		if !reflect.DeepEqual(out, []int{helloId, thereId}) {
			t.Errorf("bad read")
		}

		deleted, err := r.Splice(&zero, &helloId, nil, 0, "")
		if err != nil {
			t.Errorf("delete failed: %v", err)
		}
		if deleted != 1 {
			t.Errorf("expected deleted one, was: %v", deleted)
		}
		if r.Len() != 6 {
			t.Errorf("didn't reduce by hello size: wanted=%d, got=%d", 6, r.Len())
		}
		if thereAt = r.Find(thereId); thereAt != 6 {
			t.Errorf("wrong there: %d", thereAt)
		}
		if r.Count() != 1 {
			t.Errorf("expected count=1")
		}
	}
}

func TestRandomRope(t *testing.T) {
	ids := make([]int, 0, 51)

	for i := 0; i < 100; i++ {
		r := New[int, string]()
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
			_, err := r.Splice(&parent, nil, &newId, length, s)
			if err != nil {
				t.Errorf("couldn't insert: %v", err)
			}
			ids = append(ids, newId)
		}
	}
}

func TestIter(t *testing.T) {
	r := New[int, string]()

	zero := 0
	one := nextId()
	two := nextId()
	three := nextId()

	_, err := r.Splice(&zero, nil, &one, 5, "hello")
	if err != nil {
		t.Errorf("couldn't insert: %v", err)
	}
	if r.LastId() != one {
		t.Errorf("should have first lastId")
	}

	_, err = r.Splice(&one, nil, &two, 6, " there")
	if err != nil {
		t.Errorf("couldn't insert: %v", err)
	}
	if r.LastId() != two {
		t.Errorf("should have second lastId")
	}

	_, err = r.Splice(&two, nil, &three, 4, " bob")
	if err != nil {
		t.Errorf("couldn't insert: %v", err)
	}
	if r.LastId() != three {
		t.Errorf("should have third lastId")
	}

	i := r.Iter(0)
	next, stop := iter.Pull2(i)
	defer stop()

	id, value, _ := next()
	if id != one || value.Data != "hello" {
		t.Errorf("bad first next")
	}

	// same afterId and deleteUntilId should delete nothing
	deleted, _ := r.Splice(&one, &one, nil, 0, "")
	if deleted != 0 {
		t.Errorf("should not delete any with same values, got: %d", deleted)
	}

	// delete one (after zero, until one)
	deleted, _ = r.Splice(&zero, &one, nil, 0, "")
	if deleted != 1 {
		t.Errorf("should delete one, got: %d", deleted)
	}

	id, value, _ = next()
	if id != two || value.Data != " there" {
		t.Errorf("bad additional next, got: id=%d value=%v", id, value)
	}

	// delete from two until three
	r.Splice(&two, &three, nil, 0, "")
	_, _, ok := next()
	if ok {
		t.Errorf("should not get more values: last deleted")
	}

	i = r.Iter(0)
	next, stop = iter.Pull2(i)
	defer stop()

	if r.Count() != 1 {
		t.Errorf("should have single entry, got: %d", r.Count())
	}

	if r.LastId() != two {
		t.Errorf("should have second lastId, was=%v", r.LastId())
	}

	// delete remaining
	r.Splice(&zero, &two, nil, 0, "")

	if r.Count() != 0 {
		t.Errorf("should have no entries")
	}

	id, value, ok = next()
	if ok {
		t.Errorf("should not get more values: last deleted: was=%v %v", id, value)
	}

	if r.LastId() != 0 {
		t.Errorf("should have zero lastId, was=%v", r.LastId())
	}
}
