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
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/spf13/cobra"

	"github.com/nscaledev/unicli/pkg/factory"
	"github.com/nscaledev/unicli/pkg/flags"
	"github.com/nscaledev/unicli/pkg/util"
	computev1 "github.com/unikorn-cloud/compute/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/core/pkg/constants"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type options struct {
	UnikornFlags *factory.UnikornFlags

	organization *flags.OrganizationFlags
	project      *flags.ProjectFlags
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	if err := o.organization.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	if err := o.project.AddFlags(cmd, factory, false); err != nil {
		return err
	}

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
		Use:   "instance [name|id]",
		Short: "Show detailed information about a compute instance",
		Aliases: []string{
			"ci",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one compute instance name or ID must be specified")
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			client, err := factory.Client()
			if err != nil {
				return err
			}

			if err := o.validate(ctx, client); err != nil {
				return err
			}

			if err := o.execute(ctx, client, args[0]); err != nil {
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

func (o *options) execute(ctx context.Context, cli client.Client, identifier string) error {
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

	// Search for the instance across all namespaces
	var instance *computev1.ComputeInstance

	for _, namespace := range namespaces.Items {
		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
			Namespace:     namespace.Name,
		}

		resources := &computev1.ComputeInstanceList{}
		if err := cli.List(ctx, resources, options); err != nil {
			return fmt.Errorf("failed to list compute instances in namespace %s: %w", namespace.Name, err)
		}

		for i := range resources.Items {
			resource := &resources.Items[i]
			// Check both name and ID
			if resource.Labels[constants.NameLabel] == identifier || resource.Name == identifier {
				instance = resource
				break
			}
		}

		if instance != nil {
			break
		}
	}

	if instance == nil {
		return fmt.Errorf("compute instance %s not found", identifier)
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

	// Get organization name
	orgID := instance.Labels[constants.OrganizationLabel]
	orgName := orgNames[orgID]
	if orgName == "" {
		orgName = orgID
	}

	// Get project name
	projID := instance.Labels[constants.ProjectLabel]
	projName := projectNames[projID]
	if projName == "" {
		projName = projID
	}

	// Resolve flavor and image
	flavorID := instance.Spec.FlavorID
	flavorName := flavorNames[flavorID]
	if flavorName == "" {
		flavorName = flavorID
	}

	imageID := instance.Spec.ImageID

	// Define styles
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#1E3A8A"))

	valueStyle := lipgloss.NewStyle()

	// Status styles
	statusSuccessStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#2E7D32")).
		Padding(0, 1)

	statusPendingStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#F57F17")).
		Padding(0, 1)

	statusErrorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#C62828")).
		Padding(0, 1)

	// Build spec tree
	specTree := tree.New().
		Root("Spec").
		Child(fmt.Sprintf("%s%s", labelStyle.Render("Flavor:"), valueStyle.Render(flavorName))).
		Child(fmt.Sprintf("%s%s", labelStyle.Render("Image:"), valueStyle.Render(imageID))).
		Child(fmt.Sprintf("%s%d", labelStyle.Render("Replicas:"), instance.Spec.Replicas))

	if instance.Spec.DiskSize != nil {
		specTree.Child(fmt.Sprintf("%s%s", labelStyle.Render("Disk Size:"), valueStyle.Render(instance.Spec.DiskSize.String())))
	}

	// Build networking tree
	networkTree := tree.New().
		Root("Networking")

	if instance.Spec.Networking != nil {
		networkTree.Child(fmt.Sprintf("%s%t", labelStyle.Render("Public IP:"), instance.Spec.Networking.PublicIP))

		if len(instance.Spec.Networking.SecurityGroupIDs) > 0 {
			sgTree := tree.New().Root("Security Groups")
			for _, sg := range instance.Spec.Networking.SecurityGroupIDs {
				sgTree.Child(valueStyle.Render(sg))
			}
			networkTree.Child(sgTree)
		}

		if len(instance.Spec.Networking.AllowedSourceAddresses) > 0 {
			addrTree := tree.New().Root("Allowed Source Addresses")
			for _, addr := range instance.Spec.Networking.AllowedSourceAddresses {
				addrTree.Child(valueStyle.Render(addr.String()))
			}
			networkTree.Child(addrTree)
		}
	} else {
		networkTree.Child(valueStyle.Render("No networking configured"))
	}

	// Build status tree
	statusTree := tree.New().
		Root("Status")

	if instance.Status.PrivateIP != nil {
		statusTree.Child(fmt.Sprintf("%s%s", labelStyle.Render("Private IP:"), valueStyle.Render(*instance.Status.PrivateIP)))
	}

	if instance.Status.PublicIP != nil {
		statusTree.Child(fmt.Sprintf("%s%s", labelStyle.Render("Public IP:"), valueStyle.Render(*instance.Status.PublicIP)))
	}

	if instance.Status.PowerState != nil {
		statusTree.Child(fmt.Sprintf("%s%s", labelStyle.Render("Power State:"), valueStyle.Render(string(*instance.Status.PowerState))))
	}

	if len(instance.Status.Conditions) > 0 {
		condition := instance.Status.Conditions[0]

		var statusStyle lipgloss.Style

		switch string(condition.Reason) {
		case "Provisioned":
			statusStyle = statusSuccessStyle
		case "Provisioning":
			statusStyle = statusPendingStyle
		default:
			statusStyle = statusErrorStyle
		}
		statusTree.Child(fmt.Sprintf("%s%s", labelStyle.Render("Condition:"), statusStyle.Render(string(condition.Reason))))
	}

	// Create tree
	t := tree.New().
		Root("Compute Instance").
		Child(
			tree.New().
				Root("Basic Information").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(instance.Labels[constants.NameLabel]))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(instance.Name))),
		).
		Child(
			tree.New().
				Root("Organization").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(orgID))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(orgName))),
		).
		Child(
			tree.New().
				Root("Project").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(projID))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(projName))),
		).
		Child(specTree).
		Child(networkTree).
		Child(statusTree)

	// Print the tree
	fmt.Println(t)

	return nil
}

