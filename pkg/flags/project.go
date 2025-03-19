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

type ProjectFlags struct {
	unikornFlags      *factory.UnikornFlags
	organizationFlags *OrganizationFlags

	ProjectName string

	Project *identityv1.Project
}

func NewProjectFlags(unikornFlags *factory.UnikornFlags, organizationFlags *OrganizationFlags) *ProjectFlags {
	return &ProjectFlags{
		unikornFlags:      unikornFlags,
		organizationFlags: organizationFlags,
	}
}

func (f *ProjectFlags) AddFlags(cmd *cobra.Command, factory *factory.Factory, required bool) error {
	cmd.Flags().StringVar(&f.ProjectName, "project", "", "Project name")

	if required {
		if err := cmd.MarkFlagRequired("project"); err != nil {
			return err
		}
	}

	if err := cmd.RegisterFlagCompletionFunc("project", factory.ProjectNameCompletionFunc(&f.organizationFlags.OrganizationName)); err != nil {
		return err
	}

	return nil
}

func (f *ProjectFlags) Validate(ctx context.Context, cli client.Client) error {
	if f.ProjectName == "" {
		return nil
	}

	var organizationID string

	if f.organizationFlags.Organization != nil {
		organizationID = f.organizationFlags.Organization.Name
	}

	project, err := util.GetProject(ctx, cli, organizationID, f.ProjectName)
	if err != nil {
		return err
	}

	f.Project = project

	return nil
}
