package interpreter

const (
	MemoizeCacheLimit = 200
)

type Memoize struct {
	enabled         bool
	cacheKeyBuilder func()
	cache           map[string]interface{}
	cacheSize       int
	cacheMiss       int
}
