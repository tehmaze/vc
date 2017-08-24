package vc

import (
	"os"
	"testing"
)

func TestIsTerminal(_ *testing.T) {
	_ = IsTerminal(os.Stdout.Fd())
	_ = IsTerminal(os.Stderr.Fd())
}
