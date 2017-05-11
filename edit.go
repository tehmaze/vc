package vc

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

const (
	editingNew = `# You are editing a new secret, data can be entered as
# structured YaML, see http://yaml.org/
#
# Lines starting with a hash (#) are ignored.
`
	editingOld = editingNew + `#
# Removing all content will delete the secret from Vault.
`
)

// EditCommand opens Vault secrets in an interactive editor ($EDITOR)
type EditCommand struct {
	baseCommand
	fs     *flag.FlagSet
	lookup map[string]map[string]string
}

func (cmd *EditCommand) Help() string {
	return "Usage: vc edit <secret path>"
}

func (cmd *EditCommand) Run(args []string) int {
	if err := cmd.fs.Parse(args); err != nil {
		return 1
	}
	if args = cmd.fs.Args(); len(args) != 1 {
		return cli.RunResultHelp
	}

	client, err := cmd.Client()
	if err != nil {
		cmd.ui.Error(err.Error())
		return 2
	}

	var (
		name   string
		exists bool
	)
	if name, exists, err = cmd.readSecret(client, args[0]); err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}
	defer os.Remove(name)

	var data map[string]interface{}
	if data, err = cmd.editSecret(name); err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}

	if len(data) == 0 {
		if !exists {
			cmd.ui.Warn("no data was saved")
			return 0
		}
		if _, err = client.Logical().Delete(args[0]); err != nil {
			cmd.ui.Error(err.Error())
			return 1
		}
		cmd.ui.Info(fmt.Sprintf("secret at %s removed", args[0]))
		return 0
	}

	if _, err = client.Logical().Write(args[0], data); err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}

	cmd.ui.Info(fmt.Sprintf("secret at %s saved", args[0]))
	return 0
}

// editSecret edits a secret, unmarshals it from YaML
func (cmd *EditCommand) editSecret(name string) (data map[string]interface{}, err error) {
	editor := os.ExpandEnv("$EDITOR")
	if editor == "" {
		cmd.ui.Warn("no $EDITOR set, defaulting to vi")
		editor = "vi"
	}

	// Invoke editor
again:
	ecmd := exec.Command(editor, name)
	ecmd.Stdin = os.Stdin
	ecmd.Stdout = os.Stdout
	ecmd.Stderr = os.Stderr
	if err = ecmd.Run(); err != nil {
		return
	}

	// Read file contents
	var b []byte
	if b, err = ioutil.ReadFile(name); err != nil {
		return
	}

	// Unmarshal contents
	data = make(map[string]interface{})
	if marshalErr := yaml.Unmarshal(b, data); marshalErr != nil {
		cmd.ui.Error(marshalErr.Error())
		if confirm("edit again?") {
			goto again
		}
		err = errors.New("aborted")
	}

	return
}

// readSecret loads a secret, marshals it to YaML and saves it to a temporary file
func (cmd *EditCommand) readSecret(client *Client, path string) (name string, exists bool, err error) {
	var secret *api.Secret
	if secret, err = client.Logical().Read(path); err != nil {
		return
	}

	exists = secret != nil

	var b []byte
	if exists {
		if b, err = yaml.Marshal(secret.Data); err != nil {
			return
		}
		b = append([]byte(editingOld), b...)
	} else {
		b = []byte(editingNew)
	}

	var f *os.File
	if f, err = tempFile(".yaml"); err != nil {
		return
	}

	name = f.Name()
	if _, err = f.Write(b); err != nil {
		f.Close()
	} else {
		err = f.Close()
	}

	return
}

func (cmd *EditCommand) Synopsis() string {
	return "edit a secret"
}

func EditCommandFactory(ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		cmd := &EditCommand{
			baseCommand: baseCommand{
				ui: ui,
			},
		}

		cmd.fs = flag.NewFlagSet("edit", flag.ContinueOnError)
		cmd.fs.Usage = func() {
			fmt.Print(cmd.Help())
		}

		return cmd, nil
	}
}

// seed is a helper function for tempFile
func seed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

// tempFile creates a temporary file with given suffix
func tempFile(suffix string) (*os.File, error) {
	tmp := os.TempDir()
	if tmp == "" {
		tmp = "/tmp"
	}
	for index := 1; index < 10000; index++ {
		path := filepath.Join(tmp, fmt.Sprintf("vc%s%03d%s", strconv.Itoa(int(1e9 + seed()%1e9))[1:], index, suffix))
		if _, err := os.Stat(path); err != nil {
			return os.Create(path)
		}
	}
	// Give up
	return nil, fmt.Errorf("could not create unique temporary file")
}
