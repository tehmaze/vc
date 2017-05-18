// +build yaml

package codec

import (
	"github.com/tehmaze/vc"
	yaml "gopkg.in/yaml.v2"
)

type yamlCodec struct {
}

func (c yamlCodec) Marshal(_ string, data map[string]interface{}) ([]byte, error) {
	return yaml.Marshal(data)
}

func (c yamlCodec) Unmarshal(p []byte) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal(p, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func init() {
	vc.RegisterCodec("yaml", new(yamlCodec))
}
