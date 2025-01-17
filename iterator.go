package iter

import (
	"unicode/utf8"
)

// Iterator[T] represents an iterator yielding elements of type T.
type Iterator[T any] interface {
	// Next yields a new value from the Iterator.
	Next() Option[T]
}

type stringIter struct {
	input string
}

// String returns an Iterator yielding runes from the supplied string.
func String(input string) Iterator[rune] {
	return &stringIter{
		input: input,
	}
}

func (it *stringIter) Next() Option[rune] {
	if len(it.input) == 0 {
		return None[rune]()
	}
	value, width := utf8.DecodeRuneInString(it.input)
	it.input = it.input[width:]
	return Some(value)
}

type rangeIter struct {
	start, stop, step, i int
}

// Range returns an Iterator over a range of integers.
func Range(start, stop, step int) Iterator[int] {
	return &rangeIter{
		start: start,
		stop:  stop,
		step:  step,
		i:     0,
	}
}

func (it *rangeIter) Next() Option[int] {
	v := it.start + it.step*it.i
	if it.step > 0 {
		if v >= it.stop {
			return None[int]()
		}
	} else {
		if v <= it.stop {
			return None[int]()
		}
	}
	it.i++
	return Some(v)
}

type sliceIter[T any] struct {
	slice []T
}

// Slice returns an Iterator that yields elements from a slice.
func Slice[T any](slice []T) Iterator[T] {
	return &sliceIter[T]{
		slice: slice,
	}
}

func (it *sliceIter[T]) Next() Option[T] {
	if len(it.slice) == 0 {
		return None[T]()
	}
	first := it.slice[0]
	it.slice = it.slice[1:]
	return Some[T](first)
}

type chanIter[T any] struct {
	ch <-chan T
}

// Chan returns an Iterator that yields elements written to a channel.
func Chan[T any](ch <-chan T) Iterator[T] {
	return &chanIter[T]{
		ch: ch,
	}
}

func (c *chanIter[T]) Next() Option[T] {
	v, ok := <-c.ch
	if !ok {
		return None[T]()
	}
	return Some(v)
}

// ToSlice consumes an Iterator creating a slice from the yielded values.
func ToSlice[T any](it Iterator[T]) []T {
	result := []T{}
	ForEach(it, func(v T) {
		result = append(result, v)
	})
	return result
}

// ToString consumes a rune Iterator creating a string.
func ToString(it Iterator[rune]) string {
	return string(ToSlice(it))
}

// ToChan consumes an Iterator writing yielded values to the returned channel.
func ToChan[T any](it Iterator[T]) <-chan T {
	ch := make(chan T)
	go func() {
		defer func() {
			if r := recover(); r != nil {
			}
		}()
		ForEach(it, func(v T) {
			ch <- v
		})
		close(ch)
	}()
	return ch
}

type mapIter[T, R any] struct {
	inner Iterator[T]
	fn    func(T) R
}

// Map is an Iterator adapter that transforms each value yielded by the
// underlying iterator using fn.
func Map[T, R any](it Iterator[T], fn func(T) R) Iterator[R] {
	return &mapIter[T, R]{
		inner: it,
		fn:    fn,
	}
}

func (it *mapIter[T, R]) Next() Option[R] {
	return MapOption(it.inner.Next(), it.fn)
}

type filterIter[T any] struct {
	inner Iterator[T]
	pred  func(T) bool
}

// Filter returns an Iterator adapter that yields elements from the underlying
// Iterator for which pred returns true.
func Filter[T any](it Iterator[T], pred func(T) bool) Iterator[T] {
	return &filterIter[T]{
		inner: it,
		pred:  pred,
	}
}

func (it *filterIter[T]) Next() Option[T] {
	v := it.inner.Next()
	for v.IsSome() {
		if it.pred(v.Unwrap()) {
			break
		}
		v = it.inner.Next()
	}
	return v
}

type takeIter[T any] struct {
	inner Iterator[T]
	take  uint
}

// Take returns an Iterator adapter that yields the n first elements from the
// underlying Iterator.
func Take[T any](it Iterator[T], n uint) Iterator[T] {
	return &takeIter[T]{
		inner: it,
		take:  n,
	}
}

func (it *takeIter[T]) Next() Option[T] {
	if it.take == 0 {
		return None[T]()
	}
	v := it.inner.Next()
	if v.IsSome() {
		it.take--
	}
	return v
}

type takeWhileIter[T any] struct {
	inner Iterator[T]
	pred  func(T) bool
	done  bool
}

// TakeWhile returns an Iterator adapter that yields values from the underlying
// Iterator as long as pred predicate function returns true.
func TakeWhile[T any](it Iterator[T], pred func(T) bool) Iterator[T] {
	return &takeWhileIter[T]{
		inner: it,
		pred:  pred,
		done:  false,
	}
}

func (it *takeWhileIter[T]) Next() Option[T] {
	if it.done {
		return None[T]()
	}
	v := it.inner.Next()
	if v.IsNone() {
		it.done = true
		return v
	}
	if !it.pred(v.Unwrap()) {
		it.done = true
		return None[T]()
	}
	return v
}

type dropIter[T any] struct {
	inner Iterator[T]
	drop  uint
}

// Drop returns an Iterator adapter that drops the first n elements from the
// underlying Iterator before yielding any values.
func Drop[T any](it Iterator[T], n uint) Iterator[T] {
	return &dropIter[T]{
		inner: it,
		drop:  n,
	}
}

func (it *dropIter[T]) Next() Option[T] {
	v := None[T]()
	for it.drop > 0 {
		v = it.inner.Next()
		if v.IsNone() {
			it.drop = 0
			return v
		}
		it.drop--
	}
	return it.inner.Next()
}

type dropWhileIter[T any] struct {
	inner Iterator[T]
	pred  func(T) bool
	done  bool
}

// DropWhile returns an Iterator adapter that drops elements from the underlying
// Iterator until pred predicate function returns true.
func DropWhile[T any](it Iterator[T], pred func(T) bool) Iterator[T] {
	return &dropWhileIter[T]{
		inner: it,
		pred:  pred,
		done:  false,
	}
}

func (it *dropWhileIter[T]) Next() Option[T] {
	if !it.done {
		for {
			v := it.inner.Next()
			if v.IsNone() {
				it.done = true
				return v
			}
			if !it.pred(v.Unwrap()) {
				it.done = true
				return v
			}
		}
	}
	return it.inner.Next()
}

type repeatIter[T any] struct {
	value T
}

// Repeat returns an Iterator that repeatedly returns the same value.
func Repeat[T any](value T) Iterator[T] {
	return &repeatIter[T]{
		value: value,
	}
}

func (it *repeatIter[T]) Next() Option[T] {
	return Some(it.value)
}

// Count consumes an Iterator and returns the number of elements it yielded.
func Count[T any](it Iterator[T]) uint {
	var length uint
	v := it.Next()
	for v.IsSome() {
		length++
		v = it.Next()
	}
	return length
}

type funcIter[T any] struct {
	fn func() Option[T]
}

// Func returns an Iterator from a function.
func Func[T any](fn func() Option[T]) Iterator[T] {
	return &funcIter[T]{
		fn: fn,
	}
}

func (it *funcIter[T]) Next() Option[T] {
	return it.fn()
}

type emptyIter[T any] struct{}

// Empty returns an empty Iterator.
func Empty[T any]() Iterator[T] {
	return &emptyIter[T]{}
}

func (it *emptyIter[T]) Next() Option[T] {
	return None[T]()
}

type onceIter[T any] struct {
	value Option[T]
}

// Once returns an Iterator that returns a value exactly once.
func Once[T any](value T) Iterator[T] {
	return &onceIter[T]{
		value: Some(value),
	}
}

func (it *onceIter[T]) Next() Option[T] {
	v := it.value
	it.value = None[T]()
	return v
}

// ForEach consumes the Iterator applying fn to each yielded value.
func ForEach[T any](it Iterator[T], fn func(T)) {
	v := it.Next()
	for v.IsSome() {
		fn(v.Unwrap())
		v = it.Next()
	}
}

// Fold reduces Iterator using function fn.
func Fold[T any, B any](it Iterator[T], init B, fn func(B, T) B) B {
	ret := init
	ForEach(it, func(v T) {
		ret = fn(ret, v)
	})
	return ret
}

type fuseIter[T any] struct {
	inner Iterator[T]
	done  bool
}

// Fuse returns an Iterator adapter that will keep yielding None after the
// underlying Iterator has yielded None once.
func Fuse[T any](it Iterator[T]) Iterator[T] {
	return &fuseIter[T]{
		inner: it,
		done:  false,
	}
}

func (it *fuseIter[T]) Next() Option[T] {
	if it.done {
		return None[T]()
	}
	v := it.inner.Next()
	if v.IsNone() {
		it.done = true
	}
	return v
}

type chainIter[T any] struct {
	first  Iterator[T]
	second Iterator[T]
}

// Chain returns an Iterator that concatenates two iterators.
func Chain[T any](first Iterator[T], second Iterator[T]) Iterator[T] {
	return &chainIter[T]{
		first:  Fuse(first),
		second: second,
	}
}

func (it *chainIter[T]) Next() Option[T] {
	v := it.first.Next()
	if v.IsSome() {
		return v
	}
	return it.second.Next()
}

// Find the first element from Iterator that satisfies pred predicate function.
func Find[T any](it Iterator[T], pred func(T) bool) Option[T] {
	return Filter(it, pred).Next()
}

type flattenIter[T any] struct {
	inner   Iterator[Iterator[T]]
	current Iterator[T]
	done    bool
}

// Flatten returns an Iterator adapter that flattens nested iterators.
func Flatten[T any](it Iterator[Iterator[T]]) Iterator[T] {
	return &flattenIter[T]{
		inner:   it,
		current: Empty[T](),
		done:    false,
	}
}

func (it *flattenIter[T]) Next() Option[T] {
	for {
		if it.done {
			return None[T]()
		}
		v := it.current.Next()
		if v.IsSome() {
			return v
		}
		next := it.inner.Next()
		if next.IsNone() {
			it.done = true
			return None[T]()
		}
		it.current = next.Unwrap()
	}
}

// All tests if every element of the Iterator matches a predicate. An empty
// Iterator returns true.
func All[T any](it Iterator[T], pred func(T) bool) bool {
	v := it.Next()
	for v.IsSome() {
		if !pred(v.Unwrap()) {
			return false
		}
		v = it.Next()
	}
	return true
}

// Any tests if any element of the Iterator matches a predicate. An empty
// Iterator returns false.
func Any[T any](it Iterator[T], pred func(T) bool) bool {
	v := it.Next()
	for v.IsSome() {
		if pred(v.Unwrap()) {
			return true
		}
		v = it.Next()
	}
	return false
}

// Nth returns nth element of the Iterator.
func Nth[T any](it Iterator[T], n uint) Option[T] {
	v := it.Next()
	for n > 0 {
		if v.IsNone() {
			break
		}
		v = it.Next()
		n--
	}
	return v
}

// Determines if the elements of two Iterators are equal.
func Equal[T comparable](first Iterator[T], second Iterator[T]) bool {
	return EqualBy(
		first,
		second,
		func(a T, b T) bool {
			return a == b
		},
	)
}

// Determines if the elements of two Iterators are equal using function cmp to
// compare elements.
func EqualBy[T any](first Iterator[T], second Iterator[T], cmp func(T, T) bool) bool {
	for {
		v := first.Next()
		if v.IsNone() {
			return second.Next().IsNone()
		}
		u := second.Next()
		if u.IsNone() {
			return false
		}
		if !cmp(v.Unwrap(), u.Unwrap()) {
			return false
		}
	}
}
