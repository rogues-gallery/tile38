package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/term"
)

var wmu sync.Mutex
var wr io.Writer

var zmu sync.Mutex
var zlogger *zap.SugaredLogger

var tty atomic.Bool
var ljson atomic.Bool
var llevel atomic.Int32

func init() {
	SetOutput(os.Stderr)
	SetLevel(1)
}

// Level is the log level
// 0: silent  - do not log
// 1: normal  - show everything except debug and warn
// 2: verbose - show everything except debug
// 3: very verbose - show everything
func SetLevel(level int) {
	if level < 0 {
		level = 0
	} else if level > 3 {
		level = 3
	}
	llevel.Store(int32(level))
}

// Level returns the log level
func Level() int {
	return int(llevel.Load())
}

func SetLogJSON(logJSON bool) {
	ljson.Store(logJSON)
}

func LogJSON() bool {
	return ljson.Load()
}

// SetOutput sets the output of the logger
func SetOutput(w io.Writer) {
	f, ok := w.(*os.File)
	tty.Store(ok && term.IsTerminal(int(f.Fd())))
	wmu.Lock()
	wr = w
	wmu.Unlock()
}

// Build a zap logger from default or custom config
func Build(c string) error {
	var zcfg zap.Config
	if c == "" {
		zcfg = zap.NewProductionConfig()

		// to be able to filter with Tile38 levels
		zcfg.Level.SetLevel(zap.DebugLevel)
		// disable caller because caller is always log.go
		zcfg.DisableCaller = true

	} else {
		err := json.Unmarshal([]byte(c), &zcfg)
		if err != nil {
			return err
		}
		// to be able to filter with Tile38 levels
		zcfg.Level.SetLevel(zap.DebugLevel)
		// disable caller because caller is always log.go
		zcfg.DisableCaller = true
	}
	core, err := zcfg.Build()
	if err != nil {
		return err
	}
	defer core.Sync()
	zmu.Lock()
	zlogger = core.Sugar()
	zmu.Unlock()
	return nil
}

// Set a zap logger
func Set(sl *zap.SugaredLogger) {
	zmu.Lock()
	zlogger = sl
	zmu.Unlock()
}

// Get a zap logger
func Get() *zap.SugaredLogger {
	zmu.Lock()
	sl := zlogger
	zmu.Unlock()
	return sl
}

// Output returns the output writer
func Output() io.Writer {
	wmu.Lock()
	defer wmu.Unlock()
	return wr
}

func log(level int, tag, color string, formatted bool, format string, args ...interface{}) {
	if llevel.Load() < int32(level) {
		return
	}
	var msg string
	if formatted {
		msg = fmt.Sprintf(format, args...)
	} else {
		msg = fmt.Sprint(args...)
	}
	if ljson.Load() {
		zmu.Lock()
		defer zmu.Unlock()
		switch tag {
		case "ERRO":
			zlogger.Error(msg)
		case "FATA":
			zlogger.Fatal(msg)
		case "WARN":
			zlogger.Warn(msg)
		case "DEBU":
			zlogger.Debug(msg)
		default:
			zlogger.Info(msg)
		}
		return
	}
	s := []byte(time.Now().Format("2006/01/02 15:04:05"))
	s = append(s, ' ')
	if tty.Load() {
		s = append(s, color...)
	}
	s = append(s, '[')
	s = append(s, tag...)
	s = append(s, ']')
	if tty.Load() {
		s = append(s, "\x1b[0m"...)
	}
	s = append(s, ' ')
	s = append(s, msg...)
	if s[len(s)-1] != '\n' {
		s = append(s, '\n')
	}
	wmu.Lock()
	wr.Write(s)
	wmu.Unlock()
}

var emptyFormat string

// Infof ...
func Infof(format string, args ...interface{}) {
	if llevel.Load() >= 1 {
		log(1, "INFO", "\x1b[36m", true, format, args...)
	}
}

// Info ...
func Info(args ...interface{}) {
	if llevel.Load() >= 1 {
		log(1, "INFO", "\x1b[36m", false, emptyFormat, args...)
	}
}

// HTTPf ...
func HTTPf(format string, args ...interface{}) {
	if llevel.Load() >= 1 {
		log(1, "HTTP", "\x1b[1m\x1b[30m", true, format, args...)
	}
}

// HTTP ...
func HTTP(args ...interface{}) {
	if llevel.Load() >= 1 {
		log(1, "HTTP", "\x1b[1m\x1b[30m", false, emptyFormat, args...)
	}
}

// Errorf ...
func Errorf(format string, args ...interface{}) {
	if llevel.Load() >= 1 {
		log(1, "ERRO", "\x1b[1m\x1b[31m", true, format, args...)
	}
}

// Error ..
func Error(args ...interface{}) {
	if llevel.Load() >= 1 {
		log(1, "ERRO", "\x1b[1m\x1b[31m", false, emptyFormat, args...)
	}
}

// Warnf ...
func Warnf(format string, args ...interface{}) {
	if llevel.Load() >= 1 {
		log(2, "WARN", "\x1b[33m", true, format, args...)
	}
}

// Warn ...
func Warn(args ...interface{}) {
	if llevel.Load() >= 1 {
		log(2, "WARN", "\x1b[33m", false, emptyFormat, args...)
	}
}

// Debugf ...
func Debugf(format string, args ...interface{}) {
	if llevel.Load() >= 3 {
		log(3, "DEBU", "\x1b[35m", true, format, args...)
	}
}

// Debug ...
func Debug(args ...interface{}) {
	if llevel.Load() >= 3 {
		log(3, "DEBU", "\x1b[35m", false, emptyFormat, args...)
	}
}

// Printf ...
func Printf(format string, args ...interface{}) {
	Infof(format, args...)
}

// Print ...
func Print(args ...interface{}) {
	Info(args...)
}

// Fatalf ...
func Fatalf(format string, args ...interface{}) {
	log(1, "FATA", "\x1b[31m", true, format, args...)
	os.Exit(1)
}

// Fatal ...
func Fatal(args ...interface{}) {
	log(1, "FATA", "\x1b[31m", false, emptyFormat, args...)
	os.Exit(1)
}
