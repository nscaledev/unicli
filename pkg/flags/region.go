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
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RegionFlags struct {
	unikornFlags *factory.UnikornFlags

	RegionName string

	Region *regionv1.Region
}

func NewRegionFlags(unikornFlags *factory.UnikornFlags) *RegionFlags {
	return &RegionFlags{
		unikornFlags: unikornFlags,
	}
}

func (f *RegionFlags) AddFlags(cmd *cobra.Command, factory *factory.Factory, required bool) error {
	cmd.Flags().StringVar(&f.RegionName, "region", "", "Region name")

	if required {
		if err := cmd.MarkFlagRequired("region"); err != nil {
			return err
		}
	}

	if err := cmd.RegisterFlagCompletionFunc("region", factory.RegionNameCompletionFunc()); err != nil {
		return err
	}

	return nil
}

func (f *RegionFlags) Validate(ctx context.Context, cli client.Client) error {
	if f.RegionName == "" {
		return nil
	}

	region, err := util.GetRegionByName(ctx, cli, f.unikornFlags.RegionNamespace, f.RegionName)
	if err != nil {
		return err
	}

	f.Region = region

	return nil
}
