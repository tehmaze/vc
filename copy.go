package vc

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/cli"
)

// CopyCommand can display (structured) secrets
type CopyCommand struct {
	baseCommand
	fs    *flag.FlagSet
	force bool
}

func (cmd *CopyCommand) Help() string {
	return "Usage: vc [<options>] cp <source secret> <target secret>\n\nOptions:\n" + defaults(cmd.fs)
}

func (cmd *CopyCommand) Run(args []string) int {
	if err := cmd.fs.Parse(args); err != nil {
		return SyntaxError
	}
	if args = cmd.fs.Args(); len(args) != 2 {
		return Help
	}

	if args[0] == args[1] {
		return Success
	}

	client, err := cmd.Client()
	if err != nil {
		cmd.ui.Error(err.Error())
		return ClientError
	}

	// Read secret at old path
	secret, err := client.Logical().Read(strings.TrimLeft(args[0], "/"))
	if err != nil {
		cmd.ui.Error(err.Error())
		return ServerError
	}
	if secret == nil {
		cmd.ui.Error(fmt.Sprintf("no secret at %q", args[0]))
		return SyntaxError
	}

	// Check if secret at new path exists, unless force is enabled
	if !cmd.force {
		oldSecret, oerr := client.Logical().Read(strings.TrimLeft(args[1], "/"))
		if oerr != nil {
			cmd.ui.Error(oerr.Error())
			return SyntaxError
		}
		if oldSecret != nil {
			if !IsTerminal(os.Stdout.Fd()) {
				cmd.ui.Error(fmt.Sprintf("secret at %q already exists", args[1]))
				return SystemError
			}
			if !confirmf("secret at %s already exists, overwrite?", args[1]) {
				return Success
			}
		}
	}

	// Write secret at new path
	if _, err = client.Logical().Write(strings.TrimLeft(args[1], "/"), secret.Data); err != nil {
		cmd.ui.Error(err.Error())
		return ServerError
	}

	return Success
}

func (cmd *CopyCommand) Synopsis() string {
	return "copy a secret (clone)"
}

func CopyCommandFactory(ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		cmd := &CopyCommand{
			baseCommand: baseCommand{
				ui: ui,
			},
		}

		cmd.fs = flag.NewFlagSet("cp", flag.ContinueOnError)
		cmd.fs.BoolVar(&cmd.force, "f", false, "force overwrite")
		cmd.fs.Usage = func() {
			fmt.Print(cmd.Help())
		}

		return cmd, nil
	}
}
