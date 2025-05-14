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
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
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
		Use:   "virtualkubernetescluster [name]",
		Short: "Show detailed information about a virtual kubernetes cluster",
		Aliases: []string{
			"vkc",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one virtual kubernetes cluster name must be specified")
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

	// Search for the cluster across all namespaces
	var cluster *kubernetesv1.VirtualKubernetesCluster
	for _, namespace := range namespaces.Items {
		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
			Namespace:     namespace.Name,
		}

		resources := &kubernetesv1.VirtualKubernetesClusterList{}
		if err := cli.List(ctx, resources, options); err != nil {
			return fmt.Errorf("failed to list clusters in namespace %s: %w", namespace.Name, err)
		}

		for i := range resources.Items {
			resource := &resources.Items[i]
			if resource.Labels[constants.NameLabel] == identifier || resource.Name == identifier {
				cluster = resource
				break
			}
		}

		if cluster != nil {
			break
		}
	}

	if cluster == nil {
		return fmt.Errorf("virtual kubernetes cluster %s not found", identifier)
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

	// Get organization name
	orgID := cluster.Labels[constants.OrganizationLabel]
	orgName := orgNames[orgID]
	if orgName == "" {
		orgName = orgID
	}

	// Get project name
	projID := cluster.Labels[constants.ProjectLabel]
	projName := projectNames[projID]
	if projName == "" {
		projName = projID
	}

	// Get region name
	regionID := cluster.Spec.RegionID
	regionName := regionNames[regionID]
	if regionName == "" {
		regionName = regionID
	}

	detail := map[string]any{
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
		"spec":          cluster.Spec,
		"workloadpools": cluster.Spec.WorkloadPools,
		"status":        cluster.Status,
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
		Background(lipgloss.Color("#2E7D32")). // Green
		Padding(0, 1)

	statusPendingStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#F57F17")). // Amber
		Padding(0, 1)

	statusErrorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#C62828")). // Red
		Padding(0, 1)

	// Create tree
	t := tree.New().
		Root("Virtual Kubernetes Cluster").
		Child(
			tree.New().
				Root("Basic Information").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(detail["name"].(string)))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(cluster.Name))),
		).
		Child(
			tree.New().
				Root("Organization").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(detail["organization"].(map[string]string)["id"]))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(detail["organization"].(map[string]string)["name"]))),
		).
		Child(
			tree.New().
				Root("Project").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(detail["project"].(map[string]string)["id"]))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(detail["project"].(map[string]string)["name"]))),
		).
		Child(
			tree.New().
				Root("Region").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(detail["region"].(map[string]string)["id"]))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(detail["region"].(map[string]string)["name"]))),
		)

	// Add Workload Pools
	workloadPoolsTree := tree.New().
		Root("Workload Pools")
	workloadPools := cluster.Spec.WorkloadPools
	if len(workloadPools) > 0 {
		for _, pool := range workloadPools {
			poolTree := tree.New().
				Root(fmt.Sprintf("%s%s", labelStyle.Render("Pool:"), valueStyle.Render(pool.Name))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Flavor ID:"), valueStyle.Render(pool.FlavorID))).
				Child(fmt.Sprintf("%s%d", labelStyle.Render("Replicas:"), pool.Replicas))
			workloadPoolsTree.Child(poolTree)
		}
	} else {
		workloadPoolsTree.Child(valueStyle.Render("No workload pools configured"))
	}
	t.Child(workloadPoolsTree)

	// Add Status Information
	statusTree := tree.New().
		Root("Status")
	status := detail["status"].(kubernetesv1.VirtualKubernetesClusterStatus)
	if len(status.Conditions) > 0 {
		condition := status.Conditions[0]
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
	t.Child(statusTree)

	// Print the tree
	fmt.Println(t)
	return nil
}
