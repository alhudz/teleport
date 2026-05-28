package main

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type kindObject struct {
	kind string
}

func convertToTerraform(r io.Reader) (io.Writer, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		return nil, trace.Errorf("unable to read input YAML: %w", err)
	}

	jsonbytes, err := utils.ToJSON(buf.Bytes())
	if err != nil {
		return nil, trace.Errorf("unable to process input YAML as JSON (which we need to do to convert it to a Teleport resource type): %w", err)

	}

	var o kindObject
	if err = json.NewDecoder(r).Decode(&o); err != nil {
		return nil, trace.Errorf("unable to detect a kind in the input resource: %w", err)
	}

	switch o.kind {
	case "role":

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
		return nil, trace.Errorf("converting %o to a Terraform resource is not supported", o.kind)
	}

	return nil, nil
}

func main() {
}
