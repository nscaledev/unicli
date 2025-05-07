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

package clustermanager

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/flags"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/printers"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"
)

type options struct {
	UnikornFlags *factory.UnikornFlags

	organization *flags.OrganizationFlags
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	if err := o.organization.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	return nil
}

func (o *options) validate(ctx context.Context, cli client.Client) error {
	validators := []func(context.Context, client.Client) error{
		o.organization.Validate,
	}

	for _, validator := range validators {
		if err := validator(ctx, cli); err != nil {
			return err
		}
	}

	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	unikornFlags := &factory.UnikornFlags
	organizationFlags := flags.NewOrganizationFlags(unikornFlags)

	o := options{
		UnikornFlags: unikornFlags,
		organization: organizationFlags,
	}

	cmd := &cobra.Command{
		Use:   "clustermanager [name]",
		Short: "Get kubernetes cluster managers",
		Aliases: []string{
			"clustermanagers",
			"cm",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			client, err := factory.Client()
			if err != nil {
				return err
			}

			if err := o.validate(ctx, client); err != nil {
				return err
			}

			if err := o.execute(ctx, client); err != nil {
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

func (o *options) execute(ctx context.Context, cli client.Client) error {
	l := labels.Set{}

	if o.organization.Organization != nil {
		l[constants.OrganizationLabel] = o.organization.Organization.Name
	}

	namespaces := &corev1.NamespaceList{}
	if err := cli.List(ctx, namespaces); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Collect all cluster managers across namespaces
	var allManagers []kubernetesv1.ClusterManager

	for _, namespace := range namespaces.Items {
		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
			Namespace:     namespace.Name,
		}

		resources := &kubernetesv1.ClusterManagerList{}
		if err := cli.List(ctx, resources, options); err != nil {
			return fmt.Errorf("failed to list cluster managers in namespace %s: %w", namespace.Name, err)
		}

		allManagers = append(allManagers, resources.Items...)
	}

	// Create maps for ID to name lookups
	orgNames, err := util.CreateOrganizationNameMap(ctx, cli, o.UnikornFlags.IdentityNamespace)
	if err != nil {
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	// Get all KubernetesClusters to count associated clusters
	allClusters := &kubernetesv1.KubernetesClusterList{}
	if err := cli.List(ctx, allClusters); err != nil {
		return fmt.Errorf("failed to list kubernetes clusters: %w", err)
	}

	// Create a map of clustermanager IDs to cluster names
	clusterNames := make(map[string][]string)
	for _, cluster := range allClusters.Items {
		clusterNames[cluster.Spec.ClusterManagerID] = append(clusterNames[cluster.Spec.ClusterManagerID], cluster.Labels[constants.NameLabel])
	}

	// Create table for normal view
	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "name"},
			{Name: "id"},
			{Name: "organization"},
			{Name: "clusters"},
			{Name: "namespace"},
			{Name: "status"},
		},
		Rows: make([]metav1.TableRow, 0, len(allManagers)),
	}

	for i := range allManagers {
		resource := &allManagers[i]

		// Get organization name
		orgID := resource.Labels[constants.OrganizationLabel]
		orgName := orgNames[orgID]
		if orgName == "" {
			orgName = orgID
		}

		// Get status reason
		statusReason := ""
		if len(resource.Status.Conditions) > 0 {
			statusReason = string(resource.Status.Conditions[0].Reason)
		}

		// Get associated cluster names
		clusters := clusterNames[resource.Name]
		clusterList := ""
		if len(clusters) > 0 {
			clusterList = strings.Join(clusters, ", ")
		}

		row := metav1.TableRow{
			Cells: []any{
				resource.Labels[constants.NameLabel],
				resource.Name,
				orgName,
				clusterList,
				resource.Namespace,
				statusReason,
			},
		}

		table.Rows = append(table.Rows, row)
	}

	printer := printers.NewTablePrinter(printers.PrintOptions{
		Wide:      true,
		NoHeaders: false,
	})

	return printer.PrintObj(table, os.Stdout)
}
