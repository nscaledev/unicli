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
	"time"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/flags"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"gopkg.in/yaml.v3"
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
		Use:   "clustermanager [id]",
		Short: "Show detailed information about a cluster manager by ID",
		Aliases: []string{
			"cm",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one cluster manager ID must be specified")
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

func (o *options) execute(ctx context.Context, cli client.Client, id string) error {
	l := labels.Set{}

	if o.organization.Organization != nil {
		l[constants.OrganizationLabel] = o.organization.Organization.Name
	}

	namespaces := &corev1.NamespaceList{}
	if err := cli.List(ctx, namespaces); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Search for the cluster manager across all namespaces
	var manager *kubernetesv1.ClusterManager
	for _, namespace := range namespaces.Items {
		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
			Namespace:     namespace.Name,
		}

		resources := &kubernetesv1.ClusterManagerList{}
		if err := cli.List(ctx, resources, options); err != nil {
			return fmt.Errorf("failed to list cluster managers in namespace %s: %w", namespace.Name, err)
		}

		for i := range resources.Items {
			resource := &resources.Items[i]
			if resource.Name == id {
				manager = resource
				break
			}
		}

		if manager != nil {
			break
		}
	}

	if manager == nil {
		return fmt.Errorf("cluster manager %s not found", id)
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

	// Get organization name
	orgID := manager.Labels[constants.OrganizationLabel]
	orgName := orgNames[orgID]
	if orgName == "" {
		orgName = orgID
	}

	// Get associated cluster names
	clusters := clusterNames[manager.Name]

	detail := map[string]interface{}{
		"name": manager.Labels[constants.NameLabel],
		"id":   manager.Name,
		"organization": map[string]string{
			"id":   orgID,
			"name": orgName,
		},
		"clusters":  clusters,
		"namespace": manager.Namespace,
		"spec":      manager.Spec,
		"status":    manager.Status,
	}

	data, err := yaml.Marshal(detail)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster manager %s: %w", id, err)
	}

	fmt.Println(string(data))
	return nil
}
