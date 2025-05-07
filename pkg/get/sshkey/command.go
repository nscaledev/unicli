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

package sshkey

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/errors"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type options struct {
	UnikornFlags *factory.UnikornFlags
	clusterID    string
	clusterName  string
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	cmd.Flags().StringVar(&o.clusterID, "cluster", "", "Kubernetes cluster ID")
	cmd.Flags().StringVar(&o.clusterName, "name", "", "Kubernetes cluster name")
	return nil
}

func (o *options) validate(ctx context.Context, cli client.Client) error {
	if o.clusterID == "" && o.clusterName == "" {
		return fmt.Errorf("%w: either cluster ID or name must be provided", errors.ErrValidation)
	}
	if o.clusterID != "" && o.clusterName != "" {
		return fmt.Errorf("%w: cannot specify both cluster ID and name", errors.ErrValidation)
	}
	return nil
}

func (o *options) execute(ctx context.Context, cli client.Client) error {
	// If we have a cluster name, convert it to an ID
	if o.clusterName != "" {
		clusterNames, err := util.CreateKubernetesClusterNameMap(ctx, cli, "", "")
		if err != nil {
			return fmt.Errorf("failed to get cluster names: %w", err)
		}

		// Find the cluster ID that matches the name
		found := false
		for id, name := range clusterNames {
			if name == o.clusterName {
				o.clusterID = id
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("%w: no cluster found with name %s", errors.ErrValidation, o.clusterName)
		}
	}

	// List all OpenStack identities
	resources := &regionv1.OpenstackIdentityList{}
	if err := cli.List(ctx, resources, &client.ListOptions{Namespace: o.UnikornFlags.RegionNamespace}); err != nil {
		return fmt.Errorf("failed to list OpenStack identities: %w", err)
	}

	// Find the identity that matches the cluster ID
	var targetIdentity *regionv1.OpenstackIdentity
	for _, resource := range resources.Items {
		clusterName := strings.TrimPrefix(resource.Labels[constants.NameLabel], "kubernetes-cluster-")
		if clusterName == o.clusterID {
			targetIdentity = &resource
			break
		}
	}

	if targetIdentity == nil {
		return fmt.Errorf("%w: no OpenStack identity found for cluster %s", errors.ErrValidation, o.clusterID)
	}

	// Print the SSH private key
	fmt.Println(string(targetIdentity.Spec.SSHPrivateKey))
	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	o := options{
		UnikornFlags: &factory.UnikornFlags,
	}

	cmd := &cobra.Command{
		Use:   "sshkey",
		Short: "Get SSH private key for a Kubernetes cluster",
		Long: `Get the SSH private key for a Kubernetes cluster from its OpenStack identity.

This command retrieves the SSH private key associated with a Kubernetes cluster's OpenStack identity.
You can specify either the cluster ID or name.

Examples:
  # Get SSH private key using cluster ID
  kubectl unikorn get sshkey --cluster my-cluster-id

  # Get SSH private key using cluster name
  kubectl unikorn get sshkey --name my-cluster-name`,
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
