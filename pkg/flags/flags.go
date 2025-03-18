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
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"

	"k8s.io/client-go/util/homedir"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UnikornFlags struct {
	Kubeconfig        string
	IdentityNamespace string
	RegionNamespace   string
}

func (o *UnikornFlags) AddFlags(f *pflag.FlagSet) {
	f.StringVar(&o.Kubeconfig, "kubeconfig", filepath.Join(homedir.HomeDir(), ".kube", "config"), "Kubernetes configuration file")
	f.StringVar(&o.IdentityNamespace, "identity-namespace", "unikorn-identity", "Identity service namespace")
	f.StringVar(&o.RegionNamespace, "region-namespace", "unikorn-region", "Region service namespace")
}

type OrganizationFlags struct {
	unikornFlags *UnikornFlags

	OrganizationName string

	OrganizationID        string
	OrganizationNamespace string
}

func NewOrganizationFlags(unikornFlags *UnikornFlags) *OrganizationFlags {
	return &OrganizationFlags{
		unikornFlags: unikornFlags,
	}
}

func (o *OrganizationFlags) AddFlags(cmd *cobra.Command, required bool) error {
	cmd.Flags().StringVar(&o.OrganizationName, "organization", "", "Organization name to scope to")

	if required {
		if err := cmd.MarkFlagRequired("organization"); err != nil {
			return err
		}
	}

	return nil
}

func (o *OrganizationFlags) Validate(ctx context.Context, cli client.Client) error {
	if o.OrganizationName == "" {
		return nil
	}

	organization, err := util.GetOrganization(ctx, cli, o.unikornFlags.IdentityNamespace, o.OrganizationName)
	if err != nil {
		return err
	}

	o.OrganizationID = organization.Name
	o.OrganizationNamespace = organization.Status.Namespace

	return nil
}
