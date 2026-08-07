//go:build exclude

package object

import (
	"github.com/tidwall/geojson"
	"github.com/tidwall/geojson/geometry"
	"github.com/tidwall/tile38/internal/field"
)

type Object struct {
	id      string
	geo     geojson.Object
	expires int64 // unix nano expiration
	fields  field.List
}

func (o *Object) ID() string {
	if o == nil {
		return ""
	}
	return o.id
}

func (o *Object) Fields() field.List {
	if o == nil {
		return field.List{}
	}
	return o.fields
}

func (o *Object) Expires() int64 {
	if o == nil {
		return 0
	}
	return o.expires
}

func (o *Object) Rect() geometry.Rect {
	if o == nil || o.geo == nil {
		return geometry.Rect{}
	}
	return o.geo.Rect()
}

func (o *Object) Geo() geojson.Object {
	if o == nil || o.geo == nil {
		return nil
	}
	return o.geo
}

func (o *Object) String() string {
	if o == nil || o.geo == nil {
		return ""
	}
	return o.geo.String()
}

func (o *Object) IsSpatial() bool {
	_, ok := o.geo.(geojson.Spatial)
	return ok
}

func (o *Object) Weight() int {
	if o == nil {
		return 0
	}
	var weight int
	weight += len(o.ID())
	if o.IsSpatial() {
		weight += o.Geo().NumPoints() * 16
	} else {
		weight += len(o.Geo().String())
	}
	weight += o.Fields().Weight()
	return weight
}

func New(id string, geo geojson.Object, expires int64, fields field.List,
) *Object {
	return &Object{
		id:      id,
		geo:     geo,
		expires: expires,
		fields:  fields,
	}
}
