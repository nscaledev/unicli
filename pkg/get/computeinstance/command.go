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

package computeinstance

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
	"github.com/nscaledev/unicli/pkg/util"
	computev1 "github.com/unikorn-cloud/compute/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/core/pkg/constants"
	regionconstants "github.com/unikorn-cloud/region/pkg/constants"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// allColumns defines every available column name.
var allColumns = []string{"name", "id", "flavor", "image", "status", "organization", "project", "region"}

// defaultColumns is the set shown when --columns is not specified.
var defaultColumns = []string{"name", "flavor", "status", "organization", "project", "region"}

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
		Use:   "instance [name]",
		Short: "Get compute instances",
		Aliases: []string{
			"instances",
			"ci",
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

// buildFlavorNameMap builds a map of flavor UUID to human-readable description
// from Region CRD flavor metadata.
func buildFlavorNameMap(regions *regionv1.RegionList) map[string]string {
	flavorNames := make(map[string]string)

	for i := range regions.Items {
		region := &regions.Items[i]

		if region.Spec.Openstack == nil || region.Spec.Openstack.Compute == nil ||
			region.Spec.Openstack.Compute.Flavors == nil {
			continue
		}

		for _, fm := range region.Spec.Openstack.Compute.Flavors.Metadata {
			if _, exists := flavorNames[fm.ID]; exists {
				continue
			}

			flavorNames[fm.ID] = formatFlavorDescription(fm)
		}
	}

	return flavorNames
}

func formatFlavorDescription(fm regionv1.FlavorMetadata) string {
	desc := ""

	if fm.CPU != nil && fm.CPU.Count != nil {
		desc += fmt.Sprintf("%d CPUs", *fm.CPU.Count)
	}

	if fm.Memory != nil {
		if desc != "" {
			desc += ", "
		}

		desc += fm.Memory.String()
	}

	if fm.GPU != nil {
		if desc != "" {
			desc += ", "
		}

		desc += fmt.Sprintf("%dx %s %s", fm.GPU.PhysicalCount, fm.GPU.Vendor, fm.GPU.Model)
	}

	if desc == "" {
		return fm.ID
	}

	return desc
}

func (o *options) execute(ctx context.Context, cli client.Client, args []string) error {
	l := labels.Set{}

	if o.organization.Organization != nil {
		l[constants.OrganizationLabel] = o.organization.Organization.Name
	}

	if o.project.Project != nil {
		l[constants.ProjectLabel] = o.project.Project.Name
	}

	if o.region.Region != nil {
		l[regionconstants.RegionLabel] = o.region.Region.Name
	}

	namespaces := &corev1.NamespaceList{}
	if err := cli.List(ctx, namespaces); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Collect all instances across namespaces
	var allInstances []computev1.ComputeInstance

	for _, namespace := range namespaces.Items {
		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
			Namespace:     namespace.Name,
		}

		resources := &computev1.ComputeInstanceList{}
		if err := cli.List(ctx, resources, options); err != nil {
			return fmt.Errorf("failed to list compute instances in namespace %s: %w", namespace.Name, err)
		}

		allInstances = append(allInstances, resources.Items...)
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

	flavorNames := buildFlavorNameMap(regions)

	// Build region name map (region ID -> display name)
	regionNames := make(map[string]string)
	for _, region := range regions.Items {
		regionNames[region.Name] = region.Labels[constants.NameLabel]
	}

	// Build headers from selected columns
	headerMap := map[string]string{
		"name":         "Name",
		"id":           "ID",
		"flavor":       "Flavor",
		"image":        "Image",
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
	for i := range allInstances {
		resource := &allInstances[i]

		name := resource.Labels[constants.NameLabel]

		orgID := resource.Labels[constants.OrganizationLabel]
		orgName := orgNames[orgID]
		if orgName == "" {
			orgName = orgID
		}

		projID := resource.Labels[constants.ProjectLabel]
		projName := projectNames[projID]
		if projName == "" {
			projName = projID
		}

		flavorID := resource.Spec.FlavorID
		flavorName := flavorNames[flavorID]
		if flavorName == "" {
			flavorName = flavorID
		}

		imageID := resource.Spec.ImageID

		statusReason := ""
		if len(resource.Status.Conditions) > 0 {
			statusReason = string(resource.Status.Conditions[0].Reason)
		}

		// Resolve region from the instance's region label
		regionID := resource.Labels[regionconstants.RegionLabel]
		regionName := regionNames[regionID]
		if regionName == "" {
			regionName = regionID
		}

		// Build row values in column order
		valueMap := map[string]string{
			"name":         name,
			"id":           resource.Name,
			"flavor":       flavorName,
			"image":        imageID,
			"status":       statusReason,
			"organization": orgName,
			"project":      projName,
			"region":       regionName,
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
