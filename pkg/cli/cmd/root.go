package cmd

import (
	"github.com/openshift-pipelines/manual-approval-gate/pkg/cli/cmd/list"
	"github.com/spf13/cobra"
)

func Root() *cobra.Command {
	c := &cobra.Command{
		Use:   "tkn-approvaltask",
		Short: "tkn approvaltask is a CLI tool for managing approvalTask",
		Long:  `This application is a CLI tool to manage approvalTask`,
	}

	c.AddCommand(list.Root())

	return c
}
