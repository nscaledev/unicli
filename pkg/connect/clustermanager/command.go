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

package clustermanager

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type options struct {
	UnikornFlags *factory.UnikornFlags
}

func Command(factory *factory.Factory) *cobra.Command {
	o := options{
		UnikornFlags: &factory.UnikornFlags,
	}

	cmd := &cobra.Command{
		Use:   "clustermanager <name>",
		Short: "Connect to a kubernetes cluster manager",
		Aliases: []string{
			"cm",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			client, err := factory.Client()
			if err != nil {
				return err
			}

			if err := o.execute(ctx, client, args[0]); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func (o *options) execute(ctx context.Context, cli client.Client, name string) error {
	// List all namespaces
	namespaces := &corev1.NamespaceList{}
	if err := cli.List(ctx, namespaces); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Search for the clustermanager in all namespaces
	var manager *kubernetesv1.ClusterManager
	for _, namespace := range namespaces.Items {
		manager = &kubernetesv1.ClusterManager{}
		err := cli.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace.Name}, manager)
		if err == nil {
			break
		}
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get cluster manager %s: %w", name, err)
		}
		manager = nil
	}

	if manager == nil {
		return fmt.Errorf("cluster manager %s not found in any namespace", name)
	}

	// Get the vcluster pod name
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get pods -n %s -o name | grep ^pod/vcluster", manager.Namespace))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get vcluster pod: %w", err)
	}
	podName := strings.TrimSpace(string(output))
	podName = strings.TrimPrefix(podName, "pod/")
	podName = strings.TrimSuffix(podName, "-0")

	// Connect to the vcluster
	connectCmd := exec.Command("sh", "-c", fmt.Sprintf("vcluster connect %s -n %s > /dev/null 2>&1 &", podName, manager.Namespace))
	connectCmd.Stdout = nil
	connectCmd.Stderr = nil

	fmt.Println(connectCmd.String())
	if err := connectCmd.Start(); err != nil {
		return fmt.Errorf("failed to start vcluster connect command: %w", err)
	}

	fmt.Printf("Connecting to cluster manager %s in namespace %s, please wait...\n", name, manager.Namespace)
	return nil
}
