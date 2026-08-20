package placeid

import "strings"

// Normalize quita el prefijo "places/" de Places API (New).
func Normalize(id string) string {
	id = strings.TrimSpace(id)
	return strings.TrimPrefix(id, "places/")
}

// Variants devuelve IDs equivalentes para consultas (con y sin prefijo).
func Variants(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	norm := Normalize(id)
	out := []string{norm}
	if id != norm {
		out = append(out, id)
	}
	prefixed := "places/" + norm
	if prefixed != id && prefixed != norm {
		out = append(out, prefixed)
	}
	return out
}

// SanitizeDocID hace el id seguro como documento Firestore.
func SanitizeDocID(id string) string {
	return strings.ReplaceAll(Normalize(id), "/", "_")
}
