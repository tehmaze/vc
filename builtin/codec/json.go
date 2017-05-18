package codec

import (
	"bytes"
	"encoding/json"

	"github.com/tehmaze/vc"
)

type jsonCodec struct {
}

func (c jsonCodec) Marshal(_ string, data map[string]interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c jsonCodec) Unmarshal(p []byte) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(p, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func init() {
	vc.RegisterCodec("json", new(jsonCodec))
}
