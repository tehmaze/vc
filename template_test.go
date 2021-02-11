package vc

import (
	"bytes"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/vault"
	"github.com/mitchellh/cli"
	"io/ioutil"
	"net"
	"os"
	"testing"
)

func TestTemplateCommand_Run(t *testing.T) {

	_, vaultClient := createTestVault(t)
	writeSecret(t, vaultClient, "secret/foo/bar", map[string]interface{}{
		"secret": "bar",
	})

	commandUnderTest, output := createCommandUnderTest(t, vaultClient)
	f := createTemplateFile(t, "Test template {{ secret \"secret/foo/bar\" \"secret\" }}")

	exitCode := commandUnderTest.Run([]string{f.Name()})
	commandOutput := output.String()
	if exitCode != 0 {
		t.Fatal("Exit code is not 0", commandOutput, exitCode)
	}

	if commandOutput != "Test template bar" {
		t.Fatal("Unexpected output", "'"+commandOutput+"'")
	}
}

func TestTemplateCommand_EscapeXml(t *testing.T) {
	_, vaultClient := createTestVault(t)
	writeSecret(t, vaultClient, "secret/foo/bar", map[string]interface{}{
		"secret": "bar",
	})

	commandUnderTest, output := createCommandUnderTest(t, vaultClient)
	f := createTemplateFile(t,
		`<?xml version="1.0" encoding="utf-8"?>
<item>
     <value>{{ secret "secret/foo/bar" "secret" }}</value>
<item>
`)

	exitCode := commandUnderTest.Run([]string{f.Name()})
	commandOutput := output.String()
	if exitCode != 0 {
		t.Fatal("Exit code is not 0", commandOutput, exitCode)
	}

	expectedOutput := `&lt;?xml version="1.0" encoding="utf-8"?>
<item>
     <value>bar</value>
<item>
`
	if commandOutput != expectedOutput {
		t.Fatal("Unexpected output", "'"+commandOutput+"'")
	}
}

func TestTemplateCommand_TextTemplateXml(t *testing.T) {
	_, vaultClient := createTestVault(t)
	writeSecret(t, vaultClient, "secret/foo/bar", map[string]interface{}{
		"secret": "bar",
	})

	commandUnderTest, output := createCommandUnderTest(t, vaultClient)
	commandUnderTest.templatingMode = "text"
	f := createTemplateFile(t,
		`<?xml version="1.0" encoding="utf-8"?>
<item>
     <value>{{ secret "secret/foo/bar" "secret" }}</value>
<item>
`)

	exitCode := commandUnderTest.Run([]string{f.Name()})
	commandOutput := output.String()
	if exitCode != 0 {
		t.Fatal("Exit code is not 0", commandOutput, exitCode)
	}

	expectedOutput := `<?xml version="1.0" encoding="utf-8"?>
<item>
     <value>bar</value>
<item>
`
	if commandOutput != expectedOutput {
		t.Fatal("Unexpected output", "'"+commandOutput+"'")
	}
}

func writeSecret(t *testing.T, vaultClient *api.Client, path string, secret map[string]interface{}) {
	_, err := vaultClient.Logical().Write(path, secret)
	if err != nil {
		t.Fatal(err)
	}
}

func createTemplateFile(t *testing.T, templateContents string) *os.File {
	f, err := ioutil.TempFile(".", "template")
	t.Cleanup(func() {
		_ = os.Remove(f.Name())
	})

	if err != nil {
		t.Fatal("Failed to create temp file", err)
	}
	_, err = f.Write([]byte(templateContents))
	if err != nil {
		t.Fatal("Failed to write to temp file", err)
	}
	_ = f.Close()
	return f
}

func createCommandUnderTest(t *testing.T, client *api.Client) (*TemplateCommand, *bytes.Buffer) {
	b := new(bytes.Buffer)

	ui := &cli.BasicUi{
		Reader:      nil,
		Writer:      b,
		ErrorWriter: b,
	}
	factory := TemplateCommandFactory(ui)
	commandUnderTest, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	templateCommand := commandUnderTest.(*TemplateCommand)
	templateCommand.c = &Client{
		Path:   "/",
		Client: client,
	}
	templateCommand.w = &byteBufferWriteCloser{ByteBuffer: b}

	return templateCommand, b
}

func createTestVault(t *testing.T) (net.Listener, *api.Client) {
	t.Helper()

	// Create an in-memory, unsealed core (the "backend", if you will).
	core, keyShares, rootToken := vault.TestCoreUnsealed(t)
	_ = keyShares

	// Start an HTTP server for the core.
	listener, addr := http.TestServer(t, core)

	// Create a client that talks to the server, initially authenticating with
	// the root token.
	conf := api.DefaultConfig()
	conf.Address = addr

	client, err := api.NewClient(conf)
	if err != nil {
		t.Fatal(err)
	}
	client.SetToken(rootToken)

	return listener, client
}

type byteBufferWriteCloser struct {
	ByteBuffer *bytes.Buffer
}

func (t *byteBufferWriteCloser) Write(data []byte) (int, error) {
	return t.ByteBuffer.Write(data)
}

func (t *byteBufferWriteCloser) Close() error {
	return nil
}
