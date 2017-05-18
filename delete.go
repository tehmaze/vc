package vc

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"
)

// DeleteCommand can display (structured) secrets
type DeleteCommand struct {
	baseCommand
	fs    *flag.FlagSet
	force bool
}

func (cmd *DeleteCommand) Help() string {
	return "Usage: vc rm <secret path>\n\nOptions:\n" + defaults(cmd.fs)
}

func (cmd *DeleteCommand) Run(args []string) int {
	if err := cmd.fs.Parse(args); err != nil {
		return SyntaxError
	}
	if args = cmd.fs.Args(); len(args) != 1 {
		return Help
	}

	client, err := cmd.Client()
	if err != nil {
		cmd.ui.Error(err.Error())
		return ClientError
	}

	if !cmd.force {
		secret, err := client.Logical().Read(args[0])
		if err != nil {
			cmd.ui.Error(err.Error())
			return ServerError
		}
		if secret == nil {
			cmd.ui.Error(fmt.Sprintf("secret at %q does not exist", args[0]))
			return SyntaxError
		}
	}

	if _, err := client.Logical().Delete(args[0]); err != nil {
		cmd.ui.Error(err.Error())
		return ServerError
	}

	return Success
}

func (cmd *DeleteCommand) Synopsis() string {
	return "remove a secret"
}

func DeleteCommandFactory(ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		cmd := &DeleteCommand{
			baseCommand: baseCommand{
				ui: ui,
			},
		}

		cmd.fs = flag.NewFlagSet("rm", flag.ContinueOnError)
		cmd.fs.BoolVar(&cmd.force, "f", false, "force removal")
		cmd.fs.Usage = func() {
			fmt.Print(cmd.Help())
		}

		return cmd, nil
	}
}
