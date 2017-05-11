package codec

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"

	"github.com/tehmaze/vc"
)

const fileContentsKey = "contents"

var (
	// ErrFileContentsMissing indicates the special contents key is missing in the Vault secret
	ErrFileContentsMissing = errors.New(`vc: file contents key "` + fileContentsKey + `"  missing`)
)

type fileCodec struct{}

func (c fileCodec) Marshal(_ string, data map[string]interface{}) ([]byte, error) {
	contents, ok := data[fileContentsKey].(string)
	if !ok {
		return nil, ErrFileContentsMissing
	}

	return base64.StdEncoding.DecodeString(contents)
}

func (c fileCodec) Unmarshal(p []byte) (map[string]interface{}, error) {
	out := new(bytes.Buffer)
	if len(p) > 0 {
		var breaker lineBreaker
		breaker.out = out
		encoder := base64.NewEncoder(base64.StdEncoding, &breaker)
		if _, err := encoder.Write(p); err != nil {
			return nil, err
		}
		encoder.Close()
		breaker.Close()
	}

	return map[string]interface{}{
		vc.CodecTypeKey: "file",
		fileContentsKey: out.String(),
	}, nil
}

func init() {
	vc.RegisterCodec("file", new(fileCodec))
}

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

const lineLength = 64

type lineBreaker struct {
	line [lineLength]byte
	used int
	out  io.Writer
}

var nl = []byte{'\n'}

func (l *lineBreaker) Write(b []byte) (n int, err error) {
	if l.used+len(b) < lineLength {
		copy(l.line[l.used:], b)
		l.used += len(b)
		return len(b), nil
	}

	n, err = l.out.Write(l.line[0:l.used])
	if err != nil {
		return
	}
	excess := lineLength - l.used
	l.used = 0

	n, err = l.out.Write(b[0:excess])
	if err != nil {
		return
	}

	n, err = l.out.Write(nl)
	if err != nil {
		return
	}

	return l.Write(b[excess:])
}

func (l *lineBreaker) Close() (err error) {
	if l.used > 0 {
		_, err = l.out.Write(l.line[0:l.used])
		if err != nil {
			return
		}
		_, err = l.out.Write(nl)
	}

	return
}
