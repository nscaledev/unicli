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
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/errors"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type options struct {
	UnikornFlags *factory.UnikornFlags
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	return nil
}

func (o *options) validate(ctx context.Context, cli client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: OpenStack identity name is required", errors.ErrValidation)
	}
	return nil
}

func (o *options) execute(ctx context.Context, cli client.Client, args []string) error {
	// Get the OpenStack identity
	identity, err := util.GetOpenstackIdentity(ctx, cli, o.UnikornFlags.RegionNamespace, args[0])
	if err != nil {
		return fmt.Errorf("failed to get OpenStack identity: %w", err)
	}

	// Get cluster name mapping
	clusterNames, err := util.CreateKubernetesClusterNameMap(ctx, cli, "", "")
	if err != nil {
		return fmt.Errorf("failed to get cluster names: %w", err)
	}

	// Extract cluster ID from the label
	clusterID := strings.TrimPrefix(identity.Labels[constants.NameLabel], "kubernetes-cluster-")
	clusterName := clusterNames[clusterID]

	// Define styles
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#1E3A8A"))

	valueStyle := lipgloss.NewStyle()

	// Create tree
	t := tree.New().
		Root("OpenStack Identity").
		Child(
			tree.New().
				Root("Basic Information").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("OpenStack Identity ID:"), valueStyle.Render(identity.Name))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("OpenStack Project ID:"), valueStyle.Render(*identity.Spec.ProjectID))),
		).
		Child(
			tree.New().
				Root("Kubernetes Cluster").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("ID:"), valueStyle.Render(clusterID))).
				Child(fmt.Sprintf("%s%s", labelStyle.Render("Name:"), valueStyle.Render(clusterName))),
		).
		Child(
			tree.New().
				Root("Region").
				Child(fmt.Sprintf("%s%s", labelStyle.Render("NKS Region ID:"), valueStyle.Render(identity.Labels[constants.OrganizationLabel]))),
		)

	// Print the tree
	fmt.Println(t)
	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	o := options{
		UnikornFlags: &factory.UnikornFlags,
	}

	cmd := &cobra.Command{
		Use:   "openstackidentity [name]",
		Short: "Describe an OpenStack identity",
		Long: `Describe an OpenStack identity from the region service.

This command shows detailed information about an OpenStack identity, including its ID,
associated Kubernetes cluster information, and project ID.

Example:
  # Describe a specific OpenStack identity
  kubectl unikorn describe openstackidentity my-identity`,
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
