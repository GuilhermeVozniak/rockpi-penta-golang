package config

import "sync/atomic"

// AtomicInt is a simple wrapper around int64 for atomic operations.
type AtomicInt struct {
	value int64
}

// NewAtomicInt creates a new AtomicInt.
func NewAtomicInt(initialValue int) *AtomicInt {
	return &AtomicInt{value: int64(initialValue)}
}

// Load atomically reads the value.
func (ai *AtomicInt) Load() int {
	return int(atomic.LoadInt64(&ai.value))
}

// Store atomically writes the value.
func (ai *AtomicInt) Store(newValue int) {
	atomic.StoreInt64(&ai.value, int64(newValue))
}

// Add atomically adds delta to the value and returns the new value.
func (ai *AtomicInt) Add(delta int) int {
	return int(atomic.AddInt64(&ai.value, int64(delta)))
}

// AtomicBool is a simple wrapper around int32 for atomic boolean operations (0 or 1).
type AtomicBool struct {
	value int32
}

// NewAtomicBool creates a new AtomicBool.
func NewAtomicBool(initialValue bool) *AtomicBool {
	var v int32
	if initialValue {
		v = 1
	}
	return &AtomicBool{value: v}
}

// Load atomically reads the boolean value.
func (ab *AtomicBool) Load() bool {
	return atomic.LoadInt32(&ab.value) == 1
}

// Store atomically writes the boolean value.
func (ab *AtomicBool) Store(newValue bool) {
	var v int32
	if newValue {
		v = 1
	}
	atomic.StoreInt32(&ab.value, v)
}

// Swap atomically sets the value and returns the old value.
func (ab *AtomicBool) Swap(newValue bool) bool {
	var v int32
	if newValue {
		v = 1
	}
	return atomic.SwapInt32(&ab.value, v) == 1
}

// Toggle atomically flips the boolean value and returns the new value.
func (ab *AtomicBool) Toggle() bool {
	for {
		old := ab.Load()
		var newIntVal int32
		if !old {
			newIntVal = 1
		}
		if atomic.CompareAndSwapInt32(&ab.value, atomic.LoadInt32(&ab.value), newIntVal) {
			return !old
		}
	}
}
