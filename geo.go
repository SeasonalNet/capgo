package cap

import (
	"fmt"
	"strconv"
	"strings"
)

// Point is a WGS 84 latitude/longitude pair.
type Point struct {
	Latitude  float64
	Longitude float64
}

// ParsePoint parses a CAP WGS 84 latitude,longitude pair.
func ParsePoint(value string) (Point, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return Point{}, fmt.Errorf("coordinate must be latitude,longitude: %q", value)
	}
	latitude, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || latitude < -90 || latitude > 90 {
		return Point{}, fmt.Errorf("latitude must be between -90 and 90: %q", parts[0])
	}
	longitude, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || longitude < -180 || longitude > 180 {
		return Point{}, fmt.Errorf("longitude must be between -180 and 180: %q", parts[1])
	}
	return Point{Latitude: latitude, Longitude: longitude}, nil
}

// ParsePolygon parses and checks a CAP polygon. A polygon must contain at
// least four points and its first and last points must be identical.
func ParsePolygon(value string) ([]Point, error) {
	fields := strings.Fields(value)
	if len(fields) < 4 {
		return nil, fmt.Errorf("polygon requires at least four coordinate pairs")
	}
	points := make([]Point, len(fields))
	for index, field := range fields {
		point, err := ParsePoint(field)
		if err != nil {
			return nil, fmt.Errorf("polygon point %d: %w", index+1, err)
		}
		points[index] = point
	}
	if points[0] != points[len(points)-1] {
		return nil, fmt.Errorf("polygon must end with its first coordinate pair")
	}
	return points, nil
}

// ParseCircle parses a CAP circle into its center and radius in kilometres.
func ParseCircle(value string) (Point, float64, error) {
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return Point{}, 0, fmt.Errorf("circle must be latitude,longitude radius-km")
	}
	center, err := ParsePoint(fields[0])
	if err != nil {
		return Point{}, 0, err
	}
	radius, err := strconv.ParseFloat(fields[1], 64)
	if err != nil || radius <= 0 {
		return Point{}, 0, fmt.Errorf("circle radius must be a positive number of kilometres: %q", fields[1])
	}
	return center, radius, nil
}
