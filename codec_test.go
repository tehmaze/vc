package vc

import (
	"bytes"
	"encoding/json"
	"testing"
)

type testCodec struct {
}

func (c testCodec) Marshal(_ string, data map[string]interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c testCodec) Unmarshal(p []byte) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	if err := json.Unmarshal(p, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func TestCodec(t *testing.T) {
	RegisterCodec("test", new(testCodec))

	c, err := CodecFor("test")
	if err != nil {
		t.Fatal(err)
	}

	b, err := c.Marshal("test", map[string]interface{}{
		"test": float64(42),
	})
	if err != nil {
		t.Fatal(err)
	}

	u, err := c.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := u["test"]; !ok {
		t.Fatal("expected unmarshal[\"test\"] to exist")
	}
}
