package tests

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/tidwall/sjson"
	"github.com/tidwall/tile38/internal/log"
	"github.com/tidwall/tile38/internal/server"
)

var errTimeout = errors.New("timeout")

func mockCleanup(silent bool) {
	if !silent {
		fmt.Printf("Cleanup: may take some time... ")
	}
	files, _ := os.ReadDir(".")
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "data-mock-") {
			os.RemoveAll(file.Name())
		}
	}
	if !silent {
		fmt.Printf("OK\n")
	}
}

type mockServer struct {
	closed   bool
	port     int
	mport    int
	conn     redis.Conn
	ioJSON   bool
	dir      string
	shutdown chan bool
}

func (mc *mockServer) readAOF() ([]byte, error) {
	return os.ReadFile(filepath.Join(mc.dir, "appendonly.aof"))
}

func (mc *mockServer) metricsPort() int {
	return mc.mport
}

type MockServerOptions struct {
	AOFFileName string
	AOFData     []byte
	Silent      bool
	Metrics     bool
}

var nextPort int32 = 10000

func getNextPort() int {
	// choose a valid port between 10000-50000
	for {
		port := int(atomic.AddInt32(&nextPort, 1))
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
}

func mockOpenServer(opts MockServerOptions) (*mockServer, error) {

	logOutput := io.Discard
	if os.Getenv("PRINTLOG") == "1" {
		logOutput = os.Stderr
		log.SetLevel(3)
	}
	log.SetOutput(logOutput)

	rand.Seed(time.Now().UnixNano())
	port := getNextPort()
	dir := fmt.Sprintf("data-mock-%d", port)
	if !opts.Silent {
		fmt.Printf("Starting test server at port %d\n", port)
	}
	if len(opts.AOFData) > 0 {
		if opts.AOFFileName == "" {
			opts.AOFFileName = "appendonly.aof"
		}
		if err := os.MkdirAll(dir, 0777); err != nil {
			return nil, err
		}
		err := os.WriteFile(filepath.Join(dir, opts.AOFFileName),
			opts.AOFData, 0666)
		if err != nil {
			return nil, err
		}
	}

	shutdown := make(chan bool)
	s := &mockServer{port: port, dir: dir, shutdown: shutdown}
	if opts.Metrics {
		s.mport = getNextPort()
	}
	var ferr atomic.Pointer[error] // ferr for when the server fails to start
	go func() {
		sopts := server.Options{
			Host:              "localhost",
			Port:              port,
			Dir:               dir,
			UseHTTP:           true,
			DevMode:           true,
			AppendOnly:        true,
			Shutdown:          shutdown,
			ShowDebugMessages: true,
		}
		if opts.Metrics {
			sopts.MetricsAddr = fmt.Sprintf(":%d", s.mport)
		}
		err := server.Serve(sopts)
		if err != nil {
			ferr.CompareAndSwap(nil, &err)
		}
	}()
	if err := s.waitForStartup(&ferr); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *mockServer) waitForStartup(ferr *atomic.Pointer[error]) error {
	var lerr error
	start := time.Now()
	for {
		if perr := ferr.Load(); perr != nil {
			return *perr
		}
		if time.Since(start) > time.Second*5 {
			if lerr != nil {
				return lerr
			}
			return errTimeout
		}
		resp, err := redis.String(s.Do("SET", "please", "allow", "POINT", "33", "-115"))
		if err != nil {
			lerr = err
		} else if resp != "OK" {
			lerr = errors.New("not OK")
		} else {
			resp, err := redis.Int(s.Do("DEL", "please", "allow"))
			if err != nil {
				lerr = err
			} else if resp != 1 {
				lerr = errors.New("not 1")
			} else {
				return nil
			}
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func (mc *mockServer) Close() {
	if mc == nil || mc.closed {
		return
	}
	mc.closed = true
	mc.shutdown <- true
	if mc.conn != nil {
		mc.conn.Close()
	}
	if mc.dir != "" {
		os.RemoveAll(mc.dir)
	}
}

func (mc *mockServer) ResetConn() {
	if mc.conn != nil {
		mc.conn.Close()
		mc.conn = nil
	}
}

func (s *mockServer) DoPipeline(cmds [][]interface{}) ([]interface{}, error) {
	if s.conn == nil {
		var err error
		s.conn, err = redis.Dial("tcp", fmt.Sprintf(":%d", s.port))
		if err != nil {
			return nil, err
		}
	}
	//defer conn.Close()
	for _, cmd := range cmds {
		if err := s.conn.Send(cmd[0].(string), cmd[1:]...); err != nil {
			return nil, err
		}
	}
	if err := s.conn.Flush(); err != nil {
		return nil, err
	}
	var resps []interface{}
	for i := 0; i < len(cmds); i++ {
		resp, err := s.conn.Receive()
		if err != nil {
			resps = append(resps, err)
		} else {
			resps = append(resps, resp)
		}
	}
	return resps, nil
}
func (s *mockServer) Do(commandName string, args ...interface{}) (interface{}, error) {
	resps, err := s.DoPipeline([][]interface{}{
		append([]interface{}{commandName}, args...),
	})
	if err != nil {
		return nil, err
	}
	if len(resps) != 1 {
		return nil, errors.New("invalid number or responses")
	}
	return resps[0], nil
}

func (mc *mockServer) DoBatch(commands ...interface{}) error {
	// Probe for I/O tests
	if len(commands) > 0 {
		if _, ok := commands[0].(*IO); ok {
			var cmds []*IO
			// If the first is an I/O test then all must be
			for _, cmd := range commands {
				if cmd, ok := cmd.(*IO); ok {
					cmds = append(cmds, cmd)
				} else {
					return errors.New("DoBatch cannot mix I/O tests with other kinds")
				}
			}
			for i, cmd := range cmds {
				if err := mc.doIOTest(i, cmd); err != nil {
					return err
				}
			}
			return nil
		}
	}

	var tag string
	for _, commands := range commands {
		switch commands := commands.(type) {
		case string:
			tag = commands
		case [][]interface{}:
			for i := 0; i < len(commands); i += 2 {
				cmds := commands[i]
				if dur, ok := cmds[0].(time.Duration); ok {
					time.Sleep(dur)
				} else {
					if err := mc.DoExpect(commands[i+1], cmds[0].(string), cmds[1:]...); err != nil {
						if tag == "" {
							return fmt.Errorf("batch[%d]: %v", i/2, err)
						} else {
							return fmt.Errorf("batch[%d][%v]: %v", i/2, tag, err)
						}
					}
				}
			}
			tag = ""
		case *IO:
			return errors.New("DoBatch cannot mix I/O tests with other kinds")
		default:
			return fmt.Errorf("Unknown command input")
		}
	}
	return nil
}

func normalize(v interface{}) interface{} {
	switch v := v.(type) {
	default:
		return v
	case []interface{}:
		for i := 0; i < len(v); i++ {
			v[i] = normalize(v[i])
		}
	case []uint8:
		return string(v)
	}
	return v
}
func (mc *mockServer) DoExpect(expect interface{}, commandName string, args ...interface{}) error {
	if v, ok := expect.([]interface{}); ok {
		expect = v[0]
	}
	resp, err := mc.Do(commandName, args...)
	if err != nil {
		if exs, ok := expect.(string); ok {
			if err.Error() == exs {
				return nil
			}
		}
		return err
	}
	if b, ok := resp.([]byte); ok && len(b) > 1 && b[0] == '{' {
		b, err = sjson.DeleteBytes(b, "elapsed")
		if err == nil {
			resp = b
		}
	}
	oresp := resp
	resp = normalize(resp)
	if expect == nil && resp != nil {
		return fmt.Errorf("expected '%v', got '%v'", expect, resp)
	}
	if vv, ok := resp.([]interface{}); ok {
		var ss []string
		for _, v := range vv {
			if v == nil {
				ss = append(ss, "nil")
			} else if s, ok := v.(string); ok {
				ss = append(ss, s)
			} else if b, ok := v.([]uint8); ok {
				if b == nil {
					ss = append(ss, "nil")
				} else {
					ss = append(ss, string(b))
				}
			} else {
				ss = append(ss, fmt.Sprintf("%v", v))
			}
		}
		resp = ss
	}
	if b, ok := resp.([]uint8); ok {
		if b == nil {
			resp = nil
		} else {
			resp = string([]byte(b))
		}
	}
	err = func() (err error) {
		defer func() {
			v := recover()
			if v != nil {
				err = fmt.Errorf("panic '%v'", v)
			}
		}()
		if fn, ok := expect.(func(v, org interface{}) (resp, expect interface{})); ok {
			resp, expect = fn(resp, oresp)
		}
		if fn, ok := expect.(func(v interface{}) (resp, expect interface{})); ok {
			resp, expect = fn(resp)
		}
		return nil
	}()
	if err != nil {
		return err
	}
	if fn, ok := expect.(func(string) bool); ok {
		if !fn(fmt.Sprintf("%v", resp)) {
			return fmt.Errorf("unexpected for response '%v'", resp)
		}
	} else if fn, ok := expect.(func(string) error); ok {
		err := fn(fmt.Sprintf("%v", resp))
		if err != nil {
			return fmt.Errorf("%s, for response '%v'", err.Error(), resp)
		}
	} else if fmt.Sprintf("%v", resp) != fmt.Sprintf("%v", expect) {
		return fmt.Errorf("expected '%v', got '%v'", expect, resp)
	}
	return nil
}
