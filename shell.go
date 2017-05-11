package vc

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/mitchellh/cli"
)

// ShellHistoryFile is the file where readline history is recorded
const ShellHistoryFile = "$HOME/.vc_history"

const banner = `
      ,--.!,
   __/   -*-   	                 _               _
 ,d88b.  ` + "`" + `|'   _,    __,        // -/-     __   //  .
 888888         (_/_(_/(__(_/__(/__/_    _(_,__(/__/_
 ` + "`" + `888P'

Welcome to Vault CLI, the Vault Command Line Interface interactive shell. This
shell implements basic readline capabilities and tab completion. Type "help"
for an overview of available commands.
`

// ShellCommand is an interactive command line shell
type ShellCommand struct {
	baseCommand

	app *cli.CLI
	ui  cli.Ui

	fs    *flag.FlagSet
	force bool
	user  string
}

func (cmd *ShellCommand) Help() string {
	return "vc [<options>] shell"
}

func (cmd *ShellCommand) Synopsis() string {
	return "interactive shell"
}

func (cmd *ShellCommand) Run(args []string) int {
	client, err := cmd.Client()
	if err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}

	if len(args) > 0 {
		if strings.HasPrefix(args[0], "/") {
			client.Path = client.abspath(args[0])
		} else {
			client.Path = client.abspath("/" + args[0])
		}
	} else {
		client.Path = "/"
	}

	secret, err := client.Auth().Token().LookupSelf()
	if err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}
	if _, ok := secret.Data["id"].(string); ok {
		delete(secret.Data, "id")
	}
	Debugf("client: token: %+v", secret.Data)
	if cmd.user = secret.Data["display_name"].(string); cmd.user == "" {
		cmd.user = "?"
	}

	completer := readline.NewPrefixCompleter(
		readline.PcItem("mode",
			readline.PcItem("vi"),
			readline.PcItem("emacs"),
		),
		readline.PcItem("cd",
			readline.PcItemDynamic(client.Complete(isDir)),
		),
		readline.PcItem("cp",
			readline.PcItemDynamic(client.Complete(isAny),
				readline.PcItemDynamic(client.Complete(isAny)),
			),
		),
		readline.PcItem("edit",
			readline.PcItemDynamic(client.Complete(isAny)),
		),
		readline.PcItem("ls",
			readline.PcItemDynamic(client.Complete(isAny)),
		),
		readline.PcItem("pwd"),
		readline.PcItem("setprompt"),
		readline.PcItem("bye"),
		readline.PcItem("exit"),
		readline.PcItem("help"),
	)

	l, err := readline.NewEx(&readline.Config{
		Prompt:          cmd.prompt(),
		HistoryFile:     os.ExpandEnv(ShellHistoryFile),
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold: true,
		//FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()

	// Show banner unless ~/.hush_login exists
	if _, err = os.Stat(os.ExpandEnv("$HOME/.hush_login")); err != nil {
		cmd.ui.Output(banner)
	}

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		switch {
		case line == "help" || strings.HasPrefix(line, "help "):
			cmd.runHelp(strings.TrimSpace(line[4:]))
		case line == "cd":
			client.Path = "/"
			l.SetPrompt(cmd.prompt())
		case strings.HasPrefix(line, "cd "):
			client.Path = client.abspath(line[3:])
			l.SetPrompt(cmd.prompt())
		case line == "pwd":
			cmd.ui.Output(client.Path)
		case strings.HasPrefix(line, "mode "):
			switch line[5:] {
			case "vi":
				l.SetVimMode(true)
			case "emacs":
				l.SetVimMode(false)
			default:
				cmd.ui.Error("invalid mode: " + line[5:])
			}
		case line == "mode":
			if l.IsVimMode() {
				println("current mode: vim")
			} else {
				println("current mode: emacs")
			}
		case line == "bye" || line == "exit" || line == "quit":
			goto exit
		case line == "":
			l.SetPrompt(cmd.prompt())
		default:
			cmd.runCommand(line)
		}
	}
exit:
	return 0
}

func (cmd *ShellCommand) prompt() string {
	return fmt.Sprintf("\x1b[1;32m%s\x1b[0m@vault \x1b[1m%s\x1b[1;31m> \x1b[0m ", cmd.user, cmd.c.Path)
}

func (cmd *ShellCommand) expandArgs(args []string) string {
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			args[i] = cmd.c.abspath(arg)
		}
	}
	return strings.Join(args, " ")
}

var (
	commandsWithPathArgs = map[string]int{
		"cat": -1,
		"cd":  1,
		"cp":  2,
		"ls":  -1,
		"mv":  2,
		"rm":  1,
	}
	commandsWithDefaultPath = map[string]bool{
		"cat": true,
		"ls":  true,
	}
)

func (cmd *ShellCommand) runCommand(line string) {
	// Commands that take a path argument; we need to turn relative paths into
	// absolute paths based on our current working directory.
	args := strings.Fields(strings.TrimSpace(line))
	if len(args) == 0 {
		return
	}

	// Expand commands with path arguments, relative paths need to be resolved
	// against our working directory.
	Debugf("path=%q, args=%+v, len=%d, pathArgs=%d, defaultPath=%t", cmd.c.Path, args, len(args), commandsWithPathArgs[args[0]], commandsWithDefaultPath[args[0]])
	if _, ok := commandsWithPathArgs[args[0]]; ok {
		if len(args) == 1 {
			// Command has no arguments, check if the command takes the current
			// working directory as a default argument.
			if commandsWithDefaultPath[args[0]] {
				Debugf("args[0]=%q; with default path", args[0])
				line = args[0] + " " + cmd.c.Path
			} else {
				Debugf("args[0]=%q; no default path", args[0])
				line = args[0]
			}
		} else {
			// Expand path arguments
			Debugf("args[0]=%q; with path args %+v", args[0], args[1:])
			line = args[0] + " " + cmd.expandArgs(args[1:])
		}
	} else {
		Debugf("args[0]=%q; no path args", args[0])
	}

	Debugf("command: %q", line)
	code, err := DefaultApp(cmd.ui, strings.Fields(line)).Run()
	if err != nil {
		cmd.ui.Error(err.Error())
	}
	if code != 0 {
		Debugf("return code %d", code)
	}
}

func (cmd *ShellCommand) runHelp(line string) {
	var (
		showShellCommands bool
		args              = []string{line, "-help"}
	)
	if args[0] == "" {
		args = args[1:]
		showShellCommands = true
	}
	_, err := DefaultApp(cmd.ui, args).Run()
	if err != nil {
		cmd.ui.Error(err.Error())
	} else if showShellCommands {
		cmd.ui.Output("Available shell commands are:")
		cmd.ui.Output("    cd          set current directory")
		cmd.ui.Output("    help        get command usage")
		cmd.ui.Output("    mode        get/set readline mode")
		cmd.ui.Output("    pwd         get current directory")
		cmd.ui.Output("    quit        terminate the shell")
	}
}

func ShellCommandFactory(ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		cmd := &ShellCommand{
			ui: ui,
			baseCommand: baseCommand{
				ui: ui,
			},
		}

		cmd.fs = flag.NewFlagSet("shell", flag.ContinueOnError)
		cmd.fs.BoolVar(&cmd.force, "f", false, "force overwrite")
		cmd.fs.Usage = func() {
			fmt.Print(cmd.Help())
		}

		return cmd, nil
	}
}
