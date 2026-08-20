package placeid

import "strings"

// DocIDCandidates posibles IDs de documento Firestore para un place.
func DocIDCandidates(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	add(SanitizeDocID(id))
	add(strings.ReplaceAll(id, "/", "_"))
	add(strings.ReplaceAll(Normalize(id), "/", "_"))
	return out
}
