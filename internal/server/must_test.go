package server

import (
	"errors"
	"testing"
)

func TestMust(t *testing.T) {
	if Must(1, nil) != 1 {
		t.Fail()
	}
	func() {
		var ended bool
		defer func() {
			if ended {
				t.Fail()
			}
			err, ok := recover().(error)
			if !ok {
				t.Fail()
			}
			if err.Error() != "ok" {
				t.Fail()
			}
		}()
		Must(1, errors.New("ok"))
		ended = true
	}()
}

func TestDefault(t *testing.T) {
	if Default("", "2") != "2" {
		t.Fail()
	}
	if Default("1", "2") != "1" {
		t.Fail()
	}
}
