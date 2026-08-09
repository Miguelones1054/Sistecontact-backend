package search

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/sistecontact/api/internal/cache"
	"github.com/sistecontact/api/internal/geo"
	"github.com/sistecontact/api/internal/googlemaps"
	"github.com/sistecontact/api/internal/model"
)

// googlePageLimit es el máximo de resultados por consulta Text Search (New).
const googlePageLimit = 60

type Service struct {
	gmaps      *googlemaps.Client
	zones      *cache.TTL[string, []model.Zone]
	search     *cache.TTL[string, model.SearchResponse]
	maxPages   int
	maxDepth   int
	workers    int
	logger     *slog.Logger
}

func New(
	gmaps *googlemaps.Client,
	zoneTTL, searchTTL, cleanup time.Duration,
	maxPages, maxDepth, workers int,
	logger *slog.Logger,
) *Service {
	if maxPages < 1 {
		maxPages = 1
	}
	if maxPages > 3 {
		maxPages = 3
	}
	if maxDepth < 0 {
		maxDepth = 0
	}
	if maxDepth > 4 {
		maxDepth = 4 // evita explosión de llamadas a Google
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 8 {
		workers = 8
	}
	return &Service{
		gmaps:    gmaps,
		zones:    cache.New[string, []model.Zone](zoneTTL, cleanup),
		search:   cache.New[string, model.SearchResponse](searchTTL, cleanup),
		maxPages: maxPages,
		maxDepth: maxDepth,
		workers:  workers,
		logger:   logger,
	}
}

// FindZones devuelve zonas candidatas para un texto de búsqueda.
func (s *Service) FindZones(ctx context.Context, query string) ([]model.Zone, error) {
	if query == "" {
		return nil, fmt.Errorf("query vacío")
	}
	if cached, ok := s.zones.Get(query); ok {
		return cached, nil
	}

	zones, err := s.gmaps.Geocode(ctx, query)
	if err != nil {
		return nil, err
	}
	s.zones.Set(query, zones)
	return zones, nil
}

// Search busca comercios de un tipo dentro de una zona (nombre o place_id).
// Si radiusKm > 0, acota al círculo centro+radio; si no, usa el viewport
// geocodificado de la zona. Si una consulta alcanza el límite de Google (60),
// subdivide el área en cuadrículas y combina resultados sin duplicados.
func (s *Service) Search(ctx context.Context, businessType, zoneRef string, radiusKm float64) (model.SearchResponse, error) {
	if businessType == "" || zoneRef == "" {
		return model.SearchResponse{}, fmt.Errorf("type y zone son obligatorios")
	}

	cacheKey := fmt.Sprintf("%s|%s|%.2f", businessType, zoneRef, radiusKm)
	if cached, ok := s.search.Get(cacheKey); ok {
		return cached, nil
	}

	zone, err := s.resolveZone(ctx, zoneRef)
	if err != nil {
		return model.SearchResponse{}, err
	}

	searchVP := zone.Viewport
	if radiusKm > 0 {
		searchVP = geo.ViewportFromRadius(zone.Center, radiusKm)
	}

	businesses, err := s.searchViewport(ctx, businessType, searchVP, 0)
	if err != nil {
		return model.SearchResponse{}, err
	}

	filtered := make([]model.Business, 0, len(businesses))
	for _, b := range businesses {
		if radiusKm > 0 {
			if geo.WithinRadius(zone.Center, b.Location, radiusKm) {
				filtered = append(filtered, b)
			}
			continue
		}
		if geo.Contains(zone.Viewport, b.Location) {
			filtered = append(filtered, b)
		}
	}

	resp := model.SearchResponse{
		Zone:       *zone,
		Query:      businessType,
		Count:      len(filtered),
		RadiusKm:   radiusKm,
		Businesses: filtered,
	}
	s.search.Set(cacheKey, resp)
	s.logger.Info("búsqueda completada",
		"type", businessType,
		"zone", zone.Name,
		"radius_km", radiusKm,
		"count", len(filtered),
	)
	return resp, nil
}

// searchViewport busca en un viewport; si alcanza el límite de Google,
// subdivide en 2×2 y combina resultados (hasta maxDepth).
func (s *Service) searchViewport(ctx context.Context, query string, viewport model.Viewport, depth int) ([]model.Business, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pageResults, err := s.fetchAllPages(ctx, query, viewport, s.maxPages)
	if err != nil {
		return nil, err
	}

	// Menos del límite → esta área está completa.
	if len(pageResults) < googlePageLimit || depth >= s.maxDepth {
		return pageResults, nil
	}

	s.logger.Info("límite de Google alcanzado, subdividiendo zona",
		"depth", depth,
		"results", len(pageResults),
	)

	cells := geo.Split2x2(viewport)
	merged := s.mergeConcurrent(ctx, query, cells, depth+1)

	// Incluye lo ya obtenido + lo de las celdas (dedupe).
	return dedupe(append(pageResults, merged...)), nil
}

// mergeConcurrent busca en varias celdas en paralelo (con límite de workers).
func (s *Service) mergeConcurrent(ctx context.Context, query string, cells []model.Viewport, depth int) []model.Business {
	type result struct {
		items []model.Business
		err   error
	}

	jobs := make(chan model.Viewport)
	out := make(chan result, len(cells))

	var wg sync.WaitGroup
	workers := s.workers
	if workers > len(cells) {
		workers = len(cells)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cell := range jobs {
				items, err := s.searchViewport(ctx, query, cell, depth)
				out <- result{items: items, err: err}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, cell := range cells {
			select {
			case <-ctx.Done():
				return
			case jobs <- cell:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	var all []model.Business
	for r := range out {
		if r.err != nil {
			s.logger.Warn("celda falló", "err", r.err)
			continue
		}
		all = append(all, r.items...)
	}
	return all
}

// fetchAllPages recorre las páginas de Places API combinando resultados.
// Google requiere un pequeño retardo entre páginas para que el token sea válido.
func (s *Service) fetchAllPages(ctx context.Context, query string, viewport model.Viewport, maxPages int) ([]model.Business, error) {
	var all []model.Business
	pageToken := ""

	for page := 0; page < maxPages; page++ {
		if page > 0 {
			select {
			case <-ctx.Done():
				return all, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}

		businesses, nextToken, err := s.gmaps.SearchByText(ctx, query, viewport, pageToken)
		if err != nil {
			return all, err
		}
		all = append(all, businesses...)
		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}
	return all, nil
}

func dedupe(items []model.Business) []model.Business {
	seen := make(map[string]struct{}, len(items))
	out := make([]model.Business, 0, len(items))
	for _, b := range items {
		id := b.PlaceID
		if id == "" {
			id = b.Name + "|" + b.Address
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, b)
	}
	return out
}

// resolveZone resuelve una referencia de zona (place_id o nombre) en una Zone.
func (s *Service) resolveZone(ctx context.Context, ref string) (*model.Zone, error) {
	if isPlaceID(ref) {
		if z, err := s.gmaps.GeocodePlaceID(ctx, ref); err == nil {
			return z, nil
		}
	}

	zones, err := s.gmaps.Geocode(ctx, ref)
	if err != nil {
		return nil, err
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("zona no encontrada: %s", ref)
	}
	return &zones[0], nil
}

func isPlaceID(ref string) bool {
	return strings.HasPrefix(ref, "ChIJ") ||
		strings.HasPrefix(ref, "EiJ") ||
		strings.HasPrefix(ref, "GhIJ") ||
		strings.HasPrefix(ref, "GhoS")
}
