package vc

import (
	"bytes"
	"crypto/rand"
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
}

func (cmd *TemplateCommand) Help() string {
	return "Usage: vc template [<options>] <file>\n\nOptions:\n" + defaults(cmd.fs)
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
		"secret": cmd.templateSecret,
	}).Parse(string(b))
}

func (cmd *TemplateCommand) executeTemplate(t *template.Template) (content string, err error) {
	// Prepare lookup table
	cmd.lookup = make(map[string]map[string]string)

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

	// For each of the secret paths, lookup the secret
	for path, kv := range cmd.lookup {
		var secret *api.Secret
		if secret, err = client.Logical().Read(strings.TrimLeft(path, "/")); err != nil {
			return
		}
		if secret == nil {
			err = fmt.Errorf("%s: secret not found\n", path)
			return
		}

		// For each of the secret keys, lookup the value
		for k, placeholder := range kv {
			if v, ok := secret.Data[k].(string); ok {
				content = strings.Replace(content, placeholder, v, -1)
			} else {
				err = fmt.Errorf("%s: secret key %q not found\n", path, k)
				return
			}
		}
	}

	return
}

func (cmd *TemplateCommand) templateSecret(path string, key string) string {
	kv, ok := cmd.lookup[path]
	if !ok {
		cmd.lookup[path] = make(map[string]string)
		kv = cmd.lookup[path]
	}

	kv[key] = cmd.randomIdentifier()
	return kv[key]
}

func (cmd *TemplateCommand) randomIdentifier() string {
	r := make([]byte, 8)
	io.ReadFull(rand.Reader, r)
	return fmt.Sprintf("_VAULT_SECRET_%x_", r)
}

func (cmd *TemplateCommand) Synopsis() string {
	return "render a template"
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
