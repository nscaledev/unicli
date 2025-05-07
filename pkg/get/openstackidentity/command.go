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
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/errors"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"

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

	// Create table for output
	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{
				Name:        "OpenStack Identity ID",
				Type:        "string",
				Description: "The unique identifier of the OpenStack identity",
				Priority:    0,
			},
			{
				Name:        "Kubernetes Cluster ID",
				Type:        "string",
				Description: "The associated Kubernetes cluster identifier",
				Priority:    0,
			},
			{
				Name:        "Kubernetes Cluster Name",
				Type:        "string",
				Description: "The display name of the Kubernetes cluster",
				Priority:    0,
			},
		},
		Rows: make([]metav1.TableRow, 0, len(resources.Items)),
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

	// Add sorted rows to table
	for _, row := range rows {
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells: []any{
				row.identityID,
				row.clusterID,
				row.clusterName,
			},
		})
	}

	printer := printers.NewTablePrinter(printers.PrintOptions{
		Wide:      true,
		NoHeaders: false,
	})

	return printer.PrintObj(table, os.Stdout)
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
