/*
Copyright 2024-2025 the Unikorn Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openstackidentity

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/errors"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type options struct {
	UnikornFlags *factory.UnikornFlags
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	return nil
}

func (o *options) validate(ctx context.Context, cli client.Client, args []string) error {
	if len(args) > 0 {
		// Validate that the specified identity exists
		_, err := util.GetOpenstackIdentity(ctx, cli, o.UnikornFlags.RegionNamespace, args[0])
		if err != nil {
			return fmt.Errorf("%w: OpenStack identity %s not found", errors.ErrValidation, args[0])
		}
	}

	return nil
}

func (o *options) execute(ctx context.Context, cli client.Client, args []string) error {
	resources := &regionv1.OpenstackIdentityList{}

	if err := cli.List(ctx, resources, &client.ListOptions{Namespace: o.UnikornFlags.RegionNamespace}); err != nil {
		return fmt.Errorf("failed to list OpenStack identities: %w", err)
	}

	// Get cluster name mappings
	clusterNames, err := util.CreateKubernetesClusterNameMap(ctx, cli, "", "")
	if err != nil {
		return fmt.Errorf("failed to get cluster names: %w", err)
	}

	// Create a slice to hold all rows for sorting
	type rowData struct {
		identityID  string
		clusterID   string
		clusterName string
	}
	var rows []rowData

	if len(args) > 0 {
		// Show specific identity
		for _, resource := range resources.Items {
			if resource.Labels[constants.NameLabel] == args[0] {
				clusterName := strings.TrimPrefix(resource.Labels[constants.NameLabel], "kubernetes-cluster-")
				rows = append(rows, rowData{
					identityID:  resource.Name,
					clusterID:   clusterName,
					clusterName: clusterNames[clusterName],
				})
				break
			}
		}
	} else {
		// Show all identities
		for _, resource := range resources.Items {
			clusterName := strings.TrimPrefix(resource.Labels[constants.NameLabel], "kubernetes-cluster-")
			rows = append(rows, rowData{
				identityID:  resource.Name,
				clusterID:   clusterName,
				clusterName: clusterNames[clusterName],
			})
		}
	}

	// Sort rows by OpenStack Identity ID
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].identityID < rows[j].identityID
	})

	// Create table
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#1E3A8A"))).
		Headers("OpenStack Identity ID", "Kubernetes Cluster ID", "Kubernetes Cluster Name").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#FAFAFA")).
					Background(lipgloss.Color("#1E3A8A")).
					Padding(0, 1)
			}
			return lipgloss.NewStyle()
		})

	// Add sorted rows to table
	for _, row := range rows {
		t.Row(
			row.identityID,
			row.clusterID,
			row.clusterName,
		)
	}

	// Print the table
	fmt.Println(t)
	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	o := options{
		UnikornFlags: &factory.UnikornFlags,
	}

	cmd := &cobra.Command{
		Use:     "openstackidentity [name]",
		Aliases: []string{"openstackidentities", "osi"},
		Short:   "Get OpenStack identities",
		Long: `Get OpenStack identities from the region service.

This command lists all OpenStack identities in the region service namespace.
You can optionally specify a name to get information about a specific identity.

Examples:
  # List all OpenStack identities
  kubectl unikorn get openstackidentity

  # Get information about a specific OpenStack identity
  kubectl unikorn get openstackidentity my-identity`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			client, err := factory.Client()
			if err != nil {
				return err
			}

			if err := o.validate(ctx, client, args); err != nil {
				return err
			}

			if err := o.execute(ctx, client, args); err != nil {
				return err
			}

			return nil
		},
	}

	if err := o.AddFlags(cmd, factory); err != nil {
		panic(err)
	}

	return cmd
}
