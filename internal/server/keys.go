package server

import (
	"encoding/json"
	"time"

	"github.com/tidwall/resp"
	"github.com/tidwall/tile38/internal/collection"
	"github.com/tidwall/tile38/internal/glob"
)

// KEYS pattern
func (s *Server) cmdKEYS(msg *Message) (resp.Value, error) {
	var start = time.Now()

	// >> Args

	args := msg.Args
	if len(args) != 2 {
		return retrerr(errInvalidNumberOfArguments)
	}
	pattern := args[1]

	// >> Operation

	keys := []string{}
	g := glob.Parse(pattern, false)
	everything := g.Limits[0] == "" && g.Limits[1] == ""
	if everything {
		s.cols.Scan(
			func(key string, _ *collection.Collection) bool {
				match, _ := glob.Match(pattern, key)
				if match {
					keys = append(keys, key)
				}
				return true
			},
		)
	} else {
		s.cols.Ascend(g.Limits[0],
			func(key string, _ *collection.Collection) bool {
				if key > g.Limits[1] {
					return false
				}
				match, _ := glob.Match(pattern, key)
				if match {
					keys = append(keys, key)
				}
				return true
			},
		)
	}

	// >> Response

	if msg.OutputType == JSON {
		data, _ := json.Marshal(keys)
		return resp.StringValue(`{"ok":true,"keys":` + string(data) +
			`,"elapsed":"` + time.Since(start).String() + `"}`), nil
	}

	var vals []resp.Value
	for _, key := range keys {
		vals = append(vals, resp.StringValue(key))
	}
	return resp.ArrayValue(vals), nil
}
