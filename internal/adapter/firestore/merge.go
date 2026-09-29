package firestore

// mergePreservingUnknown overlays incoming modeled fields onto the stored document.
// Keys the struct does not declare stay in place. Mapping columns absent from incoming
// are removed; unknown fields on a column that remains are kept.
func mergePreservingUnknown(existing, incoming map[string]any) map[string]any {
	return mergeObject(existing, incoming, modeledDocumentKeys(), false)
}

func modeledDocumentKeys() map[string]struct{} {
	return map[string]struct{}{
		"publisherName": {},
		"patterns":      {},
		"destination":   {},
		"bqParams":      {},
		"mappings":      {},
		"organization":  {},
		"notification":  {},
		"mode":          {},
	}
}

func mergeObject(existing, incoming map[string]any, modeled map[string]struct{}, columns bool) map[string]any {
	result := map[string]any{}

	if !columns {
		for key, value := range existing {
			result[key] = cloneValue(value)
		}
	}

	for key, value := range incoming {
		if key == "mappings" && !columns {
			result[key] = mergeObject(asMap(existing[key]), asMap(value), nil, true)

			continue
		}

		child, ok := value.(map[string]any)
		if ok {
			result[key] = mergeObject(asMap(existing[key]), child, nil, false)

			continue
		}

		result[key] = value
	}

	if columns {
		return result
	}

	for key := range modeled {
		if _, ok := incoming[key]; !ok {
			delete(result, key)
		}
	}

	return result
}

func asMap(value any) map[string]any {
	child, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}

	return child
}

func cloneValue(value any) any {
	child, ok := value.(map[string]any)
	if !ok {
		return value
	}

	cloned := make(map[string]any, len(child))
	for key, nested := range child {
		cloned[key] = cloneValue(nested)
	}

	return cloned
}
