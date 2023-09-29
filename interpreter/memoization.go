package interpreter

const (
	MemoizeCacheLimit = 200
)

type Memoize struct {
	enabled              bool
	cache                map[string]interface{}
	cacheSize, cacheMiss int
}
