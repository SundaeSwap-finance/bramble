package bramble

import "context"

// SchemaCache provides persistent caching of federated service schemas.
// Implementations store and retrieve a mapping from service URLs to their
// raw SDL (Schema Definition Language) strings.
type SchemaCache interface {
	// Load returns the cached service schemas as a map from service URL to SDL string.
	Load(ctx context.Context) (map[string]string, error)
	// Save persists the service schemas.
	Save(ctx context.Context, schemas map[string]string) error
}
