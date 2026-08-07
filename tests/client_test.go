package tests

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gomodule/redigo/redis"
	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
)

func subTestClient(g *testGroup) {
	g.regSubTest("OUTPUT", client_OUTPUT_test)
	g.regSubTest("CLIENT", client_CLIENT_test)
}

func client_OUTPUT_test(mc *mockServer) error {
	if err := mc.DoBatch(
		// tests removal of "elapsed" member.
		Do("OUTPUT", "json", "yaml").Err(`wrong number of arguments for 'output' command`),
		Do("OUTPUT", "json").Str(`{"ok":true}`),
		Do("OUTPUT").JSON().Str(`{"ok":true,"output":"json"}`),
		Do("OUTPUT").Str(`resp`), // this is due to the internal Do test
		Do("OUTPUT", "resp").OK(),
		Do("OUTPUT", "yaml").Err(`invalid argument 'yaml'`),
		Do("OUTPUT").Str(`resp`),
		Do("OUTPUT").JSON().Str(`{"ok":true,"output":"json"}`),
	); err != nil {
		return err
	}

	// run direct commands
	if _, err := mc.Do("OUTPUT", "json"); err != nil {
		return err
	}
	res, err := mc.Do("CLIENT", "list")
	if err != nil {
		return err
	}
	bres, ok := res.([]byte)
	if !ok {
		return errors.New("Failed to type assert CLIENT response")
	}
	sres := string(bres)
	if !gjson.Valid(sres) {
		return errors.New("CLIENT response was invalid")
	}
	info := gjson.Get(sres, "list").String()
	if !gjson.Valid(info) {
		return errors.New("CLIENT.list response was invalid")
	}
	return nil
}

func client_CLIENT_test(mc *mockServer) error {
	numConns := 20
	var conns []redis.Conn
	defer func() {
		for i := range conns {
			conns[i].Close()
		}
	}()
	for i := 0; i <= numConns; i++ {
		conn, err := redis.Dial("tcp", fmt.Sprintf(":%d", mc.port))
		if err != nil {
			return err
		}
		conn.Do("PING")
		conns = append(conns, conn)
	}

	_, err := conns[1].Do("CLIENT", "setname", "cl1")
	if err != nil {
		return err
	}
	_, err = conns[2].Do("CLIENT", "setname", "cl2")
	if err != nil {
		return err
	}

	if _, err := mc.Do("OUTPUT", "JSON"); err != nil {
		return err
	}
	res, err := mc.Do("CLIENT", "list")
	if err != nil {
		return err
	}
	bres, ok := res.([]byte)
	if !ok {
		return errors.New("Failed to type assert CLIENT response")
	}
	sres := string(pretty.Pretty(bres))
	if int(gjson.Get(sres, "list.#").Int()) < numConns {
		return errors.New("Invalid number of connections")
	}

	client13ID := gjson.Get(sres, "list.13.id").String()
	client14Addr := gjson.Get(sres, "list.14.addr").String()
	client15Addr := gjson.Get(sres, "list.15.addr").String()

	return mc.DoBatch(
		Do("CLIENT", "list").JSON().Func(func(s string) error {
			if int(gjson.Get(s, "list.#").Int()) < numConns {
				return errors.New("Invalid number of connections")
			}
			return nil
		}),
		Do("CLIENT", "list").Func(func(s string) error {
			if len(strings.Split(strings.TrimSpace(s), "\n")) < numConns {
				return errors.New("Invalid number of connections")
			}
			return nil
		}),
		Do("CLIENT").Err(`wrong number of arguments for 'client' command`),
		Do("CLIENT", "hello").Err(`Syntax error, try CLIENT (LIST | KILL | GETNAME | SETNAME)`),
		Do("CLIENT", "list", "arg3").Err(`wrong number of arguments for 'client' command`),
		Do("CLIENT", "getname", "arg3").Err(`wrong number of arguments for 'client' command`),
		Do("CLIENT", "getname").JSON().Str(`{"ok":true,"name":""}`),
		Do("CLIENT", "getname").Str(``),
		Do("CLIENT", "setname", "abc").OK(),
		Do("CLIENT", "getname").Str(`abc`),
		Do("CLIENT", "getname").JSON().Str(`{"ok":true,"name":"abc"}`),
		Do("CLIENT", "setname", "abc", "efg").Err(`wrong number of arguments for 'client' command`),
		Do("CLIENT", "setname", " abc ").Err(`Client names cannot contain spaces, newlines or special characters.`),
		Do("CLIENT", "setname", "abcd").JSON().OK(),
		Do("CLIENT", "kill", "name", "abcd").Err("No such client"),
		Do("CLIENT", "getname").Str(`abcd`),
		Do("CLIENT", "kill").Err(`wrong number of arguments for 'client' command`),
		Do("CLIENT", "kill", "").Err(`No such client`),
		Do("CLIENT", "kill", "abcd").Err(`No such client`),
		Do("CLIENT", "kill", "id", client13ID).OK(),
		Do("CLIENT", "kill", "id").Err("wrong number of arguments for 'client' command"),
		Do("CLIENT", "kill", client14Addr).OK(),
		Do("CLIENT", "kill", client14Addr, "yikes").Err("wrong number of arguments for 'client' command"),
		Do("CLIENT", "kill", "addr").Err("wrong number of arguments for 'client' command"),
		Do("CLIENT", "kill", "addr", client15Addr).JSON().OK(),
		Do("CLIENT", "kill", "addr", client14Addr, "yikes").Err("wrong number of arguments for 'client' command"),
		Do("CLIENT", "kill", "id", "1000").Err("No such client"),
	)

}
