package main

import (
	"os"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/cli/cmd"
)

func main() {
	approvaltask := cmd.Root()

	if err := approvaltask.Execute(); err != nil {
		os.Exit(1)
	}
}
