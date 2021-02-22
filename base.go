package vc

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

// Return code constants
const (
	Success int = iota
	SyntaxError
	ClientError
	ServerError
	SystemError
	CodecError
	Help = cli.RunResultHelp
)

var (
	tokenFiles = []string{
		os.ExpandEnv("$HOME/.vault-token"),
		"/etc/vault-client/token",
		os.ExpandEnv("$VAULT_TOKEN_FILE"),
	}
)

// DebugLogFunc is our debug log function, defaults to nil (no debug logging)
var DebugLogFunc func(string)

// Debug is a debug message
func Debug(message string) {
	if DebugLogFunc != nil {
		DebugLogFunc(strings.TrimRight(message, " \r\n\t"))
	}
}

// Debugf is a debug message with variadic formatting
func Debugf(format string, v ...interface{}) {
	Debug(fmt.Sprintf(format, v...))
}

type baseCommand struct {
	ui cli.Ui
	c  *Client

	mode  os.FileMode
	out   string
	user  string
	group string
	tmp   *os.File
	w     io.WriteCloser
}

func (cmd *baseCommand) Client() (*Client, error) {
	var err error
	if cmd.c == nil {
		config := api.DefaultConfig()
		if err = config.ReadEnvironment(); err != nil {
			return nil, err
		}

		if cmd.c, err = NewClient(config); err != nil {
			return nil, err
		}

		// Token from environment
		if token := os.Getenv("VAULT_TOKEN"); token != "" {
			Debug("client: using VAULT_TOKEN from environment")
			cmd.c.SetToken(token)
			return cmd.c, nil
		}

		// Token from token file
		for _, tokenFile := range tokenFiles {
			if tokenFile == "" {
				continue
			}
			if fi, serr := os.Stat(tokenFile); serr == nil && !fi.IsDir() {
				b, berr := ioutil.ReadFile(tokenFile)
				if berr != nil {
					return nil, fmt.Errorf("unable to read token: %v", berr)
				}
				Debugf("client: using VAULT_TOKEN_FILE %s", tokenFile)
				cmd.c.SetToken(strings.TrimSpace(string(b)))
				break
			} else if serr != nil {
				Debugf("client: error VAULT_TOKEN_FILE %s: %v", tokenFile, serr)
			}
		}
	}
	return cmd.c, err
}

// Close the output file (if any) and rename it to cmd.out
func (cmd *baseCommand) Close() error {
	if cmd.w != nil && cmd.w != os.Stdout {
		if cmd.tmp != nil {
			tmp := cmd.tmp.Name()
			if err := cmd.w.Close(); err != nil {
				return err
			}
			return os.Rename(tmp, cmd.out)
		}
		return cmd.w.Close()
	}
	return nil
}

// writerOpen opens a file in the directory of cmd.out; with the correct mode;
// if the caller calls .Close(), the file gets renamed to cmd.out
func (cmd *baseCommand) writerOpen() error {
	if cmd.out == "" || cmd.out == "-" {
		if cmd.w == nil {
			cmd.w = os.Stdout
		}
		return nil
	}

	dir, base := filepath.Split(cmd.out)
	base = "." + base + "."

	var err error
	if cmd.tmp, err = ioutil.TempFile(dir, base); err == nil {
		Debugf("writing to %s\n", cmd.tmp.Name())
		cmd.w = cmd.tmp

		uid, err := cmd.getUserId()
		if err != nil {
			return err
		}

		gid, err := cmd.getGroupId()
		if err != nil {
			return err
		}

		if err = cmd.tmp.Chmod(cmd.mode); err != nil {
			return err
		}

		if uid != -1 || gid != -1 {
			if err = cmd.tmp.Chown(uid, gid); err != nil {
				return err
			}
		}
	}

	return err
}

func (cmd *baseCommand) getGroupId() (int, error) {
	if cmd.group == "" {
		return -1, nil
	}

	if gid, err := strconv.Atoi(cmd.group); err == nil {
		return gid, nil
	}

	grp, err := user.LookupGroup(cmd.group)
	if err != nil {
		return -1, err
	}

	if gid, err := strconv.Atoi(grp.Gid); err != nil {
		return -1, err
	} else {
		return gid, nil
	}
}

func (cmd *baseCommand) getUserId() (int, error) {
	if cmd.user == "" {
		return -1, nil
	}

	if uid, err := strconv.Atoi(cmd.user); err == nil {
		return uid, nil
	}

	usr, err := user.Lookup(cmd.user)
	if err != nil {
		return -1, err
	}

	if uid, err := strconv.Atoi(usr.Uid); err != nil {
		return -1, err
	} else {
		return uid, nil
	}
}

func (cmd *baseCommand) Write(p []byte) (int, error) {
	if cmd.w == nil {
		if err := cmd.writerOpen(); err != nil {
			return 0, err
		}
	}

	return cmd.w.Write(p)
}

type stringValue string

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

func (s *stringValue) Get() interface{} { return string(*s) }

func (s *stringValue) String() string { return string(*s) }

func defaults(fs *flag.FlagSet) string {
	b := new(bytes.Buffer)
	fs.VisitAll(func(f *flag.Flag) {
		s := fmt.Sprintf("  -%s", f.Name) // Two spaces before -; see next two comments.
		name, usage := flag.UnquoteUsage(f)
		if len(name) > 0 {
			s += " " + name
		}
		// Boolean flags of one ASCII letter are so common we
		// treat them specially, putting their usage on the same line.
		if len(s) <= 4 { // space, space, '-', 'x'.
			s += "\t"
		} else {
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			s += "\n    \t"
		}
		s += usage
		if !isZeroValue(f, f.DefValue) {
			if _, ok := f.Value.(*stringValue); ok {
				// put quotes on the value
				s += fmt.Sprintf(" (default %q)", f.DefValue)
			} else {
				s += fmt.Sprintf(" (default %v)", f.DefValue)
			}
		}
		fmt.Fprint(b, s, "\n")
	})
	return b.String()
}

// isZeroValue guesses whether the string represents the zero
// value for a flag. It is not accurate but in practice works OK.
func isZeroValue(f *flag.Flag, value string) bool {
	// Build a zero value of the flag's Value type, and see if the
	// result of calling its String method equals the value passed in.
	// This works unless the Value type is itself an interface type.
	typ := reflect.TypeOf(f.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Ptr {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	if value == z.Interface().(flag.Value).String() {
		return true
	}

	switch value {
	case "false":
		return true
	case "":
		return true
	case "0":
		return true
	}
	return false
}

// confirm prompts a message and expects yes/no
func confirm(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [yn]: ", prompt)
		var (
			input, _ = reader.ReadBytes('\n')
			char     byte
		)
		if len(input) > 1 {
			char = input[0]
		}
		switch char {
		case 'y', 'Y':
			return true
		case 'n', 'N':
			return false
		}
	}
}

// confirmf like confirm with formatting
func confirmf(format string, v ...interface{}) bool {
	return confirm(fmt.Sprintf(format, v...))
}

// DefaultCommands returns a map of default commands
func DefaultCommands(ui cli.Ui) map[string]cli.CommandFactory {
	return map[string]cli.CommandFactory{
		"cat":      CatCommandFactory(ui),
		"cp":       CopyCommandFactory(ui),
		"edit":     EditCommandFactory(ui),
		"file get": FileCommandFactory(ui, "get"),
		"file put": FileCommandFactory(ui, "put"),
		"ls":       ListCommandFactory(ui),
		"mv":       MoveCommandFactory(ui),
		"rm":       DeleteCommandFactory(ui),
		"template": TemplateCommandFactory(ui),
		"shell":    ShellCommandFactory(ui),
	}
}

// DefaultApp sets up a default CLI application
func DefaultApp(ui cli.Ui, args []string) *cli.CLI {
	app := cli.NewCLI("vc", "")
	app.Args = args
	app.Commands = DefaultCommands(ui)
	return app
}
