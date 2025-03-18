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

package factory

import (
	"context"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/unikorn-cloud/core/pkg/constants"
	identityv1 "github.com/unikorn-cloud/identity/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/cmd/flags"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Factory struct {
	UnikornFlags flags.UnikornFlags

	client client.Client
}

func NewFactory(client client.Client) *Factory {
	return &Factory{
		client: client,
	}
}

func (f *Factory) AddFlags(flags *pflag.FlagSet) {
	f.UnikornFlags.AddFlags(flags)
}

func (f *Factory) RegisterCompletionFunctions(cmd *cobra.Command) error {
	// TODO: add namespace lookups for the UnikornFlags
	return nil
}

func (f *Factory) Client() client.Client {
	return f.client
}

func (f *Factory) OrganizationNameCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		resources := &identityv1.OrganizationList{}

		if err := f.client.List(context.Background(), resources, &client.ListOptions{Namespace: f.UnikornFlags.IdentityNamespace}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		names := make([]string, len(resources.Items))

		for i := range resources.Items {
			names[i] = resources.Items[i].Labels[constants.NameLabel]
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func (f *Factory) RoleNameCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		resources := &identityv1.RoleList{}

		if err := f.client.List(context.Background(), resources, &client.ListOptions{Namespace: f.UnikornFlags.IdentityNamespace}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		resources.Items = slices.DeleteFunc(resources.Items, func(role identityv1.Role) bool {
			return role.Spec.Protected
		})

		names := make([]string, len(resources.Items))

		for i := range resources.Items {
			names[i] = resources.Items[i].Labels[constants.NameLabel]
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func (f *Factory) UserSubjectCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		resources := &identityv1.UserList{}

		if err := f.client.List(context.Background(), resources, &client.ListOptions{Namespace: f.UnikornFlags.IdentityNamespace}); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		names := make([]string, len(resources.Items))

		for i := range resources.Items {
			names[i] = resources.Items[i].Spec.Subject
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
