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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nscaledev/unicli/pkg/connect"
	"github.com/nscaledev/unicli/pkg/create"
	"github.com/nscaledev/unicli/pkg/describe"
	"github.com/nscaledev/unicli/pkg/factory"
	"github.com/nscaledev/unicli/pkg/get"
)

func main() {
	cmd := &cobra.Command{
		Use:   "unicli",
		Short: "Unified Nscale Infrastructure CLI",
	}

	factory := factory.NewFactory()
	factory.AddFlags(cmd.PersistentFlags())

	if err := factory.RegisterCompletionFunctions(cmd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cmd.AddCommand(
		create.Command(factory),
		describe.Command(factory),
		get.Command(factory),
		connect.Command(factory),
	)

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
