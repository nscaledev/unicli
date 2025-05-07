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

package util

import (
	"context"
	"fmt"
	"slices"

	"github.com/unikorn-cloud/core/pkg/constants"
	identityv1 "github.com/unikorn-cloud/identity/pkg/apis/unikorn/v1alpha1"
	"github.com/unikorn-cloud/kubectl-unikorn/pkg/errors"
	kubernetesv1 "github.com/unikorn-cloud/kubernetes/pkg/apis/unikorn/v1alpha1"
	regionv1 "github.com/unikorn-cloud/region/pkg/apis/unikorn/v1alpha1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetOrganization(ctx context.Context, cli client.Client, namespace, organizatonName string) (*identityv1.Organization, error) {
	requirement, err := labels.NewRequirement(constants.NameLabel, selection.Equals, []string{organizatonName})
	if err != nil {
		return nil, err
	}

	options := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.NewSelector().Add(*requirement),
	}

	resources := &identityv1.OrganizationList{}

	if err := cli.List(ctx, resources, options); err != nil {
		return nil, err
	}

	if len(resources.Items) != 1 {
		return nil, fmt.Errorf("%w: unable to find organization with name %s", errors.ErrValidation, organizatonName)
	}

	if resources.Items[0].Status.Namespace == "" {
		return nil, fmt.Errorf("%w: unable to find organization namespace", errors.ErrValidation)
	}

	return &resources.Items[0], nil
}

func GetProject(ctx context.Context, cli client.Client, organizationID, projectName string) (*identityv1.Project, error) {
	l := labels.Set{
		constants.NameLabel: projectName,
	}

	if organizationID != "" {
		l[constants.OrganizationLabel] = organizationID
	}

	options := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(l),
	}

	resources := &identityv1.ProjectList{}

	if err := cli.List(ctx, resources, options); err != nil {
		return nil, err
	}

	if len(resources.Items) != 1 {
		return nil, fmt.Errorf("%w: unable to find project with name %s", errors.ErrValidation, projectName)
	}

	if resources.Items[0].Status.Namespace == "" {
		return nil, fmt.Errorf("%w: unable to find project namespace", errors.ErrValidation)
	}

	return &resources.Items[0], nil
}

func GetKubernetesCluster(ctx context.Context, cli client.Client, organizationID, projectID, clusterName string) (*kubernetesv1.KubernetesCluster, error) {
	l := labels.Set{
		constants.NameLabel: clusterName,
	}

	if organizationID != "" {
		l[constants.OrganizationLabel] = organizationID
	}

	if projectID != "" {
		l[constants.ProjectLabel] = projectID
	}

	options := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(l),
	}

	resources := &kubernetesv1.KubernetesClusterList{}

	if err := cli.List(ctx, resources, options); err != nil {
		return nil, err
	}

	if len(resources.Items) != 1 {
		return nil, fmt.Errorf("%w: unable to find kubernetes cluster with name %s", errors.ErrValidation, clusterName)
	}

	return &resources.Items[0], nil
}

func GetVirtualKubernetesCluster(ctx context.Context, cli client.Client, organizationID, projectID, clusterName string) (*kubernetesv1.VirtualKubernetesCluster, error) {
	l := labels.Set{
		constants.NameLabel: clusterName,
	}

	if organizationID != "" {
		l[constants.OrganizationLabel] = organizationID
	}

	if projectID != "" {
		l[constants.ProjectLabel] = projectID
	}

	options := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(l),
	}

	resources := &kubernetesv1.VirtualKubernetesClusterList{}

	if err := cli.List(ctx, resources, options); err != nil {
		return nil, err
	}

	if len(resources.Items) != 1 {
		return nil, fmt.Errorf("%w: unable to find virtual kubernetes cluster with name %s", errors.ErrValidation, clusterName)
	}

	return &resources.Items[0], nil
}

func GetRegion(ctx context.Context, cli client.Client, namespace, id string) (*regionv1.Region, error) {
	resource := &regionv1.Region{}

	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: id}, resource); err != nil {
		return nil, err
	}

	return resource, nil
}

func GetOpenstackIdentity(ctx context.Context, cli client.Client, namespace, id string) (*regionv1.OpenstackIdentity, error) {
	resource := &regionv1.OpenstackIdentity{}

	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: id}, resource); err != nil {
		return nil, err
	}

	return resource, nil
}

func GetUser(ctx context.Context, cli client.Client, namespace, email string) (*identityv1.User, error) {
	resources := &identityv1.UserList{}

	if err := cli.List(ctx, resources, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}

	index := slices.IndexFunc(resources.Items, func(user identityv1.User) bool {
		return user.Spec.Subject == email
	})

	if index < 0 {
		return nil, fmt.Errorf("%w: unable to find user with email %s", errors.ErrValidation, email)
	}

	return &resources.Items[index], nil
}
