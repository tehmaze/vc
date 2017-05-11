package vc

import (
	"flag"
	"fmt"
	"os"

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
		return 1
	}
	if args = cmd.fs.Args(); len(args) != 2 {
		return cli.RunResultHelp
	}

	if args[0] == args[1] {
		return 0
	}

	client, err := cmd.Client()
	if err != nil {
		cmd.ui.Error(err.Error())
		return 2
	}

	// Read secret at old path
	secret, err := client.Logical().Read(args[0])
	if err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}
	if secret == nil {
		cmd.ui.Error(fmt.Sprintf("no secret at %q", args[0]))
		return 1
	}

	// Check if secret at new path exists, unless force is enabled
	if !cmd.force {
		oldSecret, oerr := client.Logical().Read(args[1])
		if oerr != nil {
			cmd.ui.Error(oerr.Error())
			return 1
		}
		if oldSecret != nil {
			if !IsTerminal(os.Stdout.Fd()) {
				cmd.ui.Error(fmt.Sprintf("secret at %q already exists", args[1]))
				return 1
			}
			if !confirmf("secret at %s already exists, overwrite?", args[1]) {
				return 0
			}
		}
	}

	// Write secret at new path
	if _, err = client.Logical().Write(args[1], secret.Data); err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}

	return 0
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
