package gws

import (
	"encoding/json"
	"io"
)

type (
	Codec interface {
		NewEncoder(io.Writer) Encoder
	}

	Encoder interface {
		Encode(v interface{}) error
	}

	jsonCodec struct{}
)

func (c jsonCodec) NewEncoder(writer io.Writer) Encoder {
	return json.NewEncoder(writer)
}

var JsonCodec = new(jsonCodec)
