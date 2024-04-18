package list

import (
	"github.com/spf13/cobra"
)

func Root() *cobra.Command {

	c := &cobra.Command{
		Use:   "list",
		Short: "List all approval tasks",
		Long:  `This command lists all the approval tasks.`,
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	return c
}
