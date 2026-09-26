package web

import (
	"encoding/json"
	"log"
)

// jsonBytes marshals a value for the WebSocket push channel. HTML escaping is
// disabled so node labels containing <, > or & are not corrupted in transit;
// every value still reaches the browser as JSON data, never as markup, because
// the client inserts it as text.
func jsonBytes(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// logf is a thin indirection so the server's log destination can be captured
// in tests without racing on the standard logger.
var logf = log.Printf
