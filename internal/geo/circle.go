package geo

import (
	"math"

	"github.com/sistecontact/api/internal/model"
)

const earthRadiusKm = 6371.0

// DistanceKm calcula la distancia haversine entre dos puntos en kilómetros.
func DistanceKm(a, b model.LatLng) float64 {
	lat1 := a.Latitude * math.Pi / 180
	lat2 := b.Latitude * math.Pi / 180
	dLat := (b.Latitude - a.Latitude) * math.Pi / 180
	dLng := (b.Longitude - a.Longitude) * math.Pi / 180

	sinDLat := math.Sin(dLat / 2)
	sinDLng := math.Sin(dLng / 2)
	h := sinDLat*sinDLat + math.Cos(lat1)*math.Cos(lat2)*sinDLng*sinDLng
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(h)))
}

// WithinRadius indica si el punto está a lo sumo radiusKm del centro.
func WithinRadius(center, point model.LatLng, radiusKm float64) bool {
	if radiusKm <= 0 {
		return false
	}
	return DistanceKm(center, point) <= radiusKm
}

// ViewportFromRadius construye el bounding box que contiene el círculo
// centro + radio (para restriction rectangular de Places API).
func ViewportFromRadius(center model.LatLng, radiusKm float64) model.Viewport {
	if radiusKm <= 0 {
		radiusKm = 1
	}
	// Δlat ≈ km / 111.32 ; Δlng ≈ km / (111.32 · cos(lat))
	dLat := radiusKm / 111.32
	cosLat := math.Cos(center.Latitude * math.Pi / 180)
	if cosLat < 0.01 {
		cosLat = 0.01
	}
	dLng := radiusKm / (111.32 * cosLat)

	return model.Viewport{
		Southwest: model.LatLng{
			Latitude:  center.Latitude - dLat,
			Longitude: center.Longitude - dLng,
		},
		Northeast: model.LatLng{
			Latitude:  center.Latitude + dLat,
			Longitude: center.Longitude + dLng,
		},
	}
}
