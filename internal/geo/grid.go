package geo

import "github.com/sistecontact/api/internal/model"

// Split2x2 divide un viewport en 4 sub-rectángulos (cuadrícula 2×2).
func Split2x2(vp model.Viewport) []model.Viewport {
	midLat := (vp.Southwest.Latitude + vp.Northeast.Latitude) / 2
	midLng := (vp.Southwest.Longitude + vp.Northeast.Longitude) / 2

	return []model.Viewport{
		{ // SW
			Southwest: vp.Southwest,
			Northeast: model.LatLng{Latitude: midLat, Longitude: midLng},
		},
		{ // SE
			Southwest: model.LatLng{Latitude: vp.Southwest.Latitude, Longitude: midLng},
			Northeast: model.LatLng{Latitude: midLat, Longitude: vp.Northeast.Longitude},
		},
		{ // NW
			Southwest: model.LatLng{Latitude: midLat, Longitude: vp.Southwest.Longitude},
			Northeast: model.LatLng{Latitude: vp.Northeast.Latitude, Longitude: midLng},
		},
		{ // NE
			Southwest: model.LatLng{Latitude: midLat, Longitude: midLng},
			Northeast: vp.Northeast,
		},
	}
}

// Contains indica si el punto está dentro del viewport (inclusive).
func Contains(vp model.Viewport, p model.LatLng) bool {
	return p.Latitude >= vp.Southwest.Latitude &&
		p.Latitude <= vp.Northeast.Latitude &&
		p.Longitude >= vp.Southwest.Longitude &&
		p.Longitude <= vp.Northeast.Longitude
}
