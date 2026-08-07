// Test Tile38 for Expiration Drift
// Issue #616

package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/tidwall/btree"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const exsecs = 10
const key = "__issue_616__"

func makeID() string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz-"
	var buf [10]byte
	rand.Read(buf[:])
	for i := 0; i < len(buf); i++ {
		buf[i] = chars[int(buf[i])%len(chars)]
	}
	return string(buf[:])
}

func main() {
	fmt.Printf(
		"The SCAN and ACTUAL values should reach about 1850 and stay\n" +
			"roughly the same from there on.\n")
	var mu sync.Mutex
	objs := btree.NewNonConcurrent(func(a, b interface{}) bool {
		ajson := a.(string)
		bjson := b.(string)
		return gjson.Get(ajson, "id").String() < gjson.Get(bjson, "id").String()
	})
	expires := btree.NewNonConcurrent(func(a, b interface{}) bool {
		ajson := a.(string)
		bjson := b.(string)
		if gjson.Get(ajson, "properties.ex").Int() < gjson.Get(bjson, "properties.ex").Int() {
			return true
		}
		if gjson.Get(ajson, "properties.ex").Int() > gjson.Get(bjson, "properties.ex").Int() {
			return false
		}
		return gjson.Get(ajson, "id").String() < gjson.Get(bjson, "id").String()
	})

	conn := must(redis.Dial("tcp", ":9851")).(redis.Conn)
	must(conn.Do("DROP", key))
	must(nil, conn.Close())

	go func() {
		conn := must(redis.Dial("tcp", ":9851")).(redis.Conn)
		defer conn.Close()
		for {
			ex := time.Now().UnixNano() + int64(exsecs*time.Second)
			for i := 0; i < 10; i++ {
				id := makeID()
				x := rand.Float64()*360 - 180
				y := rand.Float64()*180 - 90
				obj := fmt.Sprintf(`{"type":"Feature","geometry":{"type":"Point","coordinates":[%f,%f]},"properties":{}}`, x, y)
				obj, _ = sjson.Set(obj, "properties.ex", ex)
				obj, _ = sjson.Set(obj, "id", id)
				res := must(redis.String(conn.Do("SET", key, id, "ex", exsecs, "OBJECT", obj))).(string)
				if res != "OK" {
					panic(fmt.Sprintf("expected 'OK', got '%s'", res))
				}
				mu.Lock()
				prev := objs.Set(obj)
				if prev != nil {
					expires.Delete(obj)
				}
				expires.Set(obj)
				mu.Unlock()
			}
			time.Sleep(time.Second / 20)
		}
	}()

	go func() {
		conn := must(redis.Dial("tcp", ":9851")).(redis.Conn)
		defer conn.Close()
		for {
			time.Sleep(time.Second * 5)
			must(conn.Do("AOFSHRINK"))
		}
	}()

	go func() {
		conn := must(redis.Dial("tcp", ":9851")).(redis.Conn)
		defer conn.Close()
		must(conn.Do("OUTPUT", "JSON"))
		for {
			time.Sleep(time.Second / 10)
			var ids []string
			res := must(redis.String(conn.Do("SCAN", key, "LIMIT", 100000000))).(string)
			gjson.Get(res, "objects").ForEach(func(_, res gjson.Result) bool {
				ids = append(ids, res.Get("id").String())
				return true
			})
			now := time.Now().UnixNano()
			mu.Lock()
			var exobjs []string
			expires.Ascend(nil, func(v interface{}) bool {
				ex := gjson.Get(v.(string), "properties.ex").Int()
				if ex > now {
					return false
				}
				exobjs = append(exobjs, v.(string))
				return true
			})
			for _, obj := range exobjs {
				objs.Delete(obj)
				expires.Delete(obj)
			}
			fmt.Printf("\rSCAN: %d, ACTUAL: %d ", len(ids), objs.Len())
			mu.Unlock()
		}
	}()
	select {}
}

func must(v interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return v
}
