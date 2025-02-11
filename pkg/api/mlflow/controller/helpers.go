package controller

import (
	"github.com/apache/arrow/go/v12/arrow"
	"github.com/apache/arrow/go/v12/arrow/ipc"
)

// WriteStreamingRecord writes record into stream.
func WriteStreamingRecord(w *ipc.Writer, r arrow.Record) error {
	defer r.Release()
	return w.Write(r)
}
