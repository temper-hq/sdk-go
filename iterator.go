package temper

import "context"

// Iterator walks the results of a paginated endpoint one at a time, fetching pages automatically:
//
//	it := client.Environments.Iter(temper.ListEnvironmentsParams{})
//	for it.Next(ctx) {
//		env := it.Value()
//	}
//	if err := it.Err(); err != nil { ... }
type Iterator[T any] struct {
	fetch  func(ctx context.Context, after string) ([]T, string, error)
	after  string
	buf    []T
	cur    T
	err    error
	done   bool
	primed bool
}

func newIterator[T any](after string, fetch func(ctx context.Context, after string) ([]T, string, error)) *Iterator[T] {
	return &Iterator[T]{fetch: fetch, after: after}
}

// Next advances to the next item. It returns false when there are no more items or an error occurred (check Err).
func (it *Iterator[T]) Next(ctx context.Context) bool {
	for len(it.buf) == 0 {
		if it.err != nil || (it.done && it.primed) {
			return false
		}
		items, next, err := it.fetch(ctx, it.after)
		it.primed = true
		if err != nil {
			it.err = err
			return false
		}
		it.buf = items
		it.after = next
		it.done = next == ""
		if len(items) == 0 && it.done {
			return false
		}
	}
	it.cur, it.buf = it.buf[0], it.buf[1:]
	return true
}

// Value returns the current item.
func (it *Iterator[T]) Value() T { return it.cur }

// Err returns the error encountered while fetching pages, if any.
func (it *Iterator[T]) Err() error { return it.err }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
