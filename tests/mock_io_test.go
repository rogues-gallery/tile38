package tests

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type IO struct {
	args  []any
	json  bool
	out   any
	sleep bool
	dur   time.Duration
	cfile string
	cln   int
}

func Do(args ...any) *IO {
	_, cfile, cln, _ := runtime.Caller(1)
	return &IO{args: args, cfile: cfile, cln: cln}
}
func (cmd *IO) JSON() *IO {
	cmd.json = true
	return cmd
}
func (cmd *IO) Str(s string) *IO {
	cmd.out = s
	return cmd
}
func (cmd *IO) Func(fn func(s string) error) *IO {
	cmd.out = func(s string) error {
		if cmd.json {
			if !gjson.Valid(s) {
				return errors.New("invalid json")
			}
		}
		return fn(s)
	}
	return cmd
}

func (cmd *IO) OK() *IO {
	return cmd.Func(func(s string) error {
		if cmd.json {
			if gjson.Get(s, "ok").Type != gjson.True {
				return errors.New("not ok")
			}
		} else if s != "OK" {
			return errors.New("not ok")
		}
		return nil
	})
}

func (cmd *IO) Err(msg string) *IO {
	return cmd.Func(func(s string) error {
		if cmd.json {
			if gjson.Get(s, "ok").Type != gjson.False {
				return errors.New("ok=true")
			}
			if gjson.Get(s, "err").String() != msg {
				return fmt.Errorf("expected '%s', got '%s'",
					msg, gjson.Get(s, "err").String())
			}
		} else {
			s = strings.TrimPrefix(s, "ERR ")
			if s != msg {
				return fmt.Errorf("expected '%s', got '%s'", msg, s)
			}
		}
		return nil
	})
}

func Sleep(duration time.Duration) *IO {
	return &IO{sleep: true, dur: duration}
}

func (cmd *IO) deepError(index int, err error) error {
	frag := "(?)"
	bdata, _ := os.ReadFile(cmd.cfile)
	data := string(bdata)
	ln := 1
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			ln++
			if ln == cmd.cln {
				data = data[i+1:]
				i = strings.IndexByte(data, '(')
				if i != -1 {
					j := strings.IndexByte(data[i:], ')')
					if j != -1 {
						frag = string(data[i : j+i+1])
					}
				}
				break
			}
		}
	}
	fsig := fmt.Sprintf("%s:%d", filepath.Base(cmd.cfile), cmd.cln)
	emsg := err.Error()
	if strings.HasPrefix(emsg, "expected ") &&
		strings.Contains(emsg, ", got ") {
		emsg = "" +
			"  EXPECTED: " + strings.Split(emsg, ", got ")[0][9:] + "\n" +
			"       GOT: " +
			strings.Split(emsg, ", got ")[1]
	} else {
		emsg = "" +
			"     ERROR: " + emsg
	}
	return fmt.Errorf("\n%s: entry[%d]\n   COMMAND: %s\n%s",
		fsig, index+1, frag, emsg)
}

func (mc *mockServer) doIOTest(index int, cmd *IO) error {
	if cmd.sleep {
		time.Sleep(cmd.dur)
		return nil
	}
	// switch json mode if desired
	if cmd.json {
		if !mc.ioJSON {
			if _, err := mc.Do("OUTPUT", "json"); err != nil {
				return err
			}
			mc.ioJSON = true
		}
	} else {
		if mc.ioJSON {
			if _, err := mc.Do("OUTPUT", "resp"); err != nil {
				return err
			}
			mc.ioJSON = false
		}
	}

	err := mc.DoExpect(cmd.out, cmd.args[0].(string), cmd.args[1:]...)
	if err != nil {
		return cmd.deepError(index, err)
	}
	return nil
}
