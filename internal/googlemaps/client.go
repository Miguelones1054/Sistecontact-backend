package googlemaps

import (
	"net/http"
	"time"
)

// Client envuelve un http.Client optimizado para llamadas a Google.
type Client struct {
	HTTP     *http.Client
	APIKey   string
	Language string
	Region   string
}

func New(apiKey, language, region string, timeout time.Duration) *Client {
	tr := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	return &Client{
		HTTP:     &http.Client{Transport: tr, Timeout: timeout},
		APIKey:   apiKey,
		Language: language,
		Region:   region,
	}
}
