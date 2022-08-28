package validation

import (
	"k3f.io/kubeforce/agent/pkg/config"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

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
	if len(c.CertData) == 0 && len(c.CertFile) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath, "both 'certData' and 'certFile' fields cannot be empty"))
	}
	if len(c.PrivateKeyData) == 0 && len(c.PrivateKeyFile) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath, "both 'privateKeyData' and 'privateKeyFile' fields cannot be empty"))
	}
	return allErrs
}

func validateEtcdConfig(c *config.EtcdConfig, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(c.DataDir) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath.Child("dataDir"), "must not be empty"))
	}
	if len(c.CertsDir) == 0 {
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
	if len(a.ClientCAData) == 0 && len(a.ClientCAFile) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath, "both 'clientCAData' and 'clientCAFile' fields cannot be empty"))
	}
	return allErrs
}
