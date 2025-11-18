package cli

import (
	"fmt"
	"os"
	"testing"

	"gotest.tools/v3/icmd"
)

type TknApprovalTaskRunner struct {
	path      string
	namespace string
}

func NewTknApprovalTaskRunner() (TknApprovalTaskRunner, error) {
	if os.Getenv("TEST_CLIENT_BINARY") != "" {
		return TknApprovalTaskRunner{
			path: os.Getenv("TEST_CLIENT_BINARY"),
		}, nil
	}
	return TknApprovalTaskRunner{
		path: os.Getenv("TEST_CLIENT_BINARY"),
	}, fmt.Errorf("Error: couldn't Create tknApprovalTaskRunner, please do check tkn binary path: (%+v)", os.Getenv("TEST_CLIENT_BINARY"))
}

func (tknApprovalTaskRunner TknApprovalTaskRunner) Run(args ...string) *icmd.Result {
	cmd := append([]string{tknApprovalTaskRunner.path}, args...)
	return icmd.RunCmd(icmd.Cmd{Command: cmd})
}

// MustSucceed asserts that the command ran with 0 exit code
func (tknApprovalTaskRunner TknApprovalTaskRunner) MustSucceed(t *testing.T, args ...string) *icmd.Result {
	return tknApprovalTaskRunner.Assert(t, icmd.Success, args...)
}

// Assert runs a command and verifies exit code (0)
func (tknApprovalTaskRunner TknApprovalTaskRunner) Assert(t *testing.T, exp icmd.Expected, args ...string) *icmd.Result {
	res := tknApprovalTaskRunner.Run(args...)
	res.Assert(t, exp)
	return res
}
