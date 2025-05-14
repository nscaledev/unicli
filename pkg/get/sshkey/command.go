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

	"github.com/nscaledev/unicli/pkg/errors"
	"github.com/nscaledev/unicli/pkg/factory"
	"github.com/nscaledev/unicli/pkg/util"
	"github.com/unikorn-cloud/core/pkg/constants"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type options struct {
	UnikornFlags      *factory.UnikornFlags
	clusterIdentifier string // Unified field for cluster name or ID
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	// No flags needed, identifier comes from positional argument.
	return nil
}

func (o *options) validate(_ context.Context, _ client.Client) error {
	// Validation for argument presence is handled by cobra.ExactArgs(1).
	// o.clusterIdentifier will be non-empty if cobra validation passes.
	// The actual validity of the identifier (as a name or ID) is checked in 'execute'.
	return nil
}

func (o *options) execute(ctx context.Context, cli client.Client) error {
	var resolvedClusterID string

	// Retrieve all cluster names and IDs to perform the lookup.
	clusterNameMap, err := util.CreateKubernetesClusterNameMap(ctx, cli, "", "")
	if err != nil {
		return fmt.Errorf("failed to get cluster names: %w", err)
	}

	// Attempt to resolve the identifier as a name first.
	foundAsName := false
	for id, name := range clusterNameMap {
		if name == o.clusterIdentifier {
			resolvedClusterID = id
			foundAsName = true
			break
		}
	}

	if !foundAsName {
		// If not found as a name, assume the identifier is an ID.
		// Validate that this ID exists in our map of known clusters.
		if _, idExists := clusterNameMap[o.clusterIdentifier]; idExists {
			resolvedClusterID = o.clusterIdentifier
		} else {
			// The identifier is neither a known name nor a known ID.
			return fmt.Errorf("%w: cluster '%s' not found. Please provide a valid cluster name or ID", errors.ErrValidation, o.clusterIdentifier)
		}
	}

	// Now, resolvedClusterID contains the validated cluster ID.
	// Proceed to fetch the OpenStack identity.
	resources := &regionv1.OpenstackIdentityList{}
	if err := cli.List(ctx, resources, &client.ListOptions{Namespace: o.UnikornFlags.RegionNamespace}); err != nil {
		return fmt.Errorf("failed to list OpenStack identities: %w", err)
	}

	var targetIdentity *regionv1.OpenstackIdentity
	for i := range resources.Items {
		// Iterate by index to correctly get a pointer to the item in the slice,
		// fixing a potential bug from taking the address of a loop variable copy.
		resourceInLoop := &resources.Items[i]
		clusterNameInLabel := strings.TrimPrefix(resourceInLoop.Labels[constants.NameLabel], "kubernetes-cluster-")

		if clusterNameInLabel == resolvedClusterID {
			targetIdentity = resourceInLoop
			break
		}
	}

	if targetIdentity == nil {
		return fmt.Errorf("%w: no OpenStack identity found for cluster %s", errors.ErrValidation, resolvedClusterID)
	}

	fmt.Println(string(targetIdentity.Spec.SSHPrivateKey))
	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	o := options{
		UnikornFlags: &factory.UnikornFlags,
	}

	cmd := &cobra.Command{
		Use:   "sshkey <cluster-identifier>",
		Short: "Get SSH private key for a Kubernetes cluster",
		Long: `Get the SSH private key for a Kubernetes cluster from its OpenStack identity.

You can specify either the cluster ID or its name as the <cluster-identifier> argument.

Examples:
  # Get SSH private key using cluster ID
  unicli get sshkey my-cluster-id

  # Get SSH private key using cluster name
  unicli get sshkey my-cluster-name`,
		Args: cobra.ExactArgs(1), // Ensures exactly one argument is provided
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			client, err := factory.Client()
			if err != nil {
				return err
			}

			// Store the positional argument as the cluster identifier.
			o.clusterIdentifier = args[0]

			if err := o.validate(ctx, client); err != nil {
				return err
			}

			if err := o.execute(ctx, client); err != nil {
				return err
			}

			return nil
		},
	}

	// o.AddFlags now does nothing, but the call pattern is preserved.
	if err := o.AddFlags(cmd, factory); err != nil {
		panic(err)
	}

	return cmd
}
