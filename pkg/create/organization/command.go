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

package organization

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nscaledev/unicli/pkg/errors"
	"github.com/nscaledev/unicli/pkg/factory"
	"github.com/unikorn-cloud/core/pkg/constants"
	"github.com/unikorn-cloud/core/pkg/util"
	"github.com/unikorn-cloud/core/pkg/util/retry"
	identityv1 "github.com/unikorn-cloud/identity/pkg/apis/unikorn/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type createOrganizationOptions struct {
	UnikornFlags *factory.UnikornFlags

	name        string
	description string
}

func (o *createOrganizationOptions) AddFlags(cmd *cobra.Command, _ *factory.Factory) error {
	cmd.Flags().StringVar(&o.name, "name", "", "Organization name.")
	cmd.Flags().StringVar(&o.description, "description", "", "A verbose organization description.")

	if err := cmd.MarkFlagRequired("name"); err != nil {
		return err
	}

	return nil
}

// validateOrganization ensures the organization doesn't already exist.
func (o *createOrganizationOptions) validateOrganization(ctx context.Context, cli client.Client) error {
	requirement, err := labels.NewRequirement(constants.NameLabel, selection.Equals, []string{o.name})
	if err != nil {
		return err
	}

	selector := labels.NewSelector()
	selector = selector.Add(*requirement)

	options := &client.ListOptions{
		Namespace:     o.UnikornFlags.IdentityNamespace,
		LabelSelector: selector,
	}

	var resources identityv1.OrganizationList

	if err := cli.List(ctx, &resources, options); err != nil {
		return err
	}

	if len(resources.Items) != 0 {
		return fmt.Errorf("%w: expected no organizations to exist with name %s", errors.ErrValidation, o.name)
	}

	return nil
}

func (o *createOrganizationOptions) validate(ctx context.Context, cli client.Client) error {
	validators := []func(context.Context, client.Client) error{
		o.validateOrganization,
	}

	for _, validator := range validators {
		if err := validator(ctx, cli); err != nil {
			return err
		}
	}

	return nil
}

func (o *createOrganizationOptions) execute(ctx context.Context, cli client.Client) error {
	organizationID := util.GenerateResourceID()

	organization := &identityv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.UnikornFlags.IdentityNamespace,
			Name:      organizationID,
			Labels: map[string]string{
				constants.NameLabel: o.name,
			},
		},
	}

	if err := cli.Create(ctx, organization); err != nil {
		return err
	}

	callback := func() error {
		if err := cli.Get(ctx, client.ObjectKey{Namespace: o.UnikornFlags.IdentityNamespace, Name: organizationID}, organization); err != nil {
			return err
		}

		if organization.Status.Namespace == "" {
			return fmt.Errorf("%w: organization not provisioned", errors.ErrResource)
		}

		return nil
	}

	if err := retry.Forever().DoWithContext(ctx, callback); err != nil {
		return err
	}

	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	o := createOrganizationOptions{
		UnikornFlags: &factory.UnikornFlags,
	}

	cmd := &cobra.Command{
		Use:   "organization",
		Short: "Create an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			client, err := factory.Client()
			if err != nil {
				return err
			}

			if err := o.validate(ctx, client); err != nil {
				return err
			}

			if err := o.execute(ctx, client); err != nil {
				return err
			}

			return nil
		},
	}

	if err := o.AddFlags(cmd, factory); err != nil {
		panic(err)
	}

	return cmd
}
