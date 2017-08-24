package vc

import "testing"

func TestDeleteCommand(t *testing.T) {
	for _, test := range []testCommand{
		testCommand{
			Factory: DeleteCommandFactory,
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
