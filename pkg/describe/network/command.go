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

package network

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
		Use:   "network [name|id]",
		Short: "Show detailed information about a network",
		Aliases: []string{
			"net",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one network name or ID must be specified")
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

	// Search for the network across all namespaces
	var network *regionv1.Network

	for _, namespace := range namespaces.Items {
		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
			Namespace:     namespace.Name,
		}

		resources := &regionv1.NetworkList{}
		if err := cli.List(ctx, resources, options); err != nil {
			return fmt.Errorf("failed to list networks in namespace %s: %w", namespace.Name, err)
		}

		for i := range resources.Items {
			resource := &resources.Items[i]
			// Check both name and ID
			if resource.Labels[constants.NameLabel] == identifier || resource.Name == identifier {
				network = resource
				break
			}
		}

		if network != nil {
			break
		}
	}

	if network == nil {
		return fmt.Errorf("network %s not found", identifier)
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

	// Get organization name
	orgID := network.Labels[constants.OrganizationLabel]
	orgName := orgNames[orgID]
	if orgName == "" {
		orgName = orgID
	}

	// Get project name
	projID := network.Labels[constants.ProjectLabel]
	projName := projectNames[projID]
	if projName == "" {
		projName = projID
	}

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
		Child(fmt.Sprintf("%s%s", labelStyle.Render("Provider:"), valueStyle.Render(string(network.Spec.Provider))))

	if network.Spec.Prefix != nil {
		specTree.Child(fmt.Sprintf("%s%s", labelStyle.Render("Prefix:"), valueStyle.Render(network.Spec.Prefix.String())))
	}

	if len(network.Spec.DNSNameservers) > 0 {
		dnsTree := tree.New().Root("DNS Nameservers")
		for _, ns := range network.Spec.DNSNameservers {
			dnsTree.Child(valueStyle.Render(ns.String()))
		}
		specTree.Child(dnsTree)
	}

	if len(network.Spec.Routes) > 0 {
		routesTree := tree.New().Root("Routes")
		for _, route := range network.Spec.Routes {
			routesTree.Child(fmt.Sprintf("%s%s → %s%s",
				labelStyle.Render("Prefix:"), valueStyle.Render(route.Prefix.String()),
				labelStyle.Render("NextHop:"), valueStyle.Render(route.NextHop.String())))
		}
		specTree.Child(routesTree)
	}

	// Build status tree
	statusTree := tree.New().
		Root("Status")

	if network.Status.Openstack != nil {
		if network.Status.Openstack.NetworkID != nil {
			statusTree.Child(fmt.Sprintf("%s%s", labelStyle.Render("Network ID:"), valueStyle.Render(*network.Status.Openstack.NetworkID)))
		}

		if network.Status.Openstack.SubnetID != nil {
			statusTree.Child(fmt.Sprintf("%s%s", labelStyle.Render("Subnet ID:"), valueStyle.Render(*network.Status.Openstack.SubnetID)))
		}

		if network.Status.Openstack.VlanID != nil {
			statusTree.Child(fmt.Sprintf("%s%d", labelStyle.Render("VLAN ID:"), *network.Status.Openstack.VlanID))
		}
	}

	if len(network.Status.Conditions) > 0 {
		condition := network.Status.Conditions[0]

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
		Root("Network").
		Child(
			tree.New().
				Root("Basic Information").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(network.Labels[constants.NameLabel]))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(network.Name))),
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
		Child(statusTree)

	// Print the tree
	fmt.Println(t)

	return nil
}
