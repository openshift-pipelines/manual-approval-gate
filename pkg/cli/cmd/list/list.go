package list

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/fatih/color"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	cli "github.com/openshift-pipelines/manual-approval-gate/pkg/cli"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ListOptions struct {
	AllNamespaces bool
}

var ConditionColor = map[string]color.Attribute{
	"Rejected": color.FgHiRed,
	"Approved": color.FgHiGreen,
	"Pending":  color.FgHiBlue,
}

const listTemplate = `NAME	NumberOfApprovalsRequired	PendingApprovals	Rejected	STATUS{{range .ApprovalTasks.Items}}
{{.Name}}	{{.Spec.NumberOfApprovalsRequired}}	{{pendingApprovals .}}	{{rejected .}}	{{state .}}{{end}}
`

func pendingApprovals(at *v1alpha1.ApprovalTask) int {
	return at.Spec.NumberOfApprovalsRequired - len(at.Status.ApproversResponse)
}

func rejected(at *v1alpha1.ApprovalTask) int {
	count := 0
	for _, approver := range at.Status.ApproversResponse {
		if approver.Response == "rejected" {
			count = count + 1
		}
	}
	return count
}

func ColorStatus(status string) string {
	return color.New(ConditionColor[status]).Sprint(status)
}

func state(at *v1alpha1.ApprovalTask) string {
	var state string

	switch at.Status.State {
	case "approved":
		state = "Approved"
	case "rejected":
		state = "Rejected"
	case "pending":
		state = "Pending"
	}
	return ColorStatus(state)
}

func Root(p cli.Params) *cobra.Command {
	opts := &ListOptions{}
	funcMap := template.FuncMap{
		"pendingApprovals": pendingApprovals,
		"state":            state,
		"rejected":         rejected,
	}

	c := &cobra.Command{
		Use:   "list",
		Short: "List all approval tasks",
		Long:  `This command lists all the approval tasks.`,
		Annotations: map[string]string{
			"commandType": "main",
		},
		Run: func(cmd *cobra.Command, args []string) {
			cs, err := p.Clients()
			if err != nil {
				fmt.Println("Error getting clients:", err)
				return
			}

			ns := p.Namespace()
			if opts.AllNamespaces {
				ns = ""
			}

			at, err := cs.ApprovalTask.ApprovalTasks(ns).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				fmt.Println("Failed to list the approval tasks:", err)
				return
			}

			var data = struct {
				ApprovalTasks *v1alpha1.ApprovalTaskList
			}{
				ApprovalTasks: at,
			}

			t, err := template.New("List Approvaltask").Funcs(funcMap).Parse(listTemplate)

			w := tabwriter.NewWriter(os.Stdout, 8, 8, 8, ' ', 0)
			if err := t.Execute(w, data); err != nil {
				log.Fatal(err)
			}
			w.Flush()

		},
	}

	c.Flags().BoolVarP(&opts.AllNamespaces, "all-namespaces", "A", opts.AllNamespaces, "list Tasks from all namespaces")

	return c
}
