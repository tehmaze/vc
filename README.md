# vc

[![GoDoc](https://godoc.org/github.com/tehmaze/vc?status.svg)](https://godoc.org/github.com/tehmaze/vc)
[![Build Status](https://travis-ci.org/tehmaze/vc.svg?branch=master)](https://travis-ci.org/tehmaze/vc)
[![GitHub downloads](https://img.shields.io/github/downloads/tehmaze/vc/total.svg)](https://github.com/tehmaze/vc)
[![GitHub tag](https://img.shields.io/github/tag/tehmaze/vc.svg)](https://github.com/tehmaze/vc)

Vault Command Line (CLI) Client for manipulating secrets inside Vault

## Environment Variables

vc respects the following environment settings:
 * `VAULT_ADDR`   Vault server address
 * `VAULT_CACERT` Path to a PEM-encoded CA cert file to use to verify the Vault server SSL certificate.
 * `VAULT_CAPATH` Path to a directory of PEM-encoded CA cert files to verify the Vault server SSL certificate. If `VAULT_CACERT` is specified, its value will take precedence.
 * `VAULT_TOKEN` Vault access token
 * `VAULT_TOKEN_FILE` Vault access token file

If no `VAULT_TOKEN` is set, `VAULT_TOKEN_FILE` will try:

    $HOME/.vault-token
    /etc/vault-client/token

# Commands

## Command cat

Show the contents of a secret.

    Usage: vc cat [<options>] <secret path>

    Options:
      -i    ingore missing key
      -k string
            key
      -m string
            output mode (default 0600)
      -o string
            output (default stdout)
      -u string
            output file user name or numeric user id (default: current user)
      -g string
            output file group name or numeric group id (default: current group)


## Command edit

Open an interactive editor for manipulating secrets or creating new secrets.

    Usage: vc edit <secret path>


## Command file

Store or retrieve files.


    Usage: vc file <get|put> <secret path> <file path>

    Options:
      -f	force overwrite
      -i	ignore missing key
      -m string
        	output mode (for put) (default 0600)

In get mode, if the file at path already exists, vc will prompt the user to
overwrite if the terminal is interactive and otherwise throw an error, unless
force overwrite is enabled.

In put mode, if the secret at path already exists, vc will prompt the user to
overwrite if the terminal is interactive and otherwise throw an error, unless
force overwrite is enabled.

The actual secret is stored in base64 encoding, and it will have the magic type
marker (`__TYPE__`) of "file".


## Command ls

List secrets.

    Usage: vc [<options>] ls [<secret path>]

    Options:
      -1	list in compact format
      -R	recursively list subdirectories encountered
      -l	list in long format


## Command mv

Move secrets.

    Usage: vc [<options>] mv <source secret> <target secret>

    Options:
      -f	force overwrite

If the secret at the destination path exists, vc will prompt the user to
overwrite if the terminal is interactive and otherwise throw an error, unless
force overwrite is enabled.


## Command rm

Remove secrets.

    Usage: vc rm <secret path>

    Options:
      -f	force removal


## Command template

Render a template containing Vault secrets. The default render engine is
text/template, see https://golang.org/pkg/text/template/

    Usage: vc template [<options>] <file>

    Options:
      -m string
            output mode (default 0600)
      -o string
            output (default: stdout)
      -t string
            templating mode: html or text (default html)
      -u string
            output file user name or numeric user id (default: current user)
      -g string
            output file group name or numeric group id (default: current group)


The render engine will first evaluate the template file and retrieve all
desired secret paths and keys. Next, it will contact Vault and fetch the
requested secrets. The render engine will report a fatal error if any of the
secrets are missing or if there is an error contacting Vault.

### Function `decode`

Retrieves an encoded secret stored in Vault.

Example:

    We can have any {{decode "secret/test.json"}} type.


### Function `secret`

Allows for looking up secret values stored in Vault. The function expects a
path to a generic secret and a key.

Example:

    The value for key foo at secret/test is: {{secret "secret/test" "foo"}}


# Type key

Only partial support is implemented for the magic `__TYPE__` key which allows
for typed values.

Builtin types:
 * `file` Base64 encoded file in key "contents"
 * `json` Substructure is a key-value dictionary with JSON encoding
 * `yaml` Substructure is a key-value dictionary with YaML encoding
