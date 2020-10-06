package vc

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

// TemplateCommand renders (multiple) secret(s) into a templated file.
type TemplateCommand struct {
	baseCommand
	fs     *flag.FlagSet
	mod    string
	lookup map[string]map[string]string
	decode map[string]string
}

func (cmd *TemplateCommand) Help() string {
	return "Usage: vc template [<options>] <file>\n\nOptions:\n" + defaults(cmd.fs)
}

func (cmd *TemplateCommand) Synopsis() string {
	return "render a template"
}

func (cmd *TemplateCommand) Run(args []string) int {
	if err := cmd.fs.Parse(args); err != nil {
		return 1
	}
	if args = cmd.fs.Args(); len(args) != 1 {
		return cli.RunResultHelp
	}

	if mode, err := strconv.ParseInt(cmd.mod, 8, 32); err != nil {
		cmd.ui.Error("error: invalid mode: " + err.Error())
		return 1
	} else {
		cmd.mode = os.FileMode(mode)
	}

	t, err := cmd.parseTemplate(args[0])
	if err != nil {
		cmd.ui.Error("error: " + err.Error())
		return 1
	}

	s, err := cmd.executeTemplate(t)
	if err != nil {
		cmd.ui.Error("error: " + err.Error())
		return 1
	}

	if _, err = cmd.Write([]byte(s)); err != nil {
		cmd.ui.Error("error: " + err.Error())
		return 1
	}

	return 0
}

func (cmd *TemplateCommand) parseTemplate(name string) (*template.Template, error) {
	b, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	// Parse template, add a function "secret"
	return template.New(name).Funcs(template.FuncMap{
		"decode": cmd.templateDecode,
		"secret": cmd.templateSecret,
		"nested": cmd.templateNested,
	}).Parse(string(b))
}

func (cmd *TemplateCommand) executeTemplate(t *template.Template) (content string, err error) {
	// Prepare lookup tables
	cmd.lookup = make(map[string]map[string]string)
	cmd.decode = make(map[string]string)

	// Execute template: first run; here we make an inventory of what secrets are
	// required. The secret lookups will be replaced by placeholders in the
	// templateSecret function.
	w := new(bytes.Buffer)
	if err = t.Execute(w, struct{}{}); err != nil {
		return
	}
	content = w.String()

	// Time to branch out to Vault
	var client *Client
	if client, err = cmd.Client(); err != nil {
		return
	}

	if content, err = cmd.executeTemplateDecodes(client, content); err != nil {
		return
	}
	if content, err = cmd.executeTemplateSecrets(client, content); err != nil {
		return
	}
	if content, err = cmd.executeTemplateNested(client, content); err != nil {
		return
	}

	return
}

func (cmd *TemplateCommand) executeTemplateDecodes(client *Client, input string) (content string, err error) {
	content = input

	for path, k := range cmd.decode {
		var secret *api.Secret
		if secret, err = client.Logical().Read(path); err != nil {
			return
		}
		if secret == nil || secret.Data == nil {
			return "", fmt.Errorf("decode %s: not found", path)
		}

		encoderType, ok := secret.Data[CodecTypeKey].(string)
		if !ok {
			return "", fmt.Errorf("decode %s: key %s not found", path, CodecTypeKey)
		}
		delete(secret.Data, CodecTypeKey)

		c, err := CodecFor(encoderType)
		if err != nil {
			return "", err
		}

		var b []byte
		if b, err = c.Marshal(path, secret.Data); err != nil {
			return "", err
		}

		content = strings.Replace(content, k, string(b), -1)
	}

	return
}

func (cmd *TemplateCommand) executeTemplateSecrets(client *Client, input string) (content string, err error) {
	content = input

	// For each of the secret paths, lookup the secret
	for path, kv := range cmd.lookup {
		var secret *api.Secret
		if secret, err = client.Logical().Read(strings.TrimLeft(path, "/")); err != nil {
			return
		}
		if secret == nil {
			return "", fmt.Errorf("secret %s: not found", path)
		}

		// For each of the secret keys, lookup the value
		for k, placeholder := range kv {
			if strings.HasPrefix(placeholder, "_VAULT_STRING_") {
				if v, ok := secret.Data[k].(string); ok {
					content = strings.Replace(content, placeholder, v, -1)
				} else {
					return "", fmt.Errorf("secret %s: key %q not found", path, k)
				}
			}
		}
	}

	return
}

func (cmd *TemplateCommand) executeTemplateNested(client *Client, input string) (content string, err error) {
	content = input

	// For each of the secret paths, lookup the secret
	for path, kv := range cmd.lookup {
		var secret *api.Secret
		if secret, err = client.Logical().Read(strings.TrimLeft(path, "/")); err != nil {
			return
		}
		if secret == nil {
			return "", fmt.Errorf("nested %s: not found", path)
		}

		// For each of the secret keys, lookup the value
		for k, placeholder := range kv {
			if strings.HasPrefix(placeholder, "_VAULT_NESTED_") {
				keys := strings.Split(k, ".")

				var nestedData map[string]interface{}
				if err := json.Unmarshal([]byte(secret.Data[keys[0]].(string)), &nestedData); err != nil {
					return "", fmt.Errorf("nested %s/%s: failed to parse JSON", path, keys[0])
				}

				mydata := nestedData
				levels := len(keys) - 1
				for _, nestedkey := range keys[1:] {
					if levels > 1 {
						if mydata[nestedkey] == nil {
							return "", fmt.Errorf("nested %s: key %q not found", path, nestedkey)
						}
						mydata = mydata[nestedkey].(map[string]interface{})
					} else {
						if _, ok := mydata[nestedkey].(string); ok {
							content = strings.Replace(content, placeholder, mydata[nestedkey].(string), -1)
						} else {
							return "", fmt.Errorf("nested %s: key %q is not a string", path, nestedkey)
						}
					}
					levels--
				}
			}

		}
	}

	return
}

func (cmd *TemplateCommand) templateDecode(path string) string {
	if _, ok := cmd.decode[path]; !ok {
		cmd.decode[path] = cmd.randomIdentifier("decode")
	}
	return cmd.decode[path]
}

func (cmd *TemplateCommand) templateSecret(path string, key string) string {
	kv, ok := cmd.lookup[path]
	if !ok {
		cmd.lookup[path] = make(map[string]string)
		kv = cmd.lookup[path]
	}

	if _, ok = kv[key]; !ok {
		kv[key] = cmd.randomIdentifier("string")
	}

	return kv[key]
}

func (cmd *TemplateCommand) templateNested(path string, key string) string {
	// keys := strings.Split(key, ".")

	kv, ok := cmd.lookup[path]
	if !ok {
		cmd.lookup[path] = make(map[string]string)
		kv = cmd.lookup[path]
	}

	if _, ok = kv[key]; !ok {
		kv[key] = cmd.randomIdentifier("nested")
	}

	return kv[key]
}

func (cmd *TemplateCommand) randomIdentifier(t string) string {
	r := make([]byte, 8)
	io.ReadFull(rand.Reader, r)
	return fmt.Sprintf("_VAULT_%s_%x_", strings.ToUpper(t), r)
}

func TemplateCommandFactory(ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		cmd := &TemplateCommand{
			baseCommand: baseCommand{
				ui: ui,
			},
		}

		cmd.fs = flag.NewFlagSet("template", flag.ContinueOnError)
		cmd.fs.StringVar(&cmd.mod, "m", "0600", "output mode")
		cmd.fs.StringVar(&cmd.out, "o", "", "output (default: stdout)")
		cmd.fs.Usage = func() {
			fmt.Print(cmd.Help())
		}

		return cmd, nil
	}
}
