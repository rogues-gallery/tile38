package server

import (
	"strings"
	"time"

	"github.com/tidwall/resp"
)

// OUTPUT [resp|json]
func (s *Server) cmdOUTPUT(msg *Message) (resp.Value, error) {
	start := time.Now()

	args := msg.Args
	switch len(args) {
	case 1:
		if msg.OutputType == JSON {
			return resp.StringValue(`{"ok":true,"output":"json","elapsed":` +
				time.Since(start).String() + `}`), nil
		}
		return resp.StringValue("resp"), nil
	case 2:
		// Setting the original message output type will be picked up by the
		// server prior to the next command being executed.
		switch strings.ToLower(args[1]) {
		default:
			return retrerr(errInvalidArgument(args[1]))
		case "json":
			msg.OutputType = JSON
		case "resp":
			msg.OutputType = RESP
		}
		return OKMessage(msg, start), nil
	default:
		return retrerr(errInvalidNumberOfArguments)
	}
}
