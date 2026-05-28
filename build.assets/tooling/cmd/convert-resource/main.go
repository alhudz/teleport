package main

import (
	"bytes"
	"io"

	"github.com/ghodss/yaml"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type kindObject struct {
	Kind string
}

func convertToTerraform(w io.Writer, r io.Reader) error {
	var yamlBuf, kindBuf bytes.Buffer
	dest := io.MultiWriter(&yamlBuf, &kindBuf)
	_, err := io.Copy(dest, r)
	if err != nil {
		return trace.Errorf("unable to read input YAML: %w", err)
	}

	jsonbytes, err := utils.ToJSON(yamlBuf.Bytes())
	if err != nil {
		return trace.Errorf("unable to process input YAML as JSON (which we need to do to convert it to a Teleport resource type): %w", err)

	}

	var o kindObject
	if err = yaml.Unmarshal(jsonbytes, &o); err != nil {
		return trace.Errorf("unable to detect a kind in the input resource: %w", err)
	}

	var res tfgen.Resource
	switch o.Kind {
	case "role":
		res, err = services.UnmarshalRole(jsonbytes)
		if err != nil {
			return trace.Errorf("invalid Teleport role in the input %w", err)
		}
	case "user":
	case "trusted_cluster":
	case "github":
	case "saml":
	case "oidc":
	case "token":
	case "lock":
	case "cluster_networking_config":
	case "cluster_auth_preference":
	case "bot":
	case "autoupdate_config":
	case "autoupdate_version":
	case "health_check_config":
	case "workload_identity":
	case "app":
	case "db":
	case "kube_cluster":
	case "node":
	case "saml_idp_service_provider":
	case "access_list":
	case "access_list_member":
	case "access_monitoring_rule":
	case "login_rule":
	case "discovery_config":
	case "integration":
	case "okta_import_rule":
	case "device":
	case "installer":
	case "session_recording_config":
	case "ui_config":
	case "cluster_maintenance_config":
	case "dynamic_windows_desktop":
	case "static_host_user":
	case "vnet_config":
	case "app_auth_config":
	case "db_object_import_rule":
	case "workload_cluster":
	case "inference_model":
	case "inference_secret":
	case "inference_policy":
	case "retrieval_model":
	case "scoped_role":
	case "scoped_role_assignment":
	case "scoped_token":
	default:
		return trace.Errorf("converting %v to a Terraform resource is not supported", o.Kind)
	}

	outbytes, err := tfgen.Generate(res)
	if err != nil {
		return trace.Errorf("unable to convert the provided YAML manifest into HCL: %w", err)
	}
	if _, err := w.Write(outbytes); err != nil {
		return trace.Errorf("unable to process the converted HCL: %w", err)
	}
	return nil
}

func main() {
}
