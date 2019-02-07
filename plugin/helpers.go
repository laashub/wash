package plugin

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/puppetlabs/wash/log"
)

// JSONToJSONMap converts JSON to a map of its top-level keys to JSON serialized values.
func JSONToJSONMap(inrec []byte) (map[string][]byte, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(inrec, &data); err != nil {
		return nil, err
	}

	var err error
	d := make(map[string][]byte)
	for k, v := range data {
		d[k], err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
	}
	return d, nil
}

// TrackTime helper is useful for timing functions.
// Use with `defer plugin.TrackTime(time.Now(), "funcname")`.
func TrackTime(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

// PrefetchOpen can be called to open a file for DefaultTimeout (if it supports Close).
// Commonly used as `go PrefetchOpen(...)` to kick off prefetching asynchronously.
func PrefetchOpen(file FileProtocol) {
	buf, err := file.Open(context.Background())
	if closer, ok := buf.(io.Closer); err == nil && ok {
		go func() {
			time.Sleep(DefaultTimeout)
			closer.Close()
		}()
	}
}