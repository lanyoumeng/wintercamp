//go:build !memstats

package filter2

func logMemStats() {
	// no-op without build tag
}
