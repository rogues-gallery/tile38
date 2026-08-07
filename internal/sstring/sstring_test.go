package sstring

import (
	"math/rand"
	"testing"
	"time"

	"github.com/tidwall/assert"
)

func TestShared(t *testing.T) {
	for i := -1; i < 10; i++ {
		var str string
		func() {
			defer func() {
				assert.Assert(recover().(string) == "string not found")
			}()
			str = Load(i)
		}()
		assert.Assert(str == "")
	}
	assert.Assert(Store("hello") == 0)
	assert.Assert(Store("") == 1)
	assert.Assert(Store("jello") == 2)
	assert.Assert(Store("hello") == 0)
	assert.Assert(Store("") == 1)
	assert.Assert(Store("jello") == 2)
	str := Load(0)
	assert.Assert(str == "hello")
	str = Load(1)
	assert.Assert(str == "")
	str = Load(2)
	assert.Assert(str == "jello")

	assert.Assert(Len() == 3)

}

func randStr(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	for i := 0; i < n; i++ {
		b[i] = 'a' + b[i]%26
	}
	return string(b)
}

func BenchmarkStore(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	wmap := make(map[string]bool, b.N)
	for len(wmap) < b.N {
		wmap[randStr(10)] = true
	}
	words := make([]string, 0, b.N)
	for word := range wmap {
		words = append(words, word)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Store(words[i])
	}
}

func BenchmarkLoad(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	wmap := make(map[string]bool, b.N)
	for len(wmap) < b.N {
		wmap[randStr(10)] = true
	}
	words := make([]string, 0, b.N)
	for word := range wmap {
		words = append(words, word)
	}
	var nums []int
	for i := 0; i < b.N; i++ {
		nums = append(nums, Store(words[i]))
	}
	rand.Shuffle(len(nums), func(i, j int) {
		nums[i], nums[j] = nums[j], nums[i]
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Load(nums[i])
	}
}
