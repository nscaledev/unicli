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

package flags

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type VirtualKubernetesClusterFlags struct {
	unikornFlags      *factory.UnikornFlags
	organizationFlags *OrganizationFlags
	projectFlags      *ProjectFlags

	ClusterName string

	Cluster *kubernetesv1.VirtualKubernetesCluster
}

func NewVirtualKubernetesClusterFlags(unikornFlags *factory.UnikornFlags, organizationFlags *OrganizationFlags, projectFlags *ProjectFlags) *VirtualKubernetesClusterFlags {
	return &VirtualKubernetesClusterFlags{
		unikornFlags:      unikornFlags,
		organizationFlags: organizationFlags,
		projectFlags:      projectFlags,
	}
}

func (f *VirtualKubernetesClusterFlags) AddFlags(cmd *cobra.Command, factory *factory.Factory, required bool) error {
	cmd.Flags().StringVar(&f.ClusterName, "virtualkubernetescluster", "", "Virtual Kubernetes cluster name")

	if required {
		if err := cmd.MarkFlagRequired("virtualkubernetescluster"); err != nil {
			return err
		}
	}

	if err := cmd.RegisterFlagCompletionFunc("virtualkubernetescluster", factory.VirtualKubernetesClusterNameCompletionFunc(&f.organizationFlags.OrganizationName, &f.projectFlags.ProjectName)); err != nil {
		return err
	}

	return nil
}

func (f *VirtualKubernetesClusterFlags) Validate(ctx context.Context, cli client.Client) error {
	if f.ClusterName == "" {
		return nil
	}

	var organizationID string

	if f.organizationFlags.Organization != nil {
		organizationID = f.organizationFlags.Organization.Name
	}

	var projectID string

	if f.projectFlags.Project != nil {
		projectID = f.projectFlags.Project.Name
	}

	cluster, err := util.GetVirtualKubernetesCluster(ctx, cli, organizationID, projectID, f.ClusterName)
	if err != nil {
		return err
	}

	f.Cluster = cluster

	return nil
}
