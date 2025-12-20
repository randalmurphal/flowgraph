package registry

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	r := New[string, int]()
	assert.NotNil(t, r)
	assert.Equal(t, 0, r.Len())
}

func TestRegisterAndGet(t *testing.T) {
	r := New[string, int]()

	r.Register("one", 1)
	r.Register("two", 2)

	v, ok := r.Get("one")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	v, ok = r.Get("two")
	assert.True(t, ok)
	assert.Equal(t, 2, v)

	// Non-existent key
	v, ok = r.Get("three")
	assert.False(t, ok)
	assert.Equal(t, 0, v) // zero value
}

func TestRegisterOverwrite(t *testing.T) {
	r := New[string, string]()

	r.Register("key", "old")
	r.Register("key", "new")

	v, ok := r.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "new", v)
}

func TestRegisterMany(t *testing.T) {
	r := New[string, int]()

	entries := map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
	}
	r.RegisterMany(entries)

	assert.Equal(t, 3, r.Len())

	for k, v := range entries {
		got, ok := r.Get(k)
		assert.True(t, ok)
		assert.Equal(t, v, got)
	}
}

func TestRegisterManyEmpty(t *testing.T) {
	r := New[string, int]()
	r.Register("existing", 42)

	r.RegisterMany(map[string]int{})

	assert.Equal(t, 1, r.Len())
}

func TestMustGet(t *testing.T) {
	r := New[string, int]()
	r.Register("key", 42)

	v := r.MustGet("key")
	assert.Equal(t, 42, v)
}

func TestMustGetPanic(t *testing.T) {
	r := New[string, int]()

	assert.PanicsWithValue(t, "registry: key not found", func() {
		r.MustGet("nonexistent")
	})
}

func TestHas(t *testing.T) {
	r := New[string, int]()
	r.Register("key", 42)

	assert.True(t, r.Has("key"))
	assert.False(t, r.Has("nonexistent"))
}

func TestDelete(t *testing.T) {
	r := New[string, int]()
	r.Register("key", 42)

	assert.True(t, r.Has("key"))

	r.Delete("key")

	assert.False(t, r.Has("key"))
	_, ok := r.Get("key")
	assert.False(t, ok)
}

func TestDeleteNonexistent(t *testing.T) {
	r := New[string, int]()
	r.Register("key", 42)

	// Should not panic
	r.Delete("nonexistent")

	assert.Equal(t, 1, r.Len())
}

func TestKeys(t *testing.T) {
	r := New[string, int]()
	r.Register("one", 1)
	r.Register("two", 2)
	r.Register("three", 3)

	keys := r.Keys()

	assert.Len(t, keys, 3)
	assert.ElementsMatch(t, []string{"one", "two", "three"}, keys)
}

func TestKeysEmpty(t *testing.T) {
	r := New[string, int]()
	keys := r.Keys()
	assert.Empty(t, keys)
}

func TestLen(t *testing.T) {
	r := New[string, int]()
	assert.Equal(t, 0, r.Len())

	r.Register("one", 1)
	assert.Equal(t, 1, r.Len())

	r.Register("two", 2)
	assert.Equal(t, 2, r.Len())

	r.Delete("one")
	assert.Equal(t, 1, r.Len())
}

func TestRange(t *testing.T) {
	r := New[string, int]()
	r.Register("one", 1)
	r.Register("two", 2)
	r.Register("three", 3)

	visited := make(map[string]int)
	r.Range(func(k string, v int) bool {
		visited[k] = v
		return true
	})

	assert.Equal(t, map[string]int{"one": 1, "two": 2, "three": 3}, visited)
}

func TestRangeEarlyStop(t *testing.T) {
	r := New[string, int]()
	r.Register("one", 1)
	r.Register("two", 2)
	r.Register("three", 3)

	count := 0
	r.Range(func(k string, v int) bool {
		count++
		return false // stop after first
	})

	assert.Equal(t, 1, count)
}

func TestRangeEmpty(t *testing.T) {
	r := New[string, int]()

	called := false
	r.Range(func(k string, v int) bool {
		called = true
		return true
	})

	assert.False(t, called)
}

func TestRangeAllowsMutation(t *testing.T) {
	r := New[string, int]()
	r.Register("one", 1)
	r.Register("two", 2)

	// Range should work over a snapshot, allowing mutations
	r.Range(func(k string, v int) bool {
		r.Register("new-"+k, v*10)
		return true
	})

	// Original keys still exist, new keys added
	assert.True(t, r.Has("one"))
	assert.True(t, r.Has("two"))
	assert.True(t, r.Has("new-one"))
	assert.True(t, r.Has("new-two"))
	assert.Equal(t, 4, r.Len())
}

func TestGetOrCreate(t *testing.T) {
	r := New[string, int]()

	callCount := 0
	factory := func() int {
		callCount++
		return 42
	}

	// First call creates
	v := r.GetOrCreate("key", factory)
	assert.Equal(t, 42, v)
	assert.Equal(t, 1, callCount)

	// Second call returns existing
	v = r.GetOrCreate("key", factory)
	assert.Equal(t, 42, v)
	assert.Equal(t, 1, callCount) // factory not called again
}

func TestGetOrCreateMultipleKeys(t *testing.T) {
	r := New[string, string]()

	v1 := r.GetOrCreate("one", func() string { return "first" })
	v2 := r.GetOrCreate("two", func() string { return "second" })

	assert.Equal(t, "first", v1)
	assert.Equal(t, "second", v2)
	assert.Equal(t, 2, r.Len())
}

// Test with different key types
func TestIntegerKeys(t *testing.T) {
	r := New[int, string]()
	r.Register(1, "one")
	r.Register(2, "two")

	v, ok := r.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "one", v)
}

func TestStructKeys(t *testing.T) {
	type Key struct {
		Namespace string
		Name      string
	}

	r := New[Key, int]()
	k1 := Key{Namespace: "ns1", Name: "name1"}
	k2 := Key{Namespace: "ns2", Name: "name2"}

	r.Register(k1, 1)
	r.Register(k2, 2)

	v, ok := r.Get(k1)
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	v, ok = r.Get(k2)
	assert.True(t, ok)
	assert.Equal(t, 2, v)
}

// Thread-safety tests

func TestConcurrentRegister(t *testing.T) {
	r := New[int, int]()
	var wg sync.WaitGroup
	n := 1000

	for i := range n {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			r.Register(val, val*2)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, n, r.Len())
	for i := range n {
		v, ok := r.Get(i)
		assert.True(t, ok)
		assert.Equal(t, i*2, v)
	}
}

func TestConcurrentGet(t *testing.T) {
	r := New[int, int]()
	for i := range 100 {
		r.Register(i, i*2)
	}

	var wg sync.WaitGroup
	n := 1000

	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 100 {
				v, ok := r.Get(i)
				assert.True(t, ok)
				assert.Equal(t, i*2, v)
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentReadWrite(t *testing.T) {
	r := New[int, int]()
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writers
	for i := range 10 {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; ; j++ {
				select {
				case <-stop:
					return
				default:
					r.Register(writerID*1000+j, j)
				}
			}
		}(i)
	}

	// Readers
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					r.Keys()
					r.Len()
				}
			}
		}()
	}

	// Let it run briefly
	close(stop)
	wg.Wait()
}

func TestConcurrentGetOrCreate(t *testing.T) {
	r := New[string, int]()
	var wg sync.WaitGroup
	n := 100
	var callCount atomic.Int32

	factory := func() int {
		callCount.Add(1)
		return 42
	}

	// Many goroutines trying to create the same key
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := r.GetOrCreate("key", factory)
			assert.Equal(t, 42, v)
		}()
	}

	wg.Wait()

	// Factory should only be called once
	assert.Equal(t, int32(1), callCount.Load())
	assert.Equal(t, 1, r.Len())
}

func TestConcurrentGetOrCreateDifferentKeys(t *testing.T) {
	r := New[int, int]()
	var wg sync.WaitGroup
	n := 100

	for i := range n {
		wg.Add(1)
		go func(key int) {
			defer wg.Done()
			v := r.GetOrCreate(key, func() int { return key * 2 })
			assert.Equal(t, key*2, v)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, n, r.Len())
}

func TestConcurrentDelete(t *testing.T) {
	r := New[int, int]()
	for i := range 100 {
		r.Register(i, i)
	}

	var wg sync.WaitGroup

	// Concurrent deletes
	for i := range 100 {
		wg.Add(1)
		go func(key int) {
			defer wg.Done()
			r.Delete(key)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, 0, r.Len())
}

func TestConcurrentRangeWithMutations(t *testing.T) {
	r := New[int, int]()
	for i := range 100 {
		r.Register(i, i)
	}

	var wg sync.WaitGroup
	rangeStarted := make(chan struct{})

	// Range while others mutate
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		close(rangeStarted) // Signal that we're about to iterate
		r.Range(func(k, v int) bool {
			count++
			return true
		})
		// Range takes a snapshot, so count should be consistent
		// (all keys that existed when snapshot was taken)
		assert.GreaterOrEqual(t, count, 50) // At least remaining after deletes
		assert.LessOrEqual(t, count, 100)   // At most original count
	}()

	// Wait for range to start, then mutate
	<-rangeStarted

	// Concurrent mutations
	for i := range 50 {
		wg.Add(1)
		go func(key int) {
			defer wg.Done()
			r.Delete(key)
		}(i)
	}

	wg.Wait()
}

// Edge cases

func TestZeroValueKey(t *testing.T) {
	r := New[int, string]()
	r.Register(0, "zero")

	v, ok := r.Get(0)
	assert.True(t, ok)
	assert.Equal(t, "zero", v)
}

func TestEmptyStringKey(t *testing.T) {
	r := New[string, int]()
	r.Register("", 42)

	v, ok := r.Get("")
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestNilValue(t *testing.T) {
	r := New[string, *int]()
	r.Register("nil", nil)

	v, ok := r.Get("nil")
	assert.True(t, ok)
	assert.Nil(t, v)

	// Distinguish nil value from missing key
	_, ok = r.Get("missing")
	assert.False(t, ok)
}

func TestGetOrCreateWithNilValue(t *testing.T) {
	r := New[string, *int]()

	v := r.GetOrCreate("nil", func() *int { return nil })
	assert.Nil(t, v)

	// Verify it was stored
	assert.True(t, r.Has("nil"))
}

// Factory pattern example test
func TestFactoryPattern(t *testing.T) {
	type NodeFactory func(name string) string

	factories := New[string, NodeFactory]()

	factories.Register("start", func(name string) string {
		return "start:" + name
	})
	factories.Register("end", func(name string) string {
		return "end:" + name
	})

	factory, ok := factories.Get("start")
	require.True(t, ok)

	result := factory("node1")
	assert.Equal(t, "start:node1", result)
}

// Benchmark tests

func BenchmarkGet(b *testing.B) {
	r := New[int, int]()
	for i := range 1000 {
		r.Register(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Get(i % 1000)
	}
}

func BenchmarkRegister(b *testing.B) {
	r := New[int, int]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Register(i, i)
	}
}

func BenchmarkGetOrCreate_Existing(b *testing.B) {
	r := New[int, int]()
	r.Register(0, 42)
	factory := func() int { return 42 }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.GetOrCreate(0, factory)
	}
}

func BenchmarkGetOrCreate_New(b *testing.B) {
	r := New[int, int]()
	factory := func() int { return 42 }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.GetOrCreate(i, factory)
	}
}

func BenchmarkConcurrentGet(b *testing.B) {
	r := New[int, int]()
	for i := range 1000 {
		r.Register(i, i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			r.Get(i % 1000)
			i++
		}
	})
}
