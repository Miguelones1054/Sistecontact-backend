package model

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Viewport struct {
	Northeast LatLng `json:"northeast"`
	Southwest LatLng `json:"southwest"`
}

type Zone struct {
	PlaceID      string   `json:"place_id,omitempty"`
	Name         string   `json:"name"`
	Formatted    string   `json:"formatted_address"`
	Center       LatLng   `json:"center"`
	Viewport     Viewport `json:"viewport"`
	Types        []string `json:"types,omitempty"`
}

type Business struct {
	PlaceID         string   `json:"place_id"`
	Name            string   `json:"name"`
	Address         string   `json:"address"`
	Phone           string   `json:"phone,omitempty"`
	Location        LatLng   `json:"location"`
	Rating          float64  `json:"rating,omitempty"`
	UserRatingCount int      `json:"user_rating_count,omitempty"`
	GoogleMapsURI   string   `json:"google_maps_uri,omitempty"`
	Types           []string `json:"types,omitempty"`
	OpenNow         *bool    `json:"open_now,omitempty"`
}

type SearchRequest struct {
	Type     string  `json:"type"`
	Zone     string  `json:"zone"`
	RadiusKm float64 `json:"radius_km,omitempty"`
}

type SearchResponse struct {
	Zone       Zone       `json:"zone"`
	Query      string     `json:"query"`
	Count      int        `json:"count"`
	RadiusKm   float64    `json:"radius_km,omitempty"`
	Businesses []Business `json:"businesses"`
}
