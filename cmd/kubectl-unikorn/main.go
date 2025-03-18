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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	computev1 "github.com/unikorn-cloud/compute/pkg/apis/unikorn/v1alpha1"
	identityv1 "github.com/unikorn-cloud/identity/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/cmd/create"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/cmd/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/cmd/get"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

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

func getClient(ctx context.Context) (client.Client, error) {
	path := filepath.Join(homedir.HomeDir(), ".kube", "config")

	config, err := clientcmd.BuildConfigFromFlags("", path)
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

func main() {
	client, err := getClient(context.Background())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cmd := &cobra.Command{
		Use:   "kubectl-unikorn",
		Short: "Unikorn kubectl plugin",
	}

	factory := factory.NewFactory(client)
	factory.AddFlags(cmd.PersistentFlags())

	if err := factory.RegisterCompletionFunctions(cmd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cmd.AddCommand(
		create.Command(factory),
		get.Command(factory),
	)

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
