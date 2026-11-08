// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// Package json provides internal JSON utilities.

package json

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"sync"
)

// decodeOptions are the semantics of encoding/json with the object names
// matched case-sensitively: the last of duplicate names wins, invalid UTF-8
// becomes U+FFFD, numbers in an interface are float64.
var decodeOptions = jsonv2.JoinOptions(json.DefaultOptionsV1(), jsonv2.MatchCaseInsensitiveNames(false))

// defaultMaxDepth is the maximum JSON nesting depth accepted by [Unmarshal].
// Realistic MCP payloads nest only a handful of levels, so 1000 is far more
// than any legitimate message requires while keeping worst-case parse cost
// bounded.
const defaultMaxDepth = 1000

// errMaxDepthExceeded is returned by [Unmarshal] when the input nests deeper than [defaultMaxDepth].
var errMaxDepthExceeded = fmt.Errorf("json: exceeded maximum nesting depth of %d", defaultMaxDepth)

// Decoder reads a stream of JSON values, one per Decode.
type Decoder struct {
	dec *jsontext.Decoder
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{dec: jsontext.NewDecoder(r, decodeOptions)}
}

// Decode reads the next JSON value into v; it reports io.EOF when the
// stream has no more values.
func (d *Decoder) Decode(v any) error {
	return jsonv2.UnmarshalDecode(d.dec, v, decodeOptions)
}

// bytesDecoder reads directly from the bytes held by buf (jsontext does
// not copy a bytes.Buffer); both are reused across calls.
type bytesDecoder struct {
	buf bytes.Buffer
	dec jsontext.Decoder
}

var bytesDecoders = sync.Pool{New: func() any { return new(bytesDecoder) }}

// Unmarshal reads the first JSON value of data into v, as a Decoder would;
// data after that value is not read.
func Unmarshal(data []byte, v any) error {
	if err := checkMaxDepth(data, defaultMaxDepth); err != nil {
		return err
	}
	d := bytesDecoders.Get().(*bytesDecoder)
	d.buf = *bytes.NewBuffer(data)
	d.dec.Reset(&d.buf, decodeOptions)
	err := jsonv2.UnmarshalDecode(&d.dec, v)
	// Drop the reference to data before the decoder goes back to the pool.
	d.buf = bytes.Buffer{}
	d.dec.Reset(&d.buf)
	bytesDecoders.Put(d)
	return err
}

// checkMaxDepth scans data once and reports [errMaxDepthExceeded] if the
// nesting of JSON objects and arrays exceeds maxDepth. It is a lightweight pass which
// tracks '{' and '[' against '}' and ']' while skipping over the contents of strings.
func checkMaxDepth(data []byte, maxDepth int) error {
	var (
		depth    int
		inString bool
		escaped  bool
	)
	for _, b := range data {
		if inString {
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inString = false
			}
			continue
		}

		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxDepth {
				return errMaxDepthExceeded
			}
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return nil
}
