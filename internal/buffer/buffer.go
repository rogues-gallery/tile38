package buffer

import (
	"errors"
	"math"

	"github.com/tidwall/geojson"
	"github.com/tidwall/geojson/geo"
	"github.com/tidwall/geojson/geometry"
	"github.com/tidwall/gjson"
)

// TODO: detect of pole and antimeridian crossing and generate
// valid multigeometries

const bufferSteps = 15

// Simple performs a very simple buffer operation on a geojson object.
func Simple(g geojson.Object, meters float64) (geojson.Object, error) {
	if meters <= 0 {
		return g, nil
	}
	if math.IsInf(meters, 0) || math.IsNaN(meters) {
		return g, errors.New("invalid meters")
	}
	switch g := g.(type) {
	case *geojson.Point:
		return bufferSimplePoint(g.Base(), meters), nil
	case *geojson.SimplePoint:
		return bufferSimplePoint(g.Base(), meters), nil
	case *geojson.MultiPoint:
		return bufferSimpleGeometries(g.Base(), meters)
	case *geojson.LineString:
		return bufferSimpleLineString(g, meters)
	case *geojson.MultiLineString:
		return bufferSimpleGeometries(g.Base(), meters)
	case *geojson.Polygon:
		return bufferSimplePolygon(g, meters)
	case *geojson.MultiPolygon:
		return bufferSimpleGeometries(g.Base(), meters)
	case *geojson.FeatureCollection:
		return bufferSimpleFeatures(g.Base(), meters)
	case *geojson.Feature:
		bg, err := Simple(g.Base(), meters)
		if err != nil {
			return nil, err
		}
		return geojson.NewFeature(bg, g.Members()), nil
	case *geojson.Circle:
		return Simple(g.Polygon(), meters)
	case nil:
		return nil, errors.New("cannot buffer nil object")
	default:
		typ := gjson.Get(g.JSON(), "type").String()
		return nil, errors.New("cannot buffer " + typ + " type")
	}
}

func bufferSimplePoint(p geometry.Point, meters float64) *geojson.Polygon {
	meters = geo.NormalizeDistance(meters)
	points := make([]geometry.Point, 0, bufferSteps+1)

	// calc the four corners
	maxY, _ := geo.DestinationPoint(p.Y, p.X, meters, 0)
	_, maxX := geo.DestinationPoint(p.Y, p.X, meters, 90)
	minY, _ := geo.DestinationPoint(p.Y, p.X, meters, 180)
	_, minX := geo.DestinationPoint(p.Y, p.X, meters, 270)

	// use the half width of the lat and lon
	lons := (maxX - minX) / 2
	lats := (maxY - minY) / 2

	// generate the circle polygon
	for th := 0.0; th <= 360.0; th += 360.0 / float64(bufferSteps) {
		radians := (math.Pi / 180) * th
		x := p.X + lons*math.Cos(radians)
		y := p.Y + lats*math.Sin(radians)
		points = append(points, geometry.Point{X: x, Y: y})
	}
	// add last connecting point, make a total of steps+1
	points = append(points, points[0])
	poly := geojson.NewPolygon(
		geometry.NewPoly(points, nil, &geometry.IndexOptions{
			Kind: geometry.None,
		}),
	)
	return poly
}

func bufferSimpleGeometries(objs []geojson.Object, meters float64,
) (*geojson.GeometryCollection, error) {
	geoms := make([]geojson.Object, len(objs))
	for i := 0; i < len(objs); i++ {
		g, err := Simple(objs[i], meters)
		if err != nil {
			return nil, err
		}
		geoms[i] = g
	}
	return geojson.NewGeometryCollection(geoms), nil
}

func bufferSimpleFeatures(objs []geojson.Object, meters float64,
) (*geojson.FeatureCollection, error) {
	geoms := make([]geojson.Object, len(objs))
	for i := 0; i < len(objs); i++ {
		g, err := Simple(objs[i], meters)
		if err != nil {
			return nil, err
		}
		geoms[i] = g
	}
	return geojson.NewFeatureCollection(geoms), nil
}

// appendBufferSimpleSeries buffers a series and appends its parts to dst
func appendBufferSimpleSeries(dst []geojson.Object, s geometry.Series, meters float64) []geojson.Object {
	nsegs := s.NumSegments()
	for i := 0; i < nsegs; i++ {
		dst = appendSimpleBufferSegment(dst, s.SegmentAt(i), meters, i == 0)
	}
	return dst
}

// appendSimpleBufferSegment buffers a segment and appends its parts to dst
func appendSimpleBufferSegment(dst []geojson.Object, seg geometry.Segment,
	meters float64, first bool,
) []geojson.Object {
	if first {
		// endcap A
		dst = append(dst, bufferSimplePoint(seg.A, meters))
	}
	// line polygon
	bear1 := geo.BearingTo(seg.A.Y, seg.A.X, seg.B.Y, seg.B.X)
	lat1, lon1 := geo.DestinationPoint(seg.A.Y, seg.A.X, meters, bear1-90)
	lat2, lon2 := geo.DestinationPoint(seg.A.Y, seg.A.X, meters, bear1+90)
	bear2 := geo.BearingTo(seg.B.Y, seg.B.X, seg.A.Y, seg.A.X)
	lat3, lon3 := geo.DestinationPoint(seg.B.Y, seg.B.X, meters, bear2-90)
	lat4, lon4 := geo.DestinationPoint(seg.B.Y, seg.B.X, meters, bear2+90)
	dst = append(dst, geojson.NewPolygon(
		geometry.NewPoly([]geometry.Point{
			{X: lon1, Y: lat1},
			{X: lon2, Y: lat2},
			{X: lon3, Y: lat3},
			{X: lon4, Y: lat4},
			{X: lon1, Y: lat1},
		}, nil, nil)))
	// endcap B
	dst = append(dst, bufferSimplePoint(seg.B, meters))
	return dst
}

func bufferSimplePolygon(p *geojson.Polygon, meters float64,
) (*geojson.GeometryCollection, error) {
	var geoms []geojson.Object
	b := p.Base()
	geoms = appendBufferSimpleSeries(geoms, b.Exterior, meters)
	for _, hole := range b.Holes {
		geoms = appendBufferSimpleSeries(geoms, hole, meters)
	}
	geoms = append(geoms, p)
	return geojson.NewGeometryCollection(geoms), nil
}

func bufferSimpleLineString(l *geojson.LineString, meters float64,
) (*geojson.GeometryCollection, error) {
	geoms := appendBufferSimpleSeries(nil, l.Base(), meters)
	return geojson.NewGeometryCollection(geoms), nil
}
