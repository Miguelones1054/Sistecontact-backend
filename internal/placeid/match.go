package placeid

import "strings"

// Matches indica si un place_id almacenado corresponde al buscado.
func Matches(wanted, got string) bool {
	wanted = strings.TrimSpace(wanted)
	got = strings.TrimSpace(got)
	if wanted == "" || got == "" {
		return false
	}
	if wanted == got {
		return true
	}
	wn := Normalize(wanted)
	gn := Normalize(got)
	return wn != "" && wn == gn
}

// MatchesDoc indica si el ID de documento corresponde al place.
func MatchesDoc(placeID, docID string) bool {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return false
	}
	for _, c := range DocIDCandidates(placeID) {
		if c == docID {
			return true
		}
	}
	return false
}
