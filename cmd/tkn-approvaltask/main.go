package main

import (
	"os"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/cli"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/cli/cmd"
)

func main() {
	tp := &cli.ApprovalTaskParams{}
	approvaltask := cmd.Root(tp)

	if err := approvaltask.Execute(); err != nil {
		os.Exit(1)
	}
}
