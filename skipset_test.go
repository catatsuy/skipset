package skipset

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func Example() {
	l := NewInt()

	for _, v := range []int{10, 12, 15} {
		if l.Insert(v) {
			fmt.Println("skipset insert", v)
		}
	}

	if l.Contains(10) {
		fmt.Println("skipset contains 10")
	}

	l.Range(func(i int, score int) bool {
		fmt.Println("skipset range found ", score)
		return true
	})

	l.Delete(15)
	fmt.Printf("skipset contains %d items\r\n", l.Len())
}

type benchArrayCache struct {
	length      int
	itemMap     map[int64]struct{}
	Insert      []int64
	Check       []int64
	InvalidItem []int64
	Rnd37       []bool // 30% true, 70% false
	Rnd55       []bool // 50% true, 50% false
	Rnd9091     []int  // 90% ZERO, 9% ONE, 1% TWO

	count int64
	mu    sync.Mutex
}

func newBench(length int) *benchArrayCache {
	c := &benchArrayCache{}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.length = length
	c.Insert = make([]int64, length)
	c.Check = make([]int64, length)
	c.InvalidItem = make([]int64, length)
	c.Rnd37 = make([]bool, length)
	c.Rnd55 = make([]bool, length)
	c.Rnd9091 = make([]int, length)

	c.itemMap = make(map[int64]struct{}, length)
	for i := 0; i < c.length; i++ {
		c.itemMap[int64(i)] = struct{}{}
	}

	var i int
	for k := range c.itemMap {
		c.Insert[i] = k
		i++
	}
	i = 0

	for k := range c.itemMap {
		c.Check[i] = k
		i++
	}
	i = 0

	for i < length {
		c.InvalidItem[i] = int64(i + length)
		i++
	}

	for i := 0; i < length; i++ {
		nowRnd := rand.Intn(100)
		if nowRnd < 30 {
			c.Rnd37[i] = true
		}

		if nowRnd < 50 {
			c.Rnd55[i] = true
		}

		if nowRnd < 90 {
			c.Rnd9091[i] = 0
		} else if nowRnd == 99 {
			c.Rnd9091[i] = 2
		} else {
			c.Rnd9091[i] = 1
		}
	}
	return c
}

func (c *benchArrayCache) next() (n int64) {
	c.mu.Lock()
	n = c.count
	c.count++
	c.mu.Unlock()
	return n
}

func (c *benchArrayCache) rcount() {
	c.mu.Lock()
	c.count = 0
	c.mu.Unlock()
}

var benchArray = newBench(11 * 1000 * 1000)

func newSkipSet(num int) *Int64Set {
	l := NewInt64()
	var wg sync.WaitGroup
	for i := 0; i < num; i++ {
		i := i
		wg.Add(1)
		go func() {
			l.Insert(benchArray.Insert[i])
			wg.Done()
		}()
	}
	wg.Wait()
	return l
}

func newSyncMap(num int) sync.Map {
	var l sync.Map
	var wg sync.WaitGroup
	for i := 0; i < num; i++ {
		i := i
		wg.Add(1)
		go func() {
			l.Store(benchArray.Insert[i], nil)
			wg.Done()
		}()
	}
	wg.Wait()
	return l
}

func TestNewInt64(t *testing.T) {
	// Correctness.
	l := NewInt64()
	if l.length != 0 {
		t.Fatal("invalid length")
	}

	if !l.Insert(0) || l.length != 1 {
		t.Fatal("invalid insert")
	}
	if !l.Contains(0) {
		t.Fatal("invalid contains")
	}
	if !l.Delete(0) || l.length != 0 {
		t.Fatal("invalid delete")
	}

	// Concurrent insert.
	num := 1000000
	var wg sync.WaitGroup
	for i := 0; i < num; i++ {
		i := i
		wg.Add(1)
		go func() {
			l.Insert(benchArray.Insert[i])
			wg.Done()
		}()
	}
	wg.Wait()
	if l.length != int64(num) {
		t.Fatalf("invalid length expected %d, got %d", num, l.length)
	}

	// Concurrent contains.
	for i := 0; i < num; i++ {
		i := i
		wg.Add(1)
		go func() {
			if !l.Contains(benchArray.Insert[i]) {
				wg.Done()
				t.Fatalf("insert dosen't contains %d", i)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	// Concurrent delete.
	for i := 0; i < num; i++ {
		i := i
		wg.Add(1)
		go func() {
			if !l.Delete(benchArray.Insert[i]) {
				wg.Done()
				t.Fatalf("can't delete %d", i)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if l.length != 0 {
		t.Fatalf("invalid length expected %d, got %d", 0, l.length)
	}

	// Test all methods.
	for i := 0; i < num; i++ {
		wg.Add(1)
		go func() {
			r := rand.Intn(1000)
			if r == 0 {
				r = 1
			}
			if r < 333 {
				l.Insert(benchArray.Insert[rand.Intn(num)])
			} else if r < 666 {
				l.Contains(benchArray.Insert[rand.Intn(num)])
			} else if r != 999 {
				l.Delete(benchArray.Insert[rand.Intn(num)])
			} else {
				l.Range(func(i int, score int64) bool {
					if score == 0 { // default header and tail score
						t.Fatal("invalid content")
					}
					return true
				})
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkInsert_SkipSet(b *testing.B) {
	l := NewInt64()
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Insert(benchArray.Insert[benchArray.next()])
		}
	})
}

func BenchmarkInsert_SyncMap(b *testing.B) {
	var l sync.Map
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Store(benchArray.Insert[benchArray.next()], nil)
		}
	})
}

func Benchmark50Insert50Contains_SkipSet(b *testing.B) {
	l := newSkipSet(1000)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := benchArray.next()
			if benchArray.Rnd55[u] == true {
				l.Insert(benchArray.Insert[u])
			} else {
				l.Contains(benchArray.Check[u])
			}
		}
	})
}

func Benchmark50Insert50Contains_SyncMap(b *testing.B) {
	l := newSyncMap(1000)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := benchArray.next()
			if benchArray.Rnd55[u] == true {
				l.Store(benchArray.Insert[u], nil)
			} else {
				l.Load(benchArray.Check[u])
			}
		}
	})
}

func Benchmark30Insert70Contains_SkipSet(b *testing.B) {
	l := newSkipSet(1000)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := benchArray.next()
			if benchArray.Rnd37[u] == true {
				l.Insert(benchArray.Insert[u])
			} else {
				l.Contains(benchArray.Check[u])
			}
		}
	})
}

func Benchmark30Insert70Contains_SyncMap(b *testing.B) {
	l := newSyncMap(1000)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := benchArray.next()
			if benchArray.Rnd37[u] == true {
				l.Store(benchArray.Insert[u], nil)
			} else {
				l.Load(benchArray.Insert[u])
			}
		}
	})
}

func Benchmark1Delete9Insert90Contains_SkipSet(b *testing.B) {
	l := newSkipSet(1000)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := benchArray.next()
			if benchArray.Rnd9091[u] == 1 {
				l.Insert(benchArray.Insert[u])
			} else if benchArray.Rnd9091[u] == 2 {
				l.Delete(benchArray.Check[u])
			} else {
				l.Contains(benchArray.Check[u])
			}
		}
	})
}

func Benchmark1Delete9Insert90Contains_SyncMap(b *testing.B) {
	l := newSyncMap(1000)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			u := benchArray.next()
			if benchArray.Rnd9091[u] == 1 {
				l.Store(benchArray.Insert[u], nil)
			} else if benchArray.Rnd9091[u] == 2 {
				l.Delete(benchArray.Check[u])
			} else {
				l.Load(benchArray.Check[u])
			}
		}
	})
}

func BenchmarkRange_SkipSet(b *testing.B) {
	l := newSkipSet(1000)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Range(func(i int, score int64) bool {
				return true
			})
		}
	})
}

func BenchmarkRange_SyncMap(b *testing.B) {
	l := newSyncMap(1000)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Range(func(key, value interface{}) bool {
				return true
			})
		}
	})
}

func BenchmarkContains_SkipSet(b *testing.B) {
	num := 100000
	l := newSkipSet(num)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Contains(benchArray.Insert[rand.Intn(num*2)])
		}
	})
}

func BenchmarkContains_SyncMap(b *testing.B) {
	num := 100000
	l := newSyncMap(num)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Load(benchArray.Insert[rand.Intn(num*2)])
		}
	})
}

func BenchmarkDelete_100Valid_SkipSet(b *testing.B) {
	num := 100000
	l := newSkipSet(num)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if benchArray.count >= int64(num) {
				continue
			}
			l.Delete(benchArray.Insert[benchArray.next()])
		}
	})
}

func BenchmarkDelete_100Valid_SyncMap(b *testing.B) {
	num := 100000
	l := newSyncMap(num)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if benchArray.count >= int64(num) {
				continue
			}
			l.Delete(benchArray.Insert[benchArray.next()])
		}
	})
}

func BenchmarkDelete_50Valid_SkipSet(b *testing.B) {
	num := 100000
	l := newSkipSet(num)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if benchArray.count >= int64(num) {
				continue
			}
			l.Delete(benchArray.Insert[rand.Intn(num*2)])
		}
	})
}

func BenchmarkDelete_50Valid_SyncMap(b *testing.B) {
	num := 100000
	l := newSyncMap(num)
	defer benchArray.rcount()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if benchArray.count >= int64(num) {
				continue
			}
			l.Delete(benchArray.Insert[rand.Intn(num*2)])
		}
	})
}
