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

	"github.com/nscaledev/unicli/pkg/factory"
	"github.com/nscaledev/unicli/pkg/util"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KubernetesClusterFlags struct {
	unikornFlags      *factory.UnikornFlags
	organizationFlags *OrganizationFlags
	projectFlags      *ProjectFlags

	ClusterName string

	Cluster *kubernetesv1.KubernetesCluster
}

func NewKubernetesClusterFlags(unikornFlags *factory.UnikornFlags, organizationFlags *OrganizationFlags, projectFlags *ProjectFlags) *KubernetesClusterFlags {
	return &KubernetesClusterFlags{
		unikornFlags:      unikornFlags,
		organizationFlags: organizationFlags,
		projectFlags:      projectFlags,
	}
}

func (f *KubernetesClusterFlags) AddFlags(cmd *cobra.Command, factory *factory.Factory, required bool) error {
	cmd.Flags().StringVar(&f.ClusterName, "kubernetescluster", "", "Kubernetes cluster name")

	if required {
		if err := cmd.MarkFlagRequired("kubernetescluster"); err != nil {
			return err
		}
	}

	if err := cmd.RegisterFlagCompletionFunc("kubernetescluster", factory.KubernetesClusterNameCompletionFunc(&f.organizationFlags.OrganizationName, &f.projectFlags.ProjectName)); err != nil {
		return err
	}

	return nil
}

func (f *KubernetesClusterFlags) Validate(ctx context.Context, cli client.Client) error {
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

	cluster, err := util.GetKubernetesCluster(ctx, cli, organizationID, projectID, f.ClusterName)
	if err != nil {
		return err
	}

	f.Cluster = cluster

	return nil
}
