package googlemaps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sistecontact/api/internal/model"
)

const geocodeURL = "https://maps.googleapis.com/maps/api/geocode/json"

type geocodeResponse struct {
	Status  string         `json:"status"`
	Results []geocodeResult `json:"results"`
	Err     string         `json:"error_message"`
}

type geocodeResult struct {
	PlaceID   string          `json:"place_id"`
	Formatted string          `json:"formatted_address"`
	Types     []string        `json:"types"`
	Geometry  geocodeGeometry `json:"geometry"`
}

// Geocoding API usa lat/lng en sus campos (distinto de Places API New).
type geocodeGeometry struct {
	Location  geocodeLatLng `json:"location"`
	Viewport geocodeBounds  `json:"viewport"`
}

type geocodeLatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type geocodeBounds struct {
	Northeast geocodeLatLng `json:"northeast"`
	Southwest geocodeLatLng `json:"southwest"`
}

func (g geocodeLatLng) toModel() model.LatLng {
	return model.LatLng{Latitude: g.Lat, Longitude: g.Lng}
}

// Geocode resuelve un texto (nombre de zona) en una lista de zonas candidatas.
func (c *Client) Geocode(ctx context.Context, query string) ([]model.Zone, error) {
	q := url.Values{}
	q.Set("address", query)
	q.Set("key", c.APIKey)
	q.Set("language", c.Language)
	if c.Region != "" {
		q.Set("components", "country:"+c.Region)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geocodeURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gr geocodeResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("geocode decode: %w", err)
	}
	if gr.Status != "OK" && gr.Status != "ZERO_RESULTS" {
		return nil, fmt.Errorf("geocode: %s %s", gr.Status, gr.Err)
	}

	zones := make([]model.Zone, 0, len(gr.Results))
	for _, r := range gr.Results {
		name := shortName(r.Formatted, r.Types)
		zones = append(zones, model.Zone{
			PlaceID:   r.PlaceID,
			Name:      name,
			Formatted: r.Formatted,
			Center:    r.Geometry.Location.toModel(),
			Viewport: model.Viewport{
				Northeast: r.Geometry.Viewport.Northeast.toModel(),
				Southwest: r.Geometry.Viewport.Southwest.toModel(),
			},
			Types: r.Types,
		})
	}
	return zones, nil
}

// GeocodePlaceID resuelve un place_id concreto en una zona.
func (c *Client) GeocodePlaceID(ctx context.Context, placeID string) (*model.Zone, error) {
	q := url.Values{}
	q.Set("place_id", placeID)
	q.Set("key", c.APIKey)
	q.Set("language", c.Language)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geocodeURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gr geocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("geocode decode: %w", err)
	}
	if gr.Status != "OK" || len(gr.Results) == 0 {
		return nil, fmt.Errorf("geocode place_id: %s %s", gr.Status, gr.Err)
	}
	r := gr.Results[0]
	return &model.Zone{
		PlaceID:   r.PlaceID,
		Name:      shortName(r.Formatted, r.Types),
		Formatted: r.Formatted,
		Center:    r.Geometry.Location.toModel(),
		Viewport: model.Viewport{
			Northeast: r.Geometry.Viewport.Northeast.toModel(),
			Southwest: r.Geometry.Viewport.Southwest.toModel(),
		},
		Types: r.Types,
	}, nil
}

// shortName intenta devolver un nombre corto legible de la zona.
func shortName(formatted string, types []string) string {
	parts := strings.Split(formatted, ",")
	if len(parts) == 0 {
		return formatted
	}
	return strings.TrimSpace(parts[0])
}
