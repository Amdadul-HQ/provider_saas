package gql

import (
	"context"

	"github.com/vikstrous/dataloadgen"
)

// Loader is a batching, per-request caching loader.
type Loader[K comparable, V any] = dataloadgen.Loader[K, V]

// NewLoader builds a loader from a map-returning batch function. Keys missing from the
// map resolve to the zero value of V without error (nil for pointers, 0 for counts).
func NewLoader[K comparable, V any](fetch func(ctx context.Context, keys []K) (map[K]V, error)) *Loader[K, V] {
	return dataloadgen.NewLoader(func(ctx context.Context, keys []K) ([]V, []error) {
		m, err := fetch(ctx, keys)
		vals := make([]V, len(keys))
		var errs []error
		if err != nil {
			errs = make([]error, len(keys))
			for i := range errs {
				errs[i] = err
			}
			return vals, errs
		}
		for i, k := range keys {
			vals[i] = m[k]
		}
		return vals, nil
	}, dataloadgen.WithWait(LoaderWait), dataloadgen.WithBatchCapacity(500))
}
