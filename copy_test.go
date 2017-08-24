package vc

import "testing"

func TestCopyCommand(t *testing.T) {
	for _, test := range []testCommand{
		testCommand{
			Factory: CopyCommandFactory,
			Args:    []string{"--help"},
			Code:    Success,
		},
	} {
		if test.Live {
			if err := testLiveAvailable(); err != nil {
				t.Skip(err)
			}
		}
		testCommandRun(t, test)
	}
}
