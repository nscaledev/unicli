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

	identityv1 "github.com/unikorn-cloud/identity/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OrganizationFlags struct {
	unikornFlags *factory.UnikornFlags

	OrganizationName string

	Organization *identityv1.Organization
}

func NewOrganizationFlags(unikornFlags *factory.UnikornFlags) *OrganizationFlags {
	return &OrganizationFlags{
		unikornFlags: unikornFlags,
	}
}

func (f *OrganizationFlags) AddFlags(cmd *cobra.Command, factory *factory.Factory, required bool) error {
	cmd.Flags().StringVar(&f.OrganizationName, "organization", "", "Organization name")

	if required {
		if err := cmd.MarkFlagRequired("organization"); err != nil {
			return err
		}
	}

	if err := cmd.RegisterFlagCompletionFunc("organization", factory.OrganizationNameCompletionFunc()); err != nil {
		return err
	}

	return nil
}

func (f *OrganizationFlags) Validate(ctx context.Context, cli client.Client) error {
	if f.OrganizationName == "" {
		return nil
	}

	organization, err := util.GetOrganization(ctx, cli, f.unikornFlags.IdentityNamespace, f.OrganizationName)
	if err != nil {
		return err
	}

	f.Organization = organization

	return nil
}
