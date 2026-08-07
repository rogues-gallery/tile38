package server

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"os"
	"sync/atomic"
	"time"
)

func bsonID() string {
	b := make([]byte, 12)
	binary.BigEndian.PutUint32(b, uint32(time.Now().Unix()))
	copy(b[4:], bsonMachine)
	binary.BigEndian.PutUint32(b[8:], atomic.AddUint32(&bsonCounter, 1))
	binary.BigEndian.PutUint16(b[7:], bsonProcess)
	return hex.EncodeToString(b)
}

var (
	bsonProcess = uint16(os.Getpid())
	bsonMachine = func() []byte {
		host, _ := os.Hostname()
		b := make([]byte, 3)
		Must(rand.Read(b))
		host = Default(host, string(b))
		hw := md5.New()
		hw.Write([]byte(host))
		return hw.Sum(nil)[:3]
	}()
	bsonCounter = func() uint32 {
		b := make([]byte, 4)
		Must(rand.Read(b))
		return binary.BigEndian.Uint32(b)
	}()
)
