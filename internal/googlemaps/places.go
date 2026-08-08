package googlemaps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sistecontact/api/internal/model"
)

const placesTextURL = "https://places.googleapis.com/v1/places:searchText"

// Field mask: solo los campos que necesitamos (reduce coste y latencia).
// nextPageToken debe pedirse explícitamente para que Google lo devuelva.
const placesFieldMask = "places.id,places.displayName,places.formattedAddress,places.location,places.rating,places.userRatingCount,places.googleMapsUri,places.types,places.currentOpeningHours.openNow,places.nationalPhoneNumber,places.internationalPhoneNumber,nextPageToken"

type placesTextRequest struct {
	TextQuery           string                  `json:"textQuery"`
	LanguageCode        string                  `json:"languageCode"`
	RegionCode          string                  `json:"regionCode,omitempty"`
	LocationRestriction *placesLocationRestrict `json:"locationRestriction,omitempty"`
	PageToken           string                  `json:"pageToken,omitempty"`
}

type placesLocationRestrict struct {
	Rectangle placesRectangle `json:"rectangle"`
}

// placesRectangle define un bounding box con la esquina inferior (low) y
// la superior (high). LatLng usa latitude/longitude (formato Places API New).
type placesRectangle struct {
	Low  model.LatLng `json:"low"`
	High model.LatLng `json:"high"`
}

type placesTextResponse struct {
	Places       []placesPlace `json:"places"`
	NextPageToken string        `json:"nextPageToken"`
}

type placesPlace struct {
	ID                       string       `json:"id"`
	DisplayName              placesText   `json:"displayName"`
	FormattedAddress         string       `json:"formattedAddress"`
	Location                 model.LatLng `json:"location"`
	Rating                   float64      `json:"rating"`
	UserRatingCount          int          `json:"userRatingCount"`
	GoogleMapsURI            string       `json:"googleMapsUri"`
	NationalPhoneNumber      string       `json:"nationalPhoneNumber"`
	InternationalPhoneNumber string       `json:"internationalPhoneNumber"`
	Types                    []string     `json:"types"`
	CurrentOpeningHours      *placesHours `json:"currentOpeningHours"`
}

type placesText struct {
	Text string `json:"text"`
}

type placesHours struct {
	OpenNow bool `json:"openNow"`
}

// SearchByText busca comercios que coincidan con textQuery dentro de un
// rectángulo (viewport de la zona). Usa la Places API (New).
// pageToken permite traer la siguiente página de resultados (paginación).
func (c *Client) SearchByText(ctx context.Context, textQuery string, viewport model.Viewport, pageToken string) ([]model.Business, string, error) {
	reqBody := placesTextRequest{
		TextQuery:    textQuery,
		LanguageCode: c.Language,
		RegionCode:   c.Region,
		PageToken:    pageToken,
	}
	// locationRestriction debe enviarse en TODAS las páginas (Google exige
	// que los parámetros coincidan con la primera llamada al paginar).
	reqBody.LocationRestriction = &placesLocationRestrict{
		Rectangle: placesRectangle{
			Low:  viewport.Southwest,
			High: viewport.Northeast,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, placesTextURL, bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.APIKey)
	req.Header.Set("X-Goog-FieldMask", placesFieldMask)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("places search: status %d: %s", resp.StatusCode, string(body))
	}

	var pr placesTextResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, "", fmt.Errorf("places decode: %w", err)
	}

	businesses := make([]model.Business, 0, len(pr.Places))
	for _, p := range pr.Places {
		// Preferimos el nacional; si no, el internacional.
		phone := p.NationalPhoneNumber
		if phone == "" {
			phone = p.InternationalPhoneNumber
		}

		b := model.Business{
			PlaceID:         p.ID,
			Name:            p.DisplayName.Text,
			Address:         p.FormattedAddress,
			Location:        p.Location,
			Rating:          p.Rating,
			UserRatingCount: p.UserRatingCount,
			GoogleMapsURI:   p.GoogleMapsURI,
			Phone:           phone,
			Types:           p.Types,
		}
		if p.CurrentOpeningHours != nil {
			open := p.CurrentOpeningHours.OpenNow
			b.OpenNow = &open
		}
		businesses = append(businesses, b)
	}
	return businesses, pr.NextPageToken, nil
}
