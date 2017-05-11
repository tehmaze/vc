package vc

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// CodecTypeKey is the key that marks a named Codec
	CodecTypeKey = "__TYPE__"

	// ErrMarshalingNotSupported returned by MarshalingNotSupported
	ErrMarshalingNotSupported = errors.New("vc: marshaling not supported by codec")

	// ErrUnmarshalingNotSupported returned by UnmarshalingNotSupported
	ErrUnmarshalingNotSupported = errors.New("vc: unmarshaling not supported by codec")

	codecMutex sync.RWMutex
	codecs     = map[string]Codec{}
)

// Marshaler can marshal a api.Secret.Data into a byte slice
type Marshaler interface {
	Marshal(path string, data map[string]interface{}) ([]byte, error)
}

// MarshalingNotSupported is a placeholder Marshaler that returns an error
// upon marshaling.
type MarshalingNotSupported struct{}

// Marshal always returns ErrMarshalingNotSupported
func (m MarshalingNotSupported) Marshal(_ map[string]interface{}) ([]byte, error) {
	return nil, ErrMarshalingNotSupported
}

// Unmarshaler can unmarshal a byte slice into api.Secret.Data
type Unmarshaler interface {
	Unmarshal(p []byte) (map[string]interface{}, error)
}

// UnmarshalingNotSupported is a placeholder Unmarshaler that returns an error
// upon unmarshaling.
type UnmarshalingNotSupported struct{}

// Unmarshal always returns ErrUnmarshalingNotSupported
func (u UnmarshalingNotSupported) Unmarshal(_ []byte) (map[string]interface{}, error) {
	return nil, ErrUnmarshalingNotSupported
}

// Codec implements an Encoder and Decoder
type Codec interface {
	Marshaler
	Unmarshaler
}

// ReplaceCodec replaces or adds a named codec
func ReplaceCodec(name string, c Codec) (exists bool) {
	codecMutex.Lock()
	defer codecMutex.Unlock()

	_, exists = codecs[name]
	codecs[name] = c
	return
}

// RegisterCodec adds a new named codec
func RegisterCodec(name string, c Codec) {
	codecMutex.Lock()
	if d, dupe := codecs[name]; dupe {
		panic(fmt.Sprintf("vc: codec %q already registered as %T", name, d))
	}
	codecs[name] = c
	codecMutex.Unlock()
}

// CodecFor returns a codec by name
func CodecFor(name string) (Codec, error) {
	codecMutex.RLock()
	defer codecMutex.RUnlock()

	c, ok := codecs[name]
	if !ok {
		return nil, fmt.Errorf("vc: no codec available for type %q", name)
	}
	return c, nil
}
