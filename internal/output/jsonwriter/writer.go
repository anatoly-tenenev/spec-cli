// Package jsonwriter encodes a response as a single JSON line. HTML escaping
// is disabled so that expressions and paths appear literally in the output
// rather than as \u escapes.
package jsonwriter

import (
	"encoding/json"
	"io"
)

type Writer struct {
	enc *json.Encoder
}

func New(out io.Writer) *Writer {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return &Writer{enc: enc}
}

func (w *Writer) Write(v any) error {
	return w.enc.Encode(v)
}
