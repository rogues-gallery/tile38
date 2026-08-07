package tests

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gomodule/redigo/redis"
)

func subTestMonitor(g *testGroup) {
	g.regSubTest("monitor", follower_monitor_test)
}

func follower_monitor_test(mc *mockServer) error {
	N := 1000
	ch := make(chan error)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ch <- func() error {
			conn, err := redis.Dial("tcp", fmt.Sprintf("localhost:%d", mc.port))
			if err != nil {
				wg.Done()
				return err
			}
			defer conn.Close()
			s, err := redis.String(conn.Do("MONITOR"))
			if err != nil {
				wg.Done()
				return err
			}
			if s != "OK" {
				wg.Done()
				return fmt.Errorf("expected '%s', got '%s'", "OK", s)
			}
			wg.Done()

			for i := 0; i < N; i++ {
				s, err := redis.String(conn.Receive())
				if err != nil {
					return err
				}
				ex := fmt.Sprintf(`"mykey" "%d"`, i)
				if !strings.Contains(s, ex) {
					return fmt.Errorf("expected '%s', got '%s'", ex, s)
				}
			}
			return nil
		}()
	}()

	wg.Wait()

	conn, err := redis.Dial("tcp", fmt.Sprintf("localhost:%d", mc.port))
	if err != nil {
		return err
	}
	defer conn.Close()

	for i := 0; i < N; i++ {
		s, err := redis.String(conn.Do("SET", "mykey", i, "POINT", 10, 10))
		if err != nil {
			return err
		}
		if s != "OK" {
			return fmt.Errorf("expected '%s', got '%s'", "OK", s)
		}
	}

	err = <-ch
	if err != nil {
		err = fmt.Errorf("monitor client: %w", err)
	}

	return err
}
