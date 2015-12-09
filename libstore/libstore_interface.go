package libstore

type Libstore interface {
	Get(key string) (string, error)
	Put(key, value string) error
	// The list may have duplicate entries.
	GetList(key string) ([]string, error)
	AppendToList(key, newItem string) error
	RemoveFromList(key, removeItem string) error
}
