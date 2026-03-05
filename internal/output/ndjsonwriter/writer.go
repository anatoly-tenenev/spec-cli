package ndjsonwriter

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

func (w *Writer) Write(record any) error {
	return w.enc.Encode(record)
}
