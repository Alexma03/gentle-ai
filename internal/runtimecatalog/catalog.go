package runtimecatalog

import "context"

// DiscoverCatalog is retained only as a compatibility seam for dormant
// persisted profile state. Retired runtime catalogs are never queried.
func DiscoverCatalog(context.Context, string) (map[string]Provider, error) {
	return map[string]Provider{}, nil
}
