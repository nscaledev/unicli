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

package kubernetescluster

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/nscaledev/unicli/pkg/factory"
	"github.com/nscaledev/unicli/pkg/flags"
	"github.com/unikorn-cloud/core/pkg/constants"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nscaledev/unicli/pkg/util"
)

// allColumns defines every available column name.
var allColumns = []string{"name", "id", "version", "status", "organization", "project", "region"}

// defaultColumns is the set shown when --columns is not specified.
var defaultColumns = []string{"name", "version", "status", "organization", "project", "region"}

type options struct {
	UnikornFlags *factory.UnikornFlags

	organization *flags.OrganizationFlags
	project      *flags.ProjectFlags
	region       *flags.RegionFlags
	columns      []string
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	if err := o.organization.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	if err := o.project.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	if err := o.region.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	cmd.Flags().StringSliceVar(&o.columns, "columns", defaultColumns,
		fmt.Sprintf("Comma-separated list of columns to display. Available: %s", strings.Join(allColumns, ", ")))

	return nil
}

func (o *options) validate(ctx context.Context, cli client.Client) error {
	validators := []func(context.Context, client.Client) error{
		o.organization.Validate,
		o.project.Validate,
		o.region.Validate,
	}

	for _, validator := range validators {
		if err := validator(ctx, cli); err != nil {
			return err
		}
	}

	for _, col := range o.columns {
		if !slices.Contains(allColumns, strings.ToLower(col)) {
			return fmt.Errorf("unknown column %q, available columns: %s", col, strings.Join(allColumns, ", "))
		}
	}

	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	unikornFlags := &factory.UnikornFlags
	organizationFlags := flags.NewOrganizationFlags(unikornFlags)
	projectFlags := flags.NewProjectFlags(unikornFlags, organizationFlags)
	regionFlags := flags.NewRegionFlags(unikornFlags)

	o := options{
		UnikornFlags: unikornFlags,
		organization: organizationFlags,
		project:      projectFlags,
		region:       regionFlags,
	}

	cmd := &cobra.Command{
		Use:   "kubernetescluster [name]",
		Short: "Get kubernetes clusters",
		Aliases: []string{
			"kubernetesclusters",
			"kc",
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
func getClusterDetails(cluster *kubernetesv1.KubernetesCluster, orgNames, projectNames, regionNames map[string]string) map[string]interface{} {
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

	return map[string]any{
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
		"version":        cluster.Spec.Version.String(),
		"spec":           cluster.Spec,
		"clustermanager": cluster.Spec.ClusterManagerID,
		"status":         cluster.Status,
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
	var allClusters []kubernetesv1.KubernetesCluster

	for _, namespace := range namespaces.Items {
		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
			Namespace:     namespace.Name,
		}

		resources := &kubernetesv1.KubernetesClusterList{}
		if err := cli.List(ctx, resources, options); err != nil {
			return fmt.Errorf("failed to list clusters in namespace %s: %w", namespace.Name, err)
		}

		allClusters = append(allClusters, resources.Items...)
	}

	// Filter by region if specified
	if o.region.Region != nil {
		filtered := allClusters[:0]
		for _, cluster := range allClusters {
			if cluster.Spec.RegionID == o.region.Region.Name {
				filtered = append(filtered, cluster)
			}
		}
		allClusters = filtered
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

	// Build headers from selected columns
	headerMap := map[string]string{
		"name":         "Name",
		"id":           "ID",
		"version":      "Version",
		"status":       "Status",
		"organization": "Organization",
		"project":      "Project",
		"region":       "Region",
	}

	headers := make([]string, 0, len(o.columns))
	for _, col := range o.columns {
		headers = append(headers, headerMap[col])
	}

	// Create table
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#1E3A8A"))).
		Headers(headers...).
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

	// Add rows
	for i := range allClusters {
		resource := &allClusters[i]
		detail := getClusterDetails(resource, orgNames, projectNames, regionNames)

		// Extract status reason
		status := detail["status"].(kubernetesv1.KubernetesClusterStatus)
		statusReason := ""
		if len(status.Conditions) > 0 {
			statusReason = string(status.Conditions[0].Reason)
		}

		valueMap := map[string]string{
			"name":         fmt.Sprintf("%v", detail["name"]),
			"id":           resource.Name,
			"version":      fmt.Sprintf("%v", detail["version"]),
			"status":       statusReason,
			"organization": detail["organization"].(map[string]string)["name"],
			"project":      detail["project"].(map[string]string)["name"],
			"region":       detail["region"].(map[string]string)["name"],
		}

		var row []string
		for _, col := range o.columns {
			row = append(row, valueMap[col])
		}

		t.Row(row...)
	}

	// Print the table
	fmt.Println(t)
	return nil
}
