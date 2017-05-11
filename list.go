package vc

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mitchellh/cli"
)

// ListCommand can display (structured) secrets
type ListCommand struct {
	baseCommand
	fs      *flag.FlagSet
	compact bool
	long    bool
	recurse bool
}

func (cmd *ListCommand) Help() string {
	return "Usage: vc [<options>] ls [<secret path>] [... <secret path>]\n\nOptions:\n" + defaults(cmd.fs)
}

func (cmd *ListCommand) Run(args []string) int {
	if err := cmd.fs.Parse(args); err != nil {
		return 1
	}
	args = cmd.fs.Args()

	client, err := cmd.Client()
	if err != nil {
		cmd.ui.Error(err.Error())
		return 2
	}

	if len(args) == 0 {
		return cmd.list(client, ".")
	}

	var ret int
	for _, path := range args {
		if code := cmd.list(client, path); code > ret {
			ret = code
		}
	}
	return ret
}

func (cmd *ListCommand) list(client *Client, path string) int {
	//path = client.abspath(path)

	infos, err := client.Glob(path)
	if err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}

	if len(infos) == 1 && infos[0].IsDir() {
		// Single item found and it's a directory; list its contents
		infos, err = client.ReadDir(infos[0].Name())
		if err != nil {
			cmd.ui.Error(err.Error())
			return 1
		}
	}
	if len(infos) == 0 {
		cmd.ui.Error(fmt.Sprintf("%s: not found", path))
		return 1
	}

	if cmd.recurse {
		fmt.Println(path + ":")
	}

	var (
		names []string
		files = make(map[string]os.FileInfo)
	)
	for _, info := range infos {
		name := info.Name()
		names = append(names, name)
		files[name] = info
	}

	sort.Strings(names)
	for _, name := range names {
		info := files[name]
		var t = '-'
		if info.IsDir() {
			t = 'd'
		}
		if cmd.long {
			fmt.Printf("%c%s %s\n", t, info.Mode(), name)
		} else {
			fmt.Println(name)
		}
	}

	if cmd.recurse {
		for _, name := range names {
			info := files[name]
			if !info.IsDir() {
				continue
			}
			fmt.Println("")
			if code := cmd.list(client, info.Name()); code != 0 {
				return code
			}
		}
	}

	return 0
}

func (cmd *ListCommand) listMounts(client *Client) int {
	mounts, err := client.Sys().ListMounts()
	if err != nil {
		cmd.ui.Error(err.Error())
		return 1
	}

	var names []string
	for name, mount := range mounts {
		if mount.Type != "generic" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return 0
	}

	if cmd.recurse {
		fmt.Println("/:")
	}

	sort.Strings(names)
	for _, name := range names {
		name = strings.TrimSuffix(name, "/")

		if cmd.long {
			fmt.Printf("drwxr-x--- %s\n", name)
		} else {
			fmt.Println(name)
		}

		if cmd.recurse {
			fmt.Println("")
			if code := cmd.list(client, name); code != 0 {
				return code
			}
		}
	}

	return 0
}

func (cmd *ListCommand) Synopsis() string {
	return "list secrets"
}

func ListCommandFactory(ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		cmd := &ListCommand{
			baseCommand: baseCommand{
				ui: ui,
			},
		}

		cmd.fs = flag.NewFlagSet("ls", flag.ContinueOnError)
		cmd.fs.BoolVar(&cmd.compact, "1", false, "list in compact format")
		cmd.fs.BoolVar(&cmd.long, "l", false, "list in long format")
		cmd.fs.BoolVar(&cmd.recurse, "R", false, "recursively list subdirectories encountered")
		cmd.fs.Usage = func() {
			fmt.Print(cmd.Help())
		}

		return cmd, nil
	}
}
