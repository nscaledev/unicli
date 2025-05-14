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
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nscaledev/unicli/pkg/factory"
	"github.com/nscaledev/unicli/pkg/flags"
	"github.com/unikorn-cloud/core/pkg/constants"
	identityv1 "github.com/unikorn-cloud/identity/pkg/apis/unikorn/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/printers"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrConsistency = errors.New("consistency error")
)

type createUserOptions struct {
	UnikornFlags *factory.UnikornFlags

	organization *flags.OrganizationFlags
	user         *flags.UserFlags
}

func (o *createUserOptions) AddFlags(cmd *cobra.Command, factory *factory.Factory) error {
	if err := o.organization.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	if err := o.user.AddFlags(cmd, factory, false); err != nil {
		return err
	}

	return nil
}

func (o *createUserOptions) validate(ctx context.Context, cli client.Client) error {
	validators := []func(context.Context, client.Client) error{
		o.organization.Validate,
		o.user.Validate,
	}

	for _, validator := range validators {
		if err := validator(ctx, cli); err != nil {
			return err
		}
	}

	return nil
}

//nolint:cyclop
func (o *createUserOptions) execute(ctx context.Context, cli client.Client) error {
	users := &identityv1.UserList{}

	if err := cli.List(ctx, users, &client.ListOptions{}); err != nil {
		return err
	}

	userIndex := make(map[string]*identityv1.User, len(users.Items))

	for i := range users.Items {
		userIndex[users.Items[i].Name] = &users.Items[i]
	}

	organizations := &identityv1.OrganizationList{}

	if err := cli.List(ctx, organizations, &client.ListOptions{}); err != nil {
		return err
	}

	organizationIndex := make(map[string]*identityv1.Organization, len(organizations.Items))

	for i := range organizations.Items {
		organizationIndex[organizations.Items[i].Name] = &organizations.Items[i]
	}

	organizationUsers := &identityv1.OrganizationUserList{}

	options := &client.ListOptions{}

	if o.organization.Organization != nil {
		options.LabelSelector = labels.SelectorFromSet(labels.Set{
			constants.OrganizationLabel: o.organization.Organization.Name,
		})
	}

	if err := cli.List(ctx, organizationUsers, options); err != nil {
		return err
	}

	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{
				Name: "namespace",
			},
			{
				Name: "id",
			},
			{
				Name: "email",
			},
			{
				Name: "organization",
			},
		},
		Rows: make([]metav1.TableRow, 0, len(organizationUsers.Items)),
	}

	for i := range organizationUsers.Items {
		ou := &organizationUsers.Items[i]

		user, ok := userIndex[ou.Labels[constants.UserLabel]]
		if !ok {
			return fmt.Errorf("%w: organization user %s in namespace %s doesn't have corresponding user resource", ErrConsistency, ou.Name, ou.Namespace)
		}

		if o.user.Email != "" && user.Spec.Subject != o.user.Email {
			continue
		}

		organization, ok := organizationIndex[ou.Labels[constants.OrganizationLabel]]
		if !ok {
			return fmt.Errorf("%w: organization user %s in namespace %s doesn't have corresponding organization resource", ErrConsistency, ou.Name, ou.Namespace)
		}

		table.Rows = append(table.Rows, metav1.TableRow{
			Cells: []interface{}{
				ou.Namespace,
				ou.Name,
				user.Spec.Subject,
				organization.Labels[constants.NameLabel],
			},
		})
	}

	return printers.NewTablePrinter(printers.PrintOptions{}).PrintObj(table, os.Stdout)
}

func Command(factory *factory.Factory) *cobra.Command {
	unikornFlags := &factory.UnikornFlags

	o := createUserOptions{
		UnikornFlags: unikornFlags,
		organization: flags.NewOrganizationFlags(unikornFlags),
		user:         flags.NewUserFlags(unikornFlags),
	}

	cmd := &cobra.Command{
		Use:   "user",
		Short: "List users",
		Aliases: []string{
			"users",
		},
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
