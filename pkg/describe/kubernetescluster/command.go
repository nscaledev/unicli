/*
Copyright 2025 the Unikorn Authors.

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
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/flags"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	ErrConsistency = errors.New("consistency error")
)

type options struct {
	UnikornFlags *factory.UnikornFlags

	organization *flags.OrganizationFlags
	project      *flags.ProjectFlags
	cluster      *flags.KubernetesClusterFlags
}

func (o *options) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	if err := o.organization.AddFlags(cmd, factory, true); err != nil {
		return err
	}

	if err := o.project.AddFlags(cmd, factory, true); err != nil {
		return err
	}

	if err := o.cluster.AddFlags(cmd, factory, true); err != nil {
		return err
	}

	return nil
}

func (o *options) validate(ctx context.Context, cli client.Client) error {
	validators := []func(context.Context, client.Client) error{
		o.organization.Validate,
		o.project.Validate,
		o.cluster.Validate,
	}

	for _, validator := range validators {
		if err := validator(ctx, cli); err != nil {
			return err
		}
	}

	return nil
}

func (o *options) execute(ctx context.Context, cli client.Client) error {
	organization := map[string]any{
		"id":        o.organization.Organization.Name,
		"name":      o.organization.Organization.Labels[constants.NameLabel],
		"namespace": o.organization.Organization.Status.Namespace,
	}

	project := map[string]any{
		"id":        o.project.Project.Name,
		"name":      o.project.Project.Labels[constants.NameLabel],
		"namespace": o.project.Project.Status.Namespace,
	}

	regionInfo, err := util.GetRegion(ctx, cli, o.UnikornFlags.RegionNamespace, o.cluster.Cluster.Spec.RegionID)
	if err != nil {
		return err
	}

	region := map[string]any{
		"id":       regionInfo.Name,
		"name":     regionInfo.Labels[constants.NameLabel],
		"provider": regionInfo.Spec.Provider,
	}

	identity := map[string]any{
		"id": o.cluster.Cluster.Annotations[constants.IdentityAnnotation],
	}

	//nolint:gocritic,exhaustive
	switch regionInfo.Spec.Provider {
	case regionv1.ProviderOpenstack:
		openstackIdentity, err := util.GetOpenstackIdentity(ctx, cli, o.UnikornFlags.RegionNamespace, o.cluster.Cluster.Annotations[constants.IdentityAnnotation])
		if err != nil {
			return err
		}

		openstack := map[string]any{
			"user": map[string]any{
				"id":       openstackIdentity.Spec.UserID,
				"password": openstackIdentity.Spec.Password,
			},
			"project": map[string]any{
				"id": openstackIdentity.Spec.ProjectID,
			},
			"ssh": map[string]any{
				"privateKey": openstackIdentity.Spec.SSHPrivateKey,
			},
		}

		identity["openstack"] = openstack
	}

	pools := make([]map[string]any, len(o.cluster.Cluster.Spec.WorkloadPools.Pools))

	for i := range o.cluster.Cluster.Spec.WorkloadPools.Pools {
		pool := &o.cluster.Cluster.Spec.WorkloadPools.Pools[i]

		pools[i] = map[string]any{
			"name": pool.Name,
			"image": map[string]any{
				"id": pool.ImageID,
			},
			"flavor": map[string]any{
				"id": pool.FlavorID,
			},
			"replicas": pool.Replicas,
			"disk": map[string]any{
				"size": pool.DiskSize,
			},
		}
	}

	cluster := map[string]any{
		"clustermanager": map[string]any{
			"id": o.cluster.Cluster.Spec.ClusterManagerID,
		},
		"versions": map[string]any{
			"kubernetes":   o.cluster.Cluster.Spec.Version.String(),
			"applications": o.cluster.Cluster.Spec.ApplicationBundle,
		},
		"network": map[string]any{
			"podPrefix":     o.cluster.Cluster.Spec.Network.PodNetwork.String(),
			"servicePrefix": o.cluster.Cluster.Spec.Network.ServiceNetwork.String(),
			"nodePrefix":    o.cluster.Cluster.Spec.Network.NodeNetwork.String(),
		},
		"controlPlane": map[string]any{
			"image": map[string]any{
				"id": o.cluster.Cluster.Spec.ControlPlane.ImageID,
			},
			"flavor": map[string]any{
				"id": o.cluster.Cluster.Spec.ControlPlane.FlavorID,
			},
			"replicas": o.cluster.Cluster.Spec.ControlPlane.Replicas,
			"disk": map[string]any{
				"size": o.cluster.Cluster.Spec.ControlPlane.DiskSize,
			},
		},
		"pools": pools,
	}

	description := map[string]any{
		"organization": organization,
		"project":      project,
		"region":       region,
		"identity":     identity,
		"cluster":      cluster,
	}

	data, err := yaml.Marshal(description)
	if err != nil {
		return err
	}

	fmt.Println(string(data))

	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	unikornFlags := &factory.UnikornFlags
	organizationFlags := flags.NewOrganizationFlags(unikornFlags)
	projectFlags := flags.NewProjectFlags(unikornFlags, organizationFlags)
	clusterFlags := flags.NewKubernetesClusterFlags(unikornFlags, organizationFlags, projectFlags)

	o := options{
		UnikornFlags: unikornFlags,
		organization: organizationFlags,
		project:      projectFlags,
		cluster:      clusterFlags,
	}

	cmd := &cobra.Command{
		Use:   "kubernetescluster",
		Short: "Describe kubernetes clusters",
		Aliases: []string{
			"kubernetesclusters",
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
