// Package shared allows for
package sstring

import (
	"sync"
	"unsafe"

	"github.com/tidwall/hashmap"
)

var mu sync.Mutex
var nums hashmap.Map[string, int]
var strs []string

// Load a shared string from its number.
// Panics when there is no string assigned with that number.
func Load(num int) (str string) {
	mu.Lock()
	if num >= 0 && num < len(strs) {
		str = strs[num]
		mu.Unlock()
		return str
	}
	mu.Unlock()
	panic("string not found")
}

// Store a shared string.
// Returns a unique number that can be used to load the string later.
// The number is al
func Store(str string) (num int) {
	mu.Lock()
	var ok bool
	num, ok = nums.Get(str)
	if !ok {
		// Make a copy of the string to ensure we don't take in slices.
		b := make([]byte, len(str))
		copy(b, str)
		str = *(*string)(unsafe.Pointer(&b))
		num = len(strs)
		strs = append(strs, str)
		nums.Set(str, num)
	}
	mu.Unlock()
	return num
}

// Len returns the number of shared strings
func Len() int {
	mu.Lock()
	n := len(strs)
	mu.Unlock()
	return n
}
