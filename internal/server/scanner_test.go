package server

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/tidwall/geojson"
	"github.com/tidwall/geojson/geometry"
	"github.com/tidwall/tile38/internal/field"
	"github.com/tidwall/tile38/internal/object"
)

type testPointItem struct {
	object geojson.Object
	fields field.List
}

func PO(x, y float64) *geojson.Point {
	return geojson.NewPoint(geometry.Point{X: x, Y: y})
}

func BenchmarkFieldMatch(t *testing.B) {
	rand.Seed(time.Now().UnixNano())
	items := make([]testPointItem, t.N)
	for i := 0; i < t.N; i++ {
		var fields field.List
		fields = fields.Set(field.Make("foo", fmt.Sprintf("%f", rand.Float64()*9+1)))
		fields = fields.Set(field.Make("bar", fmt.Sprintf("%f", math.Round(rand.Float64()*30)+1)))
		items[i] = testPointItem{
			PO(rand.Float64()*360-180, rand.Float64()*180-90),
			fields,
		}
	}
	sw := &scanWriter{
		wheres: []whereT{
			{false, "foo", false, field.ValueOf("1"), false, field.ValueOf("3")},
			{false, "bar", false, field.ValueOf("10"), false, field.ValueOf("30")},
		},
		whereins: []whereinT{
			{"foo", []field.Value{field.ValueOf("1"), field.ValueOf("2")}},
			{"bar", []field.Value{field.ValueOf("11"), field.ValueOf("25")}},
		},
	}
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		// one call is super fast, measurements are not reliable, let's do 100
		for ix := 0; ix < 100; ix++ {
			sw.fieldMatch(object.New("", items[i].object, 0, items[i].fields))
		}
	}
}
