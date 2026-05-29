package main

import (
	"bytes"
	"io"

	"github.com/ghodss/yaml"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type kindObject struct {
	Kind string
}

type jsonToHCLConverter func(data []byte) (tfgen.Resource, error)

var defaultConf = map[string]jsonToHCLConverter{
	"role": func(data []byte) (tfgen.Resource, error) {
		var role types.RoleV6
		if err := utils.FastUnmarshal(data, &role); err != nil {
			return nil, trace.Errorf("invalid Teleport role in the input %w", err)
		}
		return &role, nil
	},
	"user": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"trusted_cluster": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"github": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"saml": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"oidc": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"token": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"lock": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"cluster_networking_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"cluster_auth_preference": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"bot": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"autoupdate_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"autoupdate_version": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"health_check_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"workload_identity": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"app": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"db": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"kube_cluster": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"node": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"saml_idp_service_provider": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"access_list": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"access_list_member": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"access_monitoring_rule": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},

	"login_rule": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},

	"discovery_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"integration": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"okta_import_rule": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"device": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"installer": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"session_recording_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"ui_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"cluster_maintenance_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"dynamic_windows_desktop": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"static_host_user": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"vnet_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"app_auth_config": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"db_object_import_rule": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"workload_cluster": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"inference_model": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"inference_secret": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"inference_policy": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"retrieval_model": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"scoped_role": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
	"scoped_role_assignment": func(data []byte) (tfgen.Resource, error) {
		return nil, nil

	},
	"scoped_token": func(data []byte) (tfgen.Resource, error) {
		return nil, nil
	},
}

func convertYAMLToHCL(w io.Writer, r io.Reader, config map[string]jsonToHCLConverter) error {
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

	convert, ok := config[o.Kind]
	if !ok {
		return trace.Errorf("converting %v to a Terraform resource is not supported", o.Kind)
	}

	res, err := convert(jsonbytes)
	if err != nil {
		return trace.Errorf("unable to convert %v to a Terraform resource: %w", o.Kind, err)
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
