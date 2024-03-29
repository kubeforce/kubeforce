/*
Copyright 2022 The Kubeforce Authors.

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

package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k3f.io/kubeforce/agent/pkg/config"
)

// Validate validates the fields of the Config object.
func Validate(c *config.Config) field.ErrorList {
	return validateConfigSpec(&c.Spec, field.NewPath("spec"))
}

func validateConfigSpec(s *config.ConfigSpec, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if s.Port == 0 {
		allErrs = append(allErrs, field.Required(fieldPath.Child("port"), "cannot be zero"))
	}
	if s.PlaybookPath == "" {
		allErrs = append(allErrs, field.Required(fieldPath.Child("playbookPath"), "cannot be empty"))
	}
	allErrs = append(allErrs, validateEtcdConfig(&s.Etcd, fieldPath.Child("etcd"))...)
	allErrs = append(allErrs, validateTLS(&s.TLS, fieldPath.Child("tls"))...)
	allErrs = append(allErrs, validateAuthentication(&s.Authentication, fieldPath.Child("authentication"))...)
	return allErrs
}

func validateTLS(c *config.TLS, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(c.CertData) == 0 && c.CertFile == "" {
		allErrs = append(allErrs, field.Required(fieldPath, "both 'certData' and 'certFile' fields cannot be empty"))
	}
	if len(c.PrivateKeyData) == 0 && c.PrivateKeyFile == "" {
		allErrs = append(allErrs, field.Required(fieldPath, "both 'privateKeyData' and 'privateKeyFile' fields cannot be empty"))
	}
	return allErrs
}

func validateEtcdConfig(c *config.EtcdConfig, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if c.DataDir == "" {
		allErrs = append(allErrs, field.Required(fieldPath.Child("dataDir"), "must not be empty"))
	}
	if c.CertsDir == "" {
		allErrs = append(allErrs, field.Required(fieldPath.Child("certsDir"), "must not be empty"))
	}
	return allErrs
}

func validateAuthentication(a *config.AgentAuthentication, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateX509Authentication(&a.X509, fieldPath.Child("x509"))...)
	return allErrs
}

func validateX509Authentication(a *config.AgentX509Authentication, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(a.ClientCAData) == 0 && a.ClientCAFile == "" {
		allErrs = append(allErrs, field.Required(fieldPath, "both 'clientCAData' and 'clientCAFile' fields cannot be empty"))
	}
	return allErrs
}
