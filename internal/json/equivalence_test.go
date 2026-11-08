// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	segmentio "github.com/segmentio/encoding/json"
)

// equivalenceInputs cover what the decoder must read exactly as the
// segmentio decoder did: case-sensitive names, duplicates, exact numbers,
// nulls, invalid UTF-8, escapes, trailing data after the first value and
// malformed input.
var equivalenceInputs = []string{
	`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{"a":[1,2.5,-3e2,"s",null,true]}}}`,
	`{"jsonrpc":"2.0","id":"abc","result":{"content":[{"type":"text","text":"ok"}],"isError":false}}`,
	`{"field":"v","custom_tag":"t","Nested":{"field":"n"},"num":12345678901234567890,"raw":{"k":[1,{"z":null}]}}`,
	`{"Field":"v","Custom_tag":"t","nested":{"Field":"n"},"NUM":1}`,
	`{"field":"a","field":"b","num":1,"num":2}`,
	`{"field":null,"num":null,"raw":null,"Nested":null,"list":null}`,
	"{\"field\":\"\xff\xfe ok \xc3\x28\",\"raw\":\"\xff\"}",
	`{"field":"\u00e8\ud83d\ude00\n\t\"\\\/\u0000","list":["\ud800","x"]}`,
	`{"num":1e400}`,
	`{"num":-0,"float":1.7976931348623157e308,"small":5e-324}`,
	`{"num":1.5}`,
	`{"num":"1"}`,
	`{"field":1}`,
	`{"list":{"a":1}}`,
	`{"a":1} trailing`,
	`{"a":1}{"b":2}`,
	`[1,2,3] [4]`,
	`  {"a" : 1 , "b" : [ ] }  `,
	``,
	`   `,
	`{"a":}`,
	`{"a":1,}`,
	`[1,2,]`,
	`{"a" 1}`,
	`{'a':1}`,
	`{"a":tru}`,
	`{"a":"unterminated`,
	`nul`,
	`123`,
	`"top"`,
	`null`,
	`{"a":01}`,
	`{"a":.5}`,
	`{"a":1.}`,
	`{"a":+1}`,
	`{"a":"\x"}`,
	`{"a":"\u12"}`,
	"{\"a\":\"tab\there\"}",
}

type equivalenceNested struct {
	Field string `json:"field"`
}

type equivalenceTarget struct {
	Field       string
	TaggedField string `json:"custom_tag"`
	Nested      *equivalenceNested
	Num         int64           `json:"num"`
	Float       float64         `json:"float"`
	Small       float64         `json:"small"`
	Raw         json.RawMessage `json:"raw"`
	List        []any           `json:"list"`
}

func segmentioDecode(in string, v any) error {
	dec := segmentio.NewDecoder(bytes.NewReader([]byte(in)))
	dec.DontMatchCaseInsensitiveStructFields()
	return dec.Decode(v)
}

// TestDecoderMatchesSegmentio decodes every input into an interface, a map,
// a slice, a struct and raw bytes, with the segmentio decoder this package
// used before and with encoding/json/v2, through Decoder and through
// Unmarshal (whose pooled decoder sees failures and successes in turn):
// the values and the error outcomes are equal.
func TestDecoderMatchesSegmentio(t *testing.T) {
	decoders := map[string]func(string, any) error{
		"Decoder":   func(in string, v any) error { return NewDecoder(strings.NewReader(in)).Decode(v) },
		"Unmarshal": func(in string, v any) error { return Unmarshal([]byte(in), v) },
	}
	for name, decode := range decoders {
		t.Run(name, func(t *testing.T) { matchSegmentio(t, decode) })
	}
}

func matchSegmentio(t *testing.T, decode func(string, any) error) {
	targets := []func() any{
		func() any { return new(any) },
		func() any { return new(map[string]any) },
		func() any { return new([]any) },
		func() any { return new(equivalenceTarget) },
		func() any { return new(json.RawMessage) },
	}
	for _, in := range equivalenceInputs {
		for _, target := range targets {
			want, got := target(), target()
			wantErr := segmentioDecode(in, want)
			gotErr := decode(in, got)
			name := reflect.TypeOf(want).Elem().String()
			if (wantErr == nil) != (gotErr == nil) {
				t.Errorf("%q into %s: error %v, segmentio %v", in, name, gotErr, wantErr)
				continue
			}
			if errors.Is(wantErr, io.EOF) != errors.Is(gotErr, io.EOF) {
				t.Errorf("%q into %s: io.EOF %v, segmentio %v", in, name, gotErr, wantErr)
			}
			if wantErr == nil && !reflect.DeepEqual(want, got) {
				t.Errorf("%q into %s:\n got %#v\nwant %#v", in, name, got, want)
			}
		}
	}
}

// TestDecoderReadsAStream reads consecutive values from one stream as the
// segmentio decoder did, then io.EOF.
func TestDecoderReadsAStream(t *testing.T) {
	const stream = `{"a":1} {"b":[2]}
"c" 4 null`
	want := segmentio.NewDecoder(strings.NewReader(stream))
	got := NewDecoder(strings.NewReader(stream))
	for i := 0; ; i++ {
		var w, g any
		wantErr, gotErr := want.Decode(&w), got.Decode(&g)
		if (wantErr == nil) != (gotErr == nil) || !reflect.DeepEqual(w, g) {
			t.Fatalf("value %d: got %#v (%v), want %#v (%v)", i, g, gotErr, w, wantErr)
		}
		if wantErr != nil {
			if !errors.Is(gotErr, io.EOF) || !errors.Is(wantErr, io.EOF) {
				t.Fatalf("value %d: end %v, segmentio %v", i, gotErr, wantErr)
			}
			return
		}
	}
}

// wireMessage has the shape of a JSON-RPC message as jsonrpc2 reads it.
type wireMessage struct {
	VersionTag string          `json:"jsonrpc"`
	ID         any             `json:"id,omitempty"`
	Method     string          `json:"method,omitempty"`
	Params     json.RawMessage `json:"params,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      *struct {
		Code    int64           `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	} `json:"error,omitempty"`
}

var benchmarkMessages = [][]byte{
	[]byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"search_rooms","arguments":{"checkIn":"2026-12-20","checkOut":"2026-12-27","guests":[{"age":34},{"age":31},{"age":6}],"notes":"vista mare, piano alto"}}}`),
	[]byte(`{"jsonrpc":"2.0","id":7,"result":{"content":[{"type":"text","text":"Camera Deluxe vista mare: 3 notti, colazione inclusa, 1.280,00 EUR. Camera Superior: 3 notti, 1.040,00 EUR."}],"structuredContent":{"rooms":[{"code":"DLX","price":1280.0},{"code":"SUP","price":1040.0}]},"isError":false}}`),
}

// BenchmarkUnmarshalWire reads JSON-RPC messages as the MCP transport does.
func BenchmarkUnmarshalWire(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		for _, m := range benchmarkMessages {
			var w wireMessage
			if err := Unmarshal(m, &w); err != nil {
				b.Fatal(err)
			}
		}
	}
}
