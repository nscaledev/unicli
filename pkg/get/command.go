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

package get

import (
	"github.com/spf13/cobra"

	"github.com/nscaledev/unicli/pkg/factory"
	"github.com/nscaledev/unicli/pkg/get/clustermanager"
	"github.com/nscaledev/unicli/pkg/get/computeinstance"
	"github.com/nscaledev/unicli/pkg/get/kubernetescluster"
	"github.com/nscaledev/unicli/pkg/get/network"
	"github.com/nscaledev/unicli/pkg/get/openstackidentity"
	"github.com/nscaledev/unicli/pkg/get/sshkey"
	"github.com/nscaledev/unicli/pkg/get/user"
	"github.com/nscaledev/unicli/pkg/get/virtualkubernetescluster"
)

func Command(factory *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get resources",
	}

	cmd.AddCommand(
		clustermanager.Command(factory),
		computeinstance.Command(factory),
		kubernetescluster.Command(factory),
		network.Command(factory),
		openstackidentity.Command(factory),
		sshkey.Command(factory),
		user.Command(factory),
		virtualkubernetescluster.Command(factory),
	)

	return cmd
}
