package vc

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

// FileCommand stores and retrieves raw files (blobs).
type FileCommand struct {
	baseCommand
	fs            *flag.FlagSet
	sub           string
	key           string
	mod           string
	encoding      string
	ignoreMissing bool
	force         bool
}

func (cmd *FileCommand) Help() string {
	return "Usage: vc file <get|put> <secret path> [<file path>]\n\nOptions:\n" + defaults(cmd.fs)
}

func (cmd *FileCommand) Run(args []string) int {
	if err := cmd.fs.Parse(args); err != nil {
		return 1
	}
	if args = cmd.fs.Args(); len(args) < 1 {
		cmd.fs.Usage()
		return 1
	}

	// Assume stdio if file path argument is missing
	if len(args) == 1 {
		args = append(args, "-")
	}

	var err error
	switch cmd.sub {
	case "get":
		err = cmd.runGet(args[0], args[1])
	case "put":
		err = cmd.runPut(args[0], args[1])
	default:
		return cli.RunResultHelp
	}

	if err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}

	return 0
}

// runGet gets a file from Vault
func (cmd *FileCommand) runGet(path, name string) (err error) {
	if !cmd.force && name != "" && name != "-" {
		if _, infoErr := os.Stat(name); infoErr == nil {
			if !IsTerminal(os.Stdout.Fd()) {
				return fmt.Errorf("%s: already exists", name)
			}
			if !confirmf("%s: already exists, overwrite?", name) {
				return nil
			}
		}
	}

	var mode int64
	if mode, err = strconv.ParseInt(cmd.mod, 8, 32); err != nil {
		return fmt.Errorf("invalid mode %q", cmd.mod)
	}
	cmd.mode = os.FileMode(mode)

	var client *Client
	if client, err = cmd.Client(); err != nil {
		return
	}

	var secret *api.Secret
	if secret, err = client.Logical().Read(strings.TrimPrefix(path, "/")); err != nil {
		return
	}
	if secret == nil {
		if cmd.ignoreMissing {
			return nil
		}
		return fmt.Errorf("no secret at %q", path)
	}

	kind, ok := secret.Data["__TYPE__"].(string)
	if !ok {
		return fmt.Errorf("secret at %q has no type marker", path)
	} else if kind != "file" {
		return fmt.Errorf("secret at %q is not a file", path)
	}

	contents, ok := secret.Data["contents"].(string)
	if !ok {
		return fmt.Errorf("secret at %q has no content", path)
	}

	var data []byte
	if data, err = base64.StdEncoding.DecodeString(contents); err != nil {
		return
	}

	if name == "" || name == "-" {
		_, err = os.Stdout.Write(data)
	} else {
		err = ioutil.WriteFile(name, data, cmd.mode)
	}

	return
}

// runPut puts a file in Vault
func (cmd *FileCommand) runPut(path, name string) (err error) {
	var b []byte
	if name == "" || name == "-" {
		if b, err = ioutil.ReadAll(os.Stdin); err != nil {
			return
		}
	} else if b, err = ioutil.ReadFile(name); err != nil {
		return
	}

	var client *Client
	if client, err = cmd.Client(); err != nil {
		return
	}

	if !cmd.force {
		if secret, _ := client.Logical().Read(strings.TrimPrefix(path, "/")); secret != nil {
			if !IsTerminal(os.Stdout.Fd()) || name == "" || name == "-" {
				return fmt.Errorf("secret at %q already exists", path)
			}
			if !confirmf("secret at %s already exists, overwrite?", path) {
				return nil
			}
		}
	}

	out := new(bytes.Buffer)
	var breaker lineBreaker
	breaker.out = out

	b64 := base64.NewEncoder(base64.StdEncoding, &breaker)
	if _, err = b64.Write(b); err != nil {
		return err
	}
	b64.Close()
	breaker.Close()

	_, err = client.Logical().Write(strings.TrimPrefix(path, "/"), map[string]interface{}{
		CodecTypeKey: "file",
		"contents":   out.String(),
	})

	return
}

func (cmd *FileCommand) Synopsis() string {
	return "store and retrieve files"
}

func FileCommandFactory(ui cli.Ui, sub string) cli.CommandFactory {
	return func() (cli.Command, error) {
		cmd := &FileCommand{
			sub: sub,
			baseCommand: baseCommand{
				ui: ui,
			},
		}

		cmd.fs = flag.NewFlagSet("file", flag.ContinueOnError)
		cmd.fs.BoolVar(&cmd.ignoreMissing, "i", false, "ignore missing key")
		cmd.fs.BoolVar(&cmd.force, "f", false, "force overwrite")
		cmd.fs.StringVar(&cmd.mod, "m", "0600", "output mode (for put)")
		cmd.fs.Usage = func() {
			fmt.Print(cmd.Help())
		}

		return cmd, nil
	}
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
