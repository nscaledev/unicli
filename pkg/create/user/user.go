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

package user

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/unikorn-cloud/core/pkg/constants"
	coreutil "github.com/unikorn-cloud/core/pkg/util"
	identityv1 "github.com/unikorn-cloud/identity/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/errors"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/factory"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/flags"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type createUserOptions struct {
	UnikornFlags *factory.UnikornFlags

	email        string
	organization *flags.OrganizationFlags
}

func (o *createUserOptions) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	cmd.Flags().StringVar(&o.email, "email", "", "User's email address.")

	if err := cmd.MarkFlagRequired("email"); err != nil {
		return err
	}

	if err := o.organization.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	return nil
}

func (o *createUserOptions) validateUser(ctx context.Context, cli client.Client) error {
	resources := &identityv1.UserList{}

	if err := cli.List(ctx, resources, &client.ListOptions{Namespace: o.UnikornFlags.IdentityNamespace}); err != nil {
		return err
	}

	matchesEmail := func(user identityv1.User) bool {
		return user.Spec.Subject == o.email
	}

	if ok := slices.ContainsFunc(resources.Items, matchesEmail); ok {
		return fmt.Errorf("%w: user already exists", errors.ErrValidation)
	}

	return nil
}

func (o *createUserOptions) validate(ctx context.Context, cli client.Client) error {
	validators := []func(context.Context, client.Client) error{
		o.validateUser,
		o.organization.Validate,
	}

	for _, validator := range validators {
		if err := validator(ctx, cli); err != nil {
			return err
		}
	}

	return nil
}

func (o *createUserOptions) execute(ctx context.Context, cli client.Client) error {
	user := &identityv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.UnikornFlags.IdentityNamespace,
			Name:      coreutil.GenerateResourceID(),
			Labels: map[string]string{
				constants.NameLabel: constants.UndefinedName,
			},
		},
		Spec: identityv1.UserSpec{
			Subject: o.email,
			State:   identityv1.UserStateActive,
		},
	}

	if err := cli.Create(ctx, user); err != nil {
		return err
	}

	return nil
}

func Command(factory *factory.Factory) *cobra.Command {
	unikornFlags := &factory.UnikornFlags

	o := createUserOptions{
		UnikornFlags: unikornFlags,
		organization: flags.NewOrganizationFlags(unikornFlags),
	}

	cmd := &cobra.Command{
		Use:   "user",
		Short: "Create a user",
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
