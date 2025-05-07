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

	computev1 "github.com/unikorn-cloud/compute/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/core/pkg/constants"
	identityv1 "github.com/unikorn-cloud/identity/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/util"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getScheme() (*runtime.Scheme, error) {
	schemes := []func(*runtime.Scheme) error{
		k8sscheme.AddToScheme,
		computev1.AddToScheme,
		identityv1.AddToScheme,
		kubernetesv1.AddToScheme,
		regionv1.AddToScheme,
	}

	scheme := runtime.NewScheme()

	for _, s := range schemes {
		if err := s(scheme); err != nil {
			return nil, err
		}
	}

	return scheme, nil
}

type UnikornFlags struct {
	Kubeconfig        string
	IdentityNamespace string
	RegionNamespace   string
}

type Factory struct {
	UnikornFlags UnikornFlags
}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) AddFlags(flags *pflag.FlagSet) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	flags.StringVar(&f.UnikornFlags.Kubeconfig, "kubeconfig", loadingRules.GetDefaultFilename(), "Kubernetes configuration file")
	flags.StringVar(&f.UnikornFlags.IdentityNamespace, "identity-namespace", "unikorn-identity", "Identity service namespace")
	flags.StringVar(&f.UnikornFlags.RegionNamespace, "region-namespace", "unikorn-region", "Region service namespace")
}

func (f *Factory) RegisterCompletionFunctions(cmd *cobra.Command) error {
	if err := cmd.RegisterFlagCompletionFunc("identity-namespace", f.NamespaceCompletionFunc()); err != nil {
		return err
	}

	if err := cmd.RegisterFlagCompletionFunc("region-namespace", f.NamespaceCompletionFunc()); err != nil {
		return err
	}

	return nil
}

func (f *Factory) Client() (client.Client, error) {
	// TODO: signal handler and cancel.
	ctx := context.Background()

	config, err := clientcmd.BuildConfigFromFlags("", f.UnikornFlags.Kubeconfig)
	if err != nil {
		return nil, err
	}

	scheme, err := getScheme()
	if err != nil {
		return nil, err
	}

	cache, err := cache.New(config, cache.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	go func() {
		_ = cache.Start(ctx)
	}()

	cache.WaitForCacheSync(ctx)

	options := client.Options{
		Scheme: scheme,
		Cache: &client.CacheOptions{
			Reader:       cache,
			Unstructured: false,
		},
	}

	client, err := client.New(config, options)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (f *Factory) NamespaceCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		c, err := f.Client()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		resources := &corev1.NamespaceList{}

		if err := c.List(context.Background(), resources, &client.ListOptions{}); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		names := make([]string, len(resources.Items))

		for i := range resources.Items {
			names[i] = resources.Items[i].Name
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func (f *Factory) OrganizationNameCompletionFunc() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		c, err := f.Client()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		resources := &identityv1.OrganizationList{}

		if err := c.List(context.Background(), resources, &client.ListOptions{Namespace: f.UnikornFlags.IdentityNamespace}); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		names := make([]string, len(resources.Items))

		for i := range resources.Items {
			names[i] = resources.Items[i].Labels[constants.NameLabel]
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func (f *Factory) ProjectNameCompletionFunc(organizationName *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		c, err := f.Client()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		options := &client.ListOptions{}

		if organizationName != nil && *organizationName != "" {
			organization, err := util.GetOrganization(context.Background(), c, f.UnikornFlags.IdentityNamespace, *organizationName)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			options.LabelSelector = labels.SelectorFromSet(labels.Set{
				constants.OrganizationLabel: organization.Name,
			})
		}

		resources := &identityv1.ProjectList{}

		if err := c.List(context.Background(), resources, options); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		names := make([]string, len(resources.Items))

		for i := range resources.Items {
			names[i] = resources.Items[i].Labels[constants.NameLabel]
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func (f *Factory) KubernetesClusterNameCompletionFunc(organizationName, projectName *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		c, err := f.Client()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		l := labels.Set{}

		if organizationName != nil && *organizationName != "" {
			organization, err := util.GetOrganization(context.Background(), c, f.UnikornFlags.IdentityNamespace, *organizationName)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			l[constants.OrganizationLabel] = organization.Name
		}

		if projectName != nil && *projectName != "" {
			project, err := util.GetProject(context.Background(), c, l[constants.OrganizationLabel], *projectName)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			l[constants.ProjectLabel] = project.Name
		}

		options := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(l),
		}

		resources := &kubernetesv1.KubernetesClusterList{}

		if err := c.List(context.Background(), resources, options); err != nil {
			return nil, cobra.ShellCompDirectiveError
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
		c, err := f.Client()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		resources := &identityv1.RoleList{}

		if err := c.List(context.Background(), resources, &client.ListOptions{Namespace: f.UnikornFlags.IdentityNamespace}); err != nil {
			return nil, cobra.ShellCompDirectiveError
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
		c, err := f.Client()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		resources := &identityv1.UserList{}

		if err := c.List(context.Background(), resources, &client.ListOptions{Namespace: f.UnikornFlags.IdentityNamespace}); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		names := make([]string, len(resources.Items))

		for i := range resources.Items {
			names[i] = resources.Items[i].Spec.Subject
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}
