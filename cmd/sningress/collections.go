package main

import "sync"

type Optional[T comparable] []T

func (l Optional[T]) Contains(v T) bool {
	for _, lv := range l {
		if lv == v {
			return true
		}
	}
	return false
}

func OptionalOf[T comparable](t T) Optional[T] {
	return Optional[T]([]T{t})
}

type Set[K comparable] map[K]struct{}

func EmptySet[K comparable]() Set[K] {
	return Set[K](nil)
}

func NewSet[K comparable]() Set[K] {
	return Set[K](make(map[K]struct{}))
}

func SetOf[K comparable](items ...K) Set[K] {
	m := make(map[K]struct{}, len(items))
	for _, item := range items {
		m[item] = struct{}{}
	}
	return Set[K](m)
}

func (s Set[K]) Contains(k K) bool {
	_, found := s[k]
	return found
}

func MapToSet[T any, K comparable](slice []T, fn func(t *T) (K, bool)) Set[K] {
	values := make([]K, 0, len(slice))
	for i := range slice {
		k, ok := fn(&slice[i])
		if ok {
			values = append(values, k)
		}
	}
	return SetOf(values...)
}

// type Iterable[K any] interface {
// 	Next() (K, bool)
// }

// type Result[T any] struct {
// 	Value T
// 	Err   error
// }

type MultiMap[K comparable, V any] struct {
	// We use an ImmutableList so we don't need a defensive copy
	m map[K]ImmutableList[V]
}

func NewMultiMap[K comparable, V any]() MultiMap[K, V] {
	return MultiMap[K, V]{
		m: make(map[K]ImmutableList[V]),
	}
}

func (m *MultiMap[K, V]) Get(k K) ImmutableList[V] {
	return m.m[k]
}

func (m *MultiMap[K, V]) Update(oldKeys, newKeys Set[K], endpoints V, valueIdentity func(v1, v2 V) bool) {
	for k := range newKeys {
		if oldKeys.Contains(k) {
			m.m[k] = m.m[k].Replace(endpoints, valueIdentity)
		} else {
			m.m[k] = m.m[k].Add(endpoints)
		}
	}

	for k := range oldKeys {
		if newKeys.Contains(k) {
			// Already done
			continue
		}

		m.m[k] = m.m[k].Remove(endpoints, valueIdentity)
	}
}

type ImmutableList[T any] []T

func ImmutableListOf[T any](values ...T) ImmutableList[T] {
	return ImmutableList[T](values)
}

func (l ImmutableList[T]) Remove(removeV T, equals func(t1, t2 T) bool) ImmutableList[T] {
	newList := make([]T, 0, len(l)-1)
	for _, v := range l {
		if !equals(v, removeV) {
			newList = append(newList, v)
		}
	}
	return newList
}

func (l ImmutableList[T]) Add(addV T) ImmutableList[T] {
	newList := make([]T, 0, len(l)+1)

	newList = append(newList, l...)
	newList = append(newList, addV)

	return ImmutableList[T](newList)
}

func (l ImmutableList[T]) Replace(replaceV T, equals func(t1, t2 T) bool) ImmutableList[T] {
	newList := make([]T, 0, len(l)-1)
	for _, v := range l {
		if !equals(v, replaceV) {
			newList = append(newList, v)
		} else {
			newList = append(newList, replaceV)
		}
	}
	return ImmutableList[T](newList)
}

func MapAndCompare[T any, U comparable](mapper func(T) U) func(l, r T) bool {
	return func(t1, t2 T) bool {
		u1 := mapper(t1)
		u2 := mapper(t2)
		return u1 == u2
	}
}

// func CompareEquals[V comparable](l, r V) bool {
// 	return l == r
// }

// type Func[T, V any] func(t T) V

// func Compose[T, U, V any](fn1 Func[T, U], fn2 Func[U, V]) Func[T, V] {
// 	composed := func(t T) V {
// 		u := fn1(t)
// 		v := fn2(u)
// 		return v
// 	}
// 	return composed
// }

// func (f Func[T, U]) [V any]Compose(f2 Func[U, V]) Func[T, V] {
// 	return Compose(f, f2)
// }

func Diff[T comparable](l []T, r []T) (onlyLeft, both, onlyRight []T) {
	// TODO: switch to maps if it gets beyond a certain size?

	doneR := make([]bool, len(r))
nextL:
	for _, vL := range l {
		for iR, vR := range r {
			if vR == vL {
				both = append(both, vL)
				doneR[iR] = true
				continue nextL
			}
		}
		onlyLeft = append(onlyLeft, vL)
	}

	for iR, vR := range r {
		if doneR[iR] {
			continue
		}
		onlyRight = append(onlyRight, vR)
	}
	return
}

type MapIndex[MapK comparable, MapV any, IndexV comparable] struct {
	mutex sync.RWMutex
	// indexFunction func(v V) []U
	m map[IndexV]map[MapK]MapV
}

func AddIndex[MapK comparable, MapV any, IndexV comparable](m *IndexedMap[MapK, MapV], fn func(v MapV) []IndexV) *MapIndex[MapK, MapV, IndexV] {
	index := &MapIndex[MapK, MapV, IndexV]{
		m: make(map[IndexV]map[MapK]MapV),
	}

	indexer := func(mapK MapK, oldV, newV MapV) {
		oldKeys := fn(oldV)
		newKeys := fn(newV)

		onlyOld, both, onlyNew := Diff(oldKeys, newKeys)

		for _, k := range onlyOld {
			delete(index.m[k], mapK)
		}
		for _, k := range onlyNew {
			m := index.m[k]
			if m == nil {
				m = make(map[MapK]MapV)
				index.m[k] = m
			}
			m[mapK] = newV
		}

		for _, k := range both {
			m := index.m[k]
			if m == nil {
				m = make(map[MapK]MapV)
				index.m[k] = m
			}
			m[mapK] = newV
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.indexers = append(m.indexers, indexer)
	return index
}

func (m *MapIndex[MapK, MapV, IndexV]) Lookup(k IndexV) ImmutableList[MapV] {
	var values []MapV
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	matches := m.m[k]
	for _, v := range matches {
		values = append(values, v)
	}
	return ImmutableListOf(values...)
}

type IndexedMap[K comparable, V any] struct {
	mutex    sync.Mutex
	m        map[K]V
	indexers []func(k K, oldV, newV V)
}

func (m *IndexedMap[K, V]) Put(k K, newV V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.m == nil {
		m.m = make(map[K]V)
	}
	oldV := m.m[k]

	for _, indexer := range m.indexers {
		indexer(k, oldV, newV)
	}
	m.m[k] = newV
}

func (m *IndexedMap[K, V]) Delete(k K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.m == nil {
		m.m = make(map[K]V)
	}
	oldV := m.m[k]

	for _, indexer := range m.indexers {
		var emptyV V
		indexer(k, oldV, emptyV)
	}
	delete(m.m, k)
}
