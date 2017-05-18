package vc

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

// CatCommand can display (structured) secrets
type CatCommand struct {
	baseCommand
	fs            *flag.FlagSet
	key           string
	mod           string
	ignoreMissing bool
}

func (cmd *CatCommand) Help() string {
	return "Usage: vc cat [<options>] <secret path> [... <secret path>]\n\nOptions:\n" + defaults(cmd.fs)
}

func (cmd *CatCommand) Run(args []string) int {
	if err := cmd.fs.Parse(args); err != nil {
		return SyntaxError
	}
	if args = cmd.fs.Args(); len(args) < 1 {
		return Help
	}

	if mode, err := strconv.ParseInt(cmd.mod, 8, 32); err != nil {
		cmd.ui.Error("error: invalid mode: " + err.Error())
		return SyntaxError
	} else {
		cmd.mode = os.FileMode(mode)
	}

	c, err := cmd.Client()
	if err != nil {
		cmd.ui.Error(err.Error())
		return ClientError
	}

	// Expand globs (if any)
	if args, err = cmd.globs(c, args); err != nil {
		cmd.ui.Error(fmt.Sprintf("error: %v", err))
		return SyntaxError
	}

	buf := new(bytes.Buffer)
	for _, path := range args {
		Debugf("cat: read %q", path)
		s, err := c.Logical().Read(path)
		if err != nil {
			cmd.ui.Error(err.Error())
			return ServerError
		}
		if s == nil {
			cmd.ui.Error(fmt.Sprintf("error: %s: secret not found", path))
			return SyntaxError
		}
		var ret int
		if cmd.key == "" {
			// No explicit key given
			if _, ok := s.Data[CodecTypeKey]; ok {
				// But the __TYPE__ key is available
				ret = cmd.runTyped(path, s, buf)
			} else {
				ret = cmd.run(path, s, buf)
			}
		} else if cmd.key == CodecTypeKey {
			// Key explicitly set to CodecTypeKey
			ret = cmd.runTyped(path, s, buf)
		} else {
			// Default, keyed item
			ret = cmd.runKeyed(path, s, buf)
		}
		if ret != Success {
			return ret
		}
	}

	// Close output file that gets opened with Write
	defer func() {
		if cerr := cmd.Close(); cerr != nil {
			cmd.ui.Error(fmt.Sprintf("error: %v", cerr))
		}
	}()

	if _, err = io.Copy(cmd, buf); err != nil {
		cmd.ui.Error(fmt.Sprintf("error: %v", err))
		return SystemError
	}

	return Success
}

// globs expands one or more paths containing glob(s)
func (cmd *CatCommand) globs(c *Client, patterns []string) (expanded []string, err error) {
	for _, pattern := range patterns {
		var infos []os.FileInfo
		if infos, err = c.Glob(pattern); err != nil {
			return
		}
		if len(infos) > 0 {
			for _, info := range infos {
				expanded = append(expanded, info.Name())
			}
		}
	}
	return
}

func (cmd *CatCommand) run(path string, s *api.Secret, buf io.Writer) int {
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.Data); err != nil {
		cmd.ui.Error(fmt.Sprintf("error: %s: key %q: %v", path, cmd.key, err))
		return CodecError
	}

	return Success
}

func (cmd *CatCommand) runKeyed(path string, s *api.Secret, buf io.Writer) int {
	val, ok := s.Data[cmd.key]
	if !ok {
		if cmd.ignoreMissing {
			return Success
		}
		cmd.ui.Error(fmt.Sprintf("error: %s: key %q not found\n", path, cmd.key))
		return SyntaxError
	}

	var err error
	switch val := val.(type) {
	case []byte:
		_, err = buf.Write(val)
	case string:
		_, err = buf.Write([]byte(val))
	default:
		err = fmt.Errorf("vc: can't cat type %T", val)
	}
	if err != nil {
		cmd.ui.Error(fmt.Sprintf("error: %s: key %q: %v", path, cmd.key, err))
		return SyntaxError
	}

	return Success
}

func (cmd *CatCommand) runTyped(path string, s *api.Secret, buf io.Writer) int {
	encoderType, ok := s.Data[CodecTypeKey].(string)
	if !ok {
		if cmd.ignoreMissing {
			return Success
		}
		cmd.ui.Error(fmt.Sprintf("error: %s: key %s not found; maybe supply a key with -k?\n", path, CodecTypeKey))
		return SyntaxError
	}

	delete(s.Data, CodecTypeKey)

	c, err := CodecFor(encoderType)
	if err != nil {
		cmd.ui.Error(err.Error())
		return CodecError
	}

	var b []byte
	if b, err = c.Marshal(path, s.Data); err != nil {
		cmd.ui.Error(err.Error())
		return SystemError
	}

	if _, err = buf.Write(b); err != nil {
		cmd.ui.Error(err.Error())
		return SystemError
	}

	return Success
}

func (cmd *CatCommand) Synopsis() string {
	return "concatenate and print secrets"
}

func CatCommandFactory(ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		cmd := &CatCommand{
			baseCommand: baseCommand{
				ui: ui,
			},
		}

		cmd.fs = flag.NewFlagSet("cat", flag.ContinueOnError)
		cmd.fs.BoolVar(&cmd.ignoreMissing, "i", false, "ingore missing key")
		cmd.fs.StringVar(&cmd.key, "k", "", "key")
		cmd.fs.StringVar(&cmd.mod, "m", "0600", "output mode")
		cmd.fs.StringVar(&cmd.out, "o", "", "output (default stdout)")
		cmd.fs.Usage = func() {
			fmt.Print(cmd.Help())
		}

		return cmd, nil
	}
}
