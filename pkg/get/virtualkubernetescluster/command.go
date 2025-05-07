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

package virtualkubernetescluster

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/flags"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/printers"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"
)

type options struct {
	UnikornFlags *factory.UnikornFlags

	organization *flags.OrganizationFlags
	project      *flags.ProjectFlags
	detail       bool
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	if err := o.organization.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	if err := o.project.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	cmd.Flags().BoolVar(&o.detail, "detail", false, "Show detailed information about the virtual clusters")

	return nil
}

func (o *options) validate(ctx context.Context, cli client.Client) error {
	validators := []func(context.Context, client.Client) error{
		o.organization.Validate,
		o.project.Validate,
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
	projectFlags := flags.NewProjectFlags(unikornFlags, organizationFlags)

	o := options{
		UnikornFlags: unikornFlags,
		organization: organizationFlags,
		project:      projectFlags,
	}

	cmd := &cobra.Command{
		Use:   "virtualkubernetescluster [name]",
		Short: "Get virtual kubernetes clusters",
		Aliases: []string{
			"virtualkubernetesclusters",
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

// Get cluster details in a sane format
func getClusterDetails(cluster *kubernetesv1.VirtualKubernetesCluster, orgNames, projectNames, regionNames map[string]string) map[string]interface{} {
	orgID := cluster.Labels[constants.OrganizationLabel]
	orgName := orgNames[orgID]
	if orgName == "" {
		orgName = orgID
	}

	projID := cluster.Labels[constants.ProjectLabel]
	projName := projectNames[projID]
	if projName == "" {
		projName = projID
	}

	regionID := cluster.Spec.RegionID
	regionName := regionNames[regionID]
	if regionName == "" {
		regionName = regionID
	}

	return map[string]interface{}{
		"name": cluster.Labels[constants.NameLabel],
		"organization": map[string]string{
			"id":   orgID,
			"name": orgName,
		},
		"project": map[string]string{
			"id":   projID,
			"name": projName,
		},
		"region": map[string]string{
			"id":   regionID,
			"name": regionName,
		},
		"spec":   cluster.Spec,
		"status": cluster.Status,
	}
}

func (o *options) execute(ctx context.Context, cli client.Client, args []string) error {
	l := labels.Set{}

	if o.organization.Organization != nil {
		l[constants.OrganizationLabel] = o.organization.Organization.Name
	}

	if o.project.Project != nil {
		l[constants.ProjectLabel] = o.project.Project.Name
	}

	namespaces := &corev1.NamespaceList{}
	if err := cli.List(ctx, namespaces); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Collect all clusters across namespaces
	var allClusters []kubernetesv1.VirtualKubernetesCluster

	for _, namespace := range namespaces.Items {
		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
			Namespace:     namespace.Name,
		}

		resources := &kubernetesv1.VirtualKubernetesClusterList{}
		if err := cli.List(ctx, resources, options); err != nil {
			return fmt.Errorf("failed to list clusters in namespace %s: %w", namespace.Name, err)
		}

		allClusters = append(allClusters, resources.Items...)
	}

	// Create maps for ID to name lookups
	orgNames, err := util.CreateOrganizationNameMap(ctx, cli, o.UnikornFlags.IdentityNamespace)
	if err != nil {
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	projectNames, err := util.CreateProjectNameMap(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	regions := &regionv1.RegionList{}
	if err := cli.List(ctx, regions, &client.ListOptions{Namespace: o.UnikornFlags.RegionNamespace}); err != nil {
		return fmt.Errorf("failed to list regions: %w", err)
	}
	regionNames := make(map[string]string)
	for _, region := range regions.Items {
		regionNames[region.Name] = region.Labels[constants.NameLabel]
	}

	// If detail flag is set, print YAML for specified or all clusters
	if o.detail {
		// Create a map of cluster names to clusters for easy lookup
		clusterMap := make(map[string]*kubernetesv1.VirtualKubernetesCluster)
		for i := range allClusters {
			cluster := &allClusters[i]
			clusterMap[cluster.Labels[constants.NameLabel]] = cluster
		}

		// If a cluster name was provided, only show that one
		if len(args) > 0 {
			clusterName := args[0]
			cluster, exists := clusterMap[clusterName]
			if !exists {
				return fmt.Errorf("cluster %s not found", clusterName)
			}

			detail := getClusterDetails(cluster, orgNames, projectNames, regionNames)
			data, err := yaml.Marshal(detail)
			if err != nil {
				return fmt.Errorf("failed to marshal cluster %s: %w", clusterName, err)
			}
			fmt.Println(string(data))
			return nil
		}

		// Otherwise show all clusters
		for _, cluster := range allClusters {
			detail := getClusterDetails(&cluster, orgNames, projectNames, regionNames)
			data, err := yaml.Marshal(detail)
			if err != nil {
				return fmt.Errorf("failed to marshal cluster %s: %w", cluster.Name, err)
			}
			fmt.Printf("---\n%s\n", string(data))
		}
		return nil
	}

	// Create table for normal view
	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "name"},
			{Name: "organization"},
			{Name: "project"},
			{Name: "region"},
			{Name: "status"},
		},
		Rows: make([]metav1.TableRow, 0, len(allClusters)),
	}

	for i := range allClusters {
		resource := &allClusters[i]
		detail := getClusterDetails(resource, orgNames, projectNames, regionNames)

		// Extract status reason
		status := detail["status"].(kubernetesv1.VirtualKubernetesClusterStatus)
		statusReason := ""
		if len(status.Conditions) > 0 {
			statusReason = string(status.Conditions[0].Reason)
		}

		row := metav1.TableRow{
			Cells: []any{
				detail["name"],
				detail["organization"].(map[string]string)["name"],
				detail["project"].(map[string]string)["name"],
				detail["region"].(map[string]string)["name"],
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
