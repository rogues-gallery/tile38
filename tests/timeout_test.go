package tests

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
)

func subTestTimeout(g *testGroup) {
	g.regSubTest("spatial", timeout_spatial_test)
	g.regSubTest("search", timeout_search_test)
	g.regSubTest("scripts", timeout_scripts_test)
	g.regSubTest("no writes", timeout_no_writes_test)
	g.regSubTest("within scripts", timeout_within_scripts_test)
	g.regSubTest("no writes within scripts", timeout_no_writes_within_scripts_test)
}

func setup(mc *mockServer, count int, points bool) (err error) {
	rand.Seed(time.Now().UnixNano())

	// add a bunch of points
	for i := 0; i < count; i++ {
		val := fmt.Sprintf("val:%d", i)
		var resp string
		var lat, lon, fval float64
		fval = rand.Float64()
		if points {
			lat = rand.Float64()*180 - 90
			lon = rand.Float64()*360 - 180
			resp, err = redis.String(mc.conn.Do("SET",
				"mykey", val,
				"FIELD", "foo", fval,
				"POINT", lat, lon))
		} else {
			resp, err = redis.String(mc.conn.Do("SET",
				"mykey", val,
				"FIELD", "foo", fval,
				"STRING", val))
		}
		if err != nil {
			return
		}
		if resp != "OK" {
			err = fmt.Errorf("expected 'OK', got '%s'", resp)
			return
		}
		time.Sleep(time.Nanosecond)
	}
	time.Sleep(time.Second * 3)
	return
}

func timeout_spatial_test(mc *mockServer) error {
	err := setup(mc, 10000, true)
	if err != nil {
		return err
	}
	return mc.DoBatch(
		Do("SCAN", "mykey", "WHERE", "foo", -1, 2, "COUNT").Str("10000"),
		Do("INTERSECTS", "mykey", "WHERE", "foo", -1, 2, "COUNT", "BOUNDS", -90, -180, 90, 180).Str("10000"),
		Do("WITHIN", "mykey", "WHERE", "foo", -1, 2, "COUNT", "BOUNDS", -90, -180, 90, 180).Str("10000"),

		Do("TIMEOUT", "0.000001", "SCAN", "mykey", "WHERE", "foo", -1, 2, "COUNT").Err("timeout"),
		Do("TIMEOUT", "0.000001", "INTERSECTS", "mykey", "WHERE", "foo", -1, 2, "COUNT", "BOUNDS", -90, -180, 90, 180).Err("timeout"),
		Do("TIMEOUT", "0.000001", "WITHIN", "mykey", "WHERE", "foo", -1, 2, "COUNT", "BOUNDS", -90, -180, 90, 180).Err("timeout"),
	)
}

func timeout_search_test(mc *mockServer) (err error) {
	err = setup(mc, 10000, false)
	if err != nil {
		return err
	}

	return mc.DoBatch([][]interface{}{
		{"SEARCH", "mykey", "MATCH", "val:*", "COUNT"}, {"10000"},
		{"TIMEOUT", "0.000001", "SEARCH", "mykey", "MATCH", "val:*", "COUNT"}, {"ERR timeout"},
	})
}

func timeout_scripts_test(mc *mockServer) (err error) {
	script := `
		local clock = os.clock
		local function sleep(n)
			local t0 = clock()
			while clock() - t0 <= n do end
		end
		sleep(0.5)
	`
	sha := "e3ce9449853a622327f30c727a6e086ccd91d9d4"

	return mc.DoBatch([][]interface{}{
		{"SCRIPT LOAD", script}, {sha},

		{"EVALSHA", sha, 0}, {nil},
		{"EVALROSHA", sha, 0}, {nil},
		{"EVALNASHA", sha, 0}, {nil},

		{"TIMEOUT", "0.1", "EVALSHA", sha, 0}, {"ERR timeout"},
		{"TIMEOUT", "0.1", "EVALROSHA", sha, 0}, {"ERR timeout"},
		{"TIMEOUT", "0.1", "EVALNASHA", sha, 0}, {"ERR timeout"},

		{"TIMEOUT", "0.9", "EVALSHA", sha, 0}, {nil},
		{"TIMEOUT", "0.9", "EVALROSHA", sha, 0}, {nil},
		{"TIMEOUT", "0.9", "EVALNASHA", sha, 0}, {nil},
	})
}

func timeout_no_writes_test(mc *mockServer) (err error) {
	return mc.DoBatch([][]interface{}{
		{"SET", "mykey", "myid", "STRING", "foo"}, {"OK"},
		{"TIMEOUT", 1, "SET", "mykey", "myid", "STRING", "foo"}, {"ERR timeout not supported for 'set'"},
	})
}

func scriptTimeoutErr(v interface{}) (resp, expect interface{}) {
	s := fmt.Sprintf("%v", v)
	if strings.Contains(s, "ERR timeout") {
		return v, v
	}
	return v, "A lua stack containing 'ERR timeout'"
}

func timeout_within_scripts_test(mc *mockServer) (err error) {
	err = setup(mc, 10000, true)
	if err != nil {
		return err
	}

	script1 := "return tile38.call('timeout', 10, 'SCAN', 'mykey', 'WHERE', 'foo', -1, 2, 'COUNT')"
	script2 := "return tile38.call('timeout', 0.000001, 'SCAN', 'mykey', 'WHERE', 'foo', -1, 2, 'COUNT')"
	sha1 := "27a364b4e46ef493f6b70371086c286e2d5b5f49"
	sha2 := "2da9c05b54abfe870bdc8383a143f9d3aa656192"

	return mc.DoBatch([][]interface{}{
		{"SCRIPT LOAD", script1}, {sha1},
		{"SCRIPT LOAD", script2}, {sha2},

		{"EVALSHA", sha1, 0}, {"10000"},
		{"EVALROSHA", sha1, 0}, {"10000"},
		{"EVALNASHA", sha1, 0}, {"10000"},
		{"EVALSHA", sha2, 0}, {scriptTimeoutErr},
		{"EVALROSHA", sha2, 0}, {scriptTimeoutErr},
		{"EVALNASHA", sha2, 0}, {scriptTimeoutErr},
	})
}

func scriptTimeoutNotSupportedErr(v interface{}) (resp, expect interface{}) {
	s := fmt.Sprintf("%v", v)
	if strings.Contains(s, "ERR timeout not supported for") {
		return v, v
	}
	return v, "A lua stack containing 'ERR timeout not supported for'"
}

func timeout_no_writes_within_scripts_test(mc *mockServer) (err error) {
	script1 := "return tile38.call('SET', 'mykey', 'myval', 'STRING', 'foo')"
	script2 := "return tile38.call('timeout', 10, 'SET', 'mykey', 'myval', 'STRING', 'foo')"
	sha1 := "393d0adff113fdda45e3b5aff93c188c30099f48"
	sha2 := "5287c158d15eb53d800b7389d82df0d73b004bf1"

	return mc.DoBatch([][]interface{}{
		{"SCRIPT LOAD", script1}, {sha1},
		{"SCRIPT LOAD", script2}, {sha2},
		{"EVALSHA", sha1, 0, "foo"}, {"OK"},
		{"EVALSHA", sha2, 0, "foo"}, {scriptTimeoutNotSupportedErr},
	})
}
