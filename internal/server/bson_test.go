package server

import "testing"

func TestBSON(t *testing.T) {
	id := bsonID()
	if len(id) != 24 {
		t.Fail()
	}
}
