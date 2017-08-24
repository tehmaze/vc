package vc

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

type testCommand struct {
	Factory func(cli.Ui) cli.CommandFactory
	Args    []string
	Code    int
	Live    bool
}

func testLiveAvailable() error {
	if s := os.Getenv("VAULT_ADDR"); s == "" {
		return errors.New("live test: missing VAULT_ADDR env")
	}
	if s := os.Getenv("VAULT_TOKEN"); s == "" {
		return errors.New("live test: missing VAULT_TOKEN env")
	}
	if s := os.Getenv("VAULT_TEST_PATH"); s == "" {
		return errors.New("live test: missing VAULT_TEST_PATH env")
	}
	return nil
}

func testCommandRun(t *testing.T, test testCommand) {
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	ui := &cli.BasicUi{
		Reader:      devnull,
		Writer:      devnull,
		ErrorWriter: devnull,
	}
	app := cli.NewCLI("vc", "test")
	app.HelpWriter = devnull
	app.Args = append([]string{"test"}, test.Args...)
	app.Commands = map[string]cli.CommandFactory{
		"test": test.Factory(ui),
	}

	if app.Commands["test"] == nil {
		t.Fatalf("expected %T to return a Command, got nil", test.Factory)
	}

	if code, err := app.Run(); err != nil {
		t.Fatal(err)
	} else if code != test.Code {
		t.Fatalf("expected %q return code %d; got %d", strings.Join(app.Args, " "), test.Code, code)
	}
}
