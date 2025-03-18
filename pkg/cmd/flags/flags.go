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
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/spf13/pflag"

	"github.com/unikorn-cloud/kubectl-unikorn/pkg/cmd/errors"

	"k8s.io/client-go/util/homedir"
)

type HostnameVar string

func (v *HostnameVar) Set(s string) error {
	u, err := url.ParseRequestURI("scheme://" + s)
	if err != nil {
		return err
	}

	if u.Host != s {
		return fmt.Errorf("%w: %s is not a valid domain name", errors.ErrValidation, s)
	}

	*v = HostnameVar(s)

	return nil
}

func (v HostnameVar) String() string {
	return string(v)
}

func (v HostnameVar) Type() string {
	return "domainname"
}

type UnikornFlags struct {
	Kubeconfig        string
	IdentityNamespace string
	RegionNamespace   string
}

func (o *UnikornFlags) AddFlags(f *pflag.FlagSet) {
	f.StringVar(&o.Kubeconfig, "kubeconfig", filepath.Join(homedir.HomeDir(), ".kube", "config"), "Kubernetes configuration file")
	f.StringVar(&o.IdentityNamespace, "identity-namespace", "unikorn-identity", "Identity service namespace")
	f.StringVar(&o.RegionNamespace, "region-namespace", "unikorn-region", "Region service namespace")
}

type OrganizationFlags struct {
	OrganizationName string
}

func (o *OrganizationFlags) AddFlags(f *pflag.FlagSet) {
	f.StringVar(&o.OrganizationName, "organization", "", "Organization to scope to")
}
