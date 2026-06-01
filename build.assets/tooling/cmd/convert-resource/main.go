package main

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/ghodss/yaml"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	vnet "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	workloadcluster "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	convertv1 "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	discoveryConfigConvertv1 "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type kindObject struct {
	Kind string
}

type jsonConverter func(data []byte) (tfgen.Resource, error)
type kubernetesConverter func(res tfgen.Resource) (runtime.Object, error)

type conversionRule struct {
	toTeleport   jsonConverter
	toKubernetes kubernetesConverter
}

var resourceTypeOverrides = map[string]string{
	"cluster_auth_preference": "teleport_auth_preference",
	"db":                      "teleport_database",
	"github":                  "teleport_github_connector",
	"oidc":                    "teleport_oidc_connector",
	"saml":                    "teleport_saml_connector",
	"token":                   "teleport_provision_token",
	"node":                    "teleport_server",
	"device":                  "teleport_device_trust",
}

// resourceConfig maps the kind values of resources supported by the Terraform
// provider to functions for converting JSON to HCL, as well as to functions for
// converting Teleport resource types to Kubernetes resources. There are three
// patterns for applying the conversion:
//  1. For legacy gogo-proto types, the YAML/JSON type directly maps to the
//     Protobuf-generated type, which includes json struct tags, so we can
//     unmarshal directly using utils.FastUnmarshal.
//  2. For types based on non-gogo Protobuf messages, unmarshal using
//     protojson.Unmarshal.
//  3. For resources that include a header, call utils.FastUnmarshal into the
//     internal representation of the type, then convert to a Protobuf-based type
//     and wrap with a header.
var resourceConfig = map[string]conversionRule{
	"role": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.RoleV6
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid Teleport role: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			role, ok := res.(*types.RoleV6)
			if !ok {
				return nil, trace.Errorf("cannot convert the input resource to a valid Teleport role")
			}

			crd := resourcesv1.TeleportRoleV8{
				TypeMeta: metav1.TypeMeta{
					Kind:       "TeleportRoleV8",
					APIVersion: "resources.teleport.dev/v1",
				},
				Spec: resourcesv1.TeleportRoleV8Spec(role.Spec),
				ObjectMeta: metav1.ObjectMeta{
					Name: role.Metadata.Name,
				},
			}
			return &crd, nil
		},
	},
	"user": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.UserV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid user: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"trusted_cluster": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.TrustedClusterV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid trusted_cluster: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"github": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.GithubConnectorV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid github connector: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"saml": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.SAMLConnectorV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid saml connector: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"oidc": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.OIDCConnectorV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid oidc connector: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"token": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.ProvisionTokenV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid token: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"lock": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.LockV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid lock: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"cluster_networking_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.ClusterNetworkingConfigV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid cluster_networking_config: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"cluster_auth_preference": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.AuthPreferenceV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid cluster_auth_preference: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"bot": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r machineidv1.Bot
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid bot: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"autoupdate_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r autoupdatev1pb.AutoUpdateConfig
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid autoupdate_config: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"autoupdate_version": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r autoupdatev1pb.AutoUpdateVersion
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid autoupdate_version: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"health_check_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r healthcheckconfigv1.HealthCheckConfig
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid health_check_config: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"workload_identity": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r workloadidentityv1.WorkloadIdentity
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid workload_identity: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"app": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.AppV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid app: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"db": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.DatabaseV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid db: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"kube_cluster": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.KubernetesClusterV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid kube_cluster: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"node": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.ServerV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid node: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"saml_idp_service_provider": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.SAMLIdPServiceProviderV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid saml_idp_service_provider: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"access_list": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var al accesslist.AccessList
			if err := utils.FastUnmarshal(data, &al); err != nil {
				return nil, trace.Errorf("invalid access_list: %w", err)
			}
			return tfgen.WrapHeaderResource(convertv1.ToProto(&al)), nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"access_list_member": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var m accesslist.AccessListMember
			if err := utils.FastUnmarshal(data, &m); err != nil {
				return nil, trace.Errorf("invalid access_list_member: %w", err)
			}
			return tfgen.WrapHeaderResource(convertv1.ToMemberProto(&m)), nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"access_monitoring_rule": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r accessmonitoringrulesv1.AccessMonitoringRule
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid access_monitoring_rule: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"login_rule": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			return nil, trace.Errorf("login_rule is not yet supported for HCL conversion, since performing the conversion requires running the Terraform provider")
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"discovery_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var dc discoveryconfig.DiscoveryConfig
			if err := utils.FastUnmarshal(data, &dc); err != nil {
				return nil, trace.Errorf("invalid discovery_config: %w", err)
			}
			return tfgen.WrapHeaderResource(discoveryConfigConvertv1.ToProto(&dc)), nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"integration": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.IntegrationV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid integration: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"okta_import_rule": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.OktaImportRuleV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid okta_import_rule: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"device": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.DeviceV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid device: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"installer": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.InstallerV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid installer: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"session_recording_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.SessionRecordingConfigV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid session_recording_config: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"ui_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.UIConfigV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid ui_config: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"cluster_maintenance_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.ClusterMaintenanceConfigV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid cluster_maintenance_config: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"dynamic_windows_desktop": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.DynamicWindowsDesktopV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid dynamic_windows_desktop: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"static_host_user": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r userprovisioningpb.StaticHostUser
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid static_host_user: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"vnet_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r vnet.VnetConfig
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid vnet_config: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"app_auth_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r appauthconfigv1.AppAuthConfig
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid app_auth_config: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"db_object_import_rule": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r dbobjectimportrulev1.DatabaseObjectImportRule
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid db_object_import_rule: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"workload_cluster": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r workloadcluster.WorkloadCluster
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid workload_cluster: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"inference_model": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r summarizerv1.InferenceModel
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid inference_model: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"inference_secret": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r summarizerv1.InferenceSecret
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid inference_secret: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"inference_policy": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r summarizerv1.InferencePolicy
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid inference_policy: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"retrieval_model": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r summarizerv1.RetrievalModel
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid retrieval_model: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"scoped_role": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r scopedaccessv1.ScopedRole
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid scoped_role: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"scoped_role_assignment": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r scopedaccessv1.ScopedRoleAssignment
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid scoped_role_assignment: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
	"scoped_token": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r joiningv1.ScopedToken
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid scoped_token: %w", err)
			}
			return &r, nil
		},
		toKubernetes: func(res tfgen.Resource) (runtime.Object, error) {
			return nil, nil
		},
	},
}

func convertYAMLToHCL(w io.Writer, r io.Reader) error {
	var yamlBuf bytes.Buffer
	if _, err := io.Copy(&yamlBuf, r); err != nil {
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

	convert, ok := resourceConfig[o.Kind]
	if !ok {
		return trace.Errorf("converting %v to a Terraform resource is not supported", o.Kind)
	}

	res, err := convert.toTeleport(jsonbytes)
	if err != nil {
		return trace.Errorf("unable to convert %v to a Teleport resource: %w", o.Kind, err)
	}

	var opts []tfgen.GenerateOpt
	if override, ok := resourceTypeOverrides[o.Kind]; ok {
		opts = append(opts, tfgen.WithResourceType(override))
	}

	outbytes, err := tfgen.Generate(res, opts...)
	if err != nil {
		return trace.Errorf("unable to convert the provided YAML manifest into HCL: %w", err)
	}
	if _, err := w.Write(outbytes); err != nil {
		return trace.Errorf("unable to process the converted HCL: %w", err)
	}
	return nil
}

func marshalToYAMLWithNoEmptyFields(obj runtime.Object) ([]byte, error) {
	jsonbytes, err := json.Marshal(obj)
	if err != nil {
		return nil, trace.Errorf("unable to convert an object to JSON (this is a bug): %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(jsonbytes, &m); err != nil {
		return nil, trace.Errorf("unable to unmarshal JSON to a map (this is a bug): %w", err)
	}
	stripEmptyFields(m)
	return yaml.Marshal(m)
}

func stripEmptyFields(m map[string]any) {
	for k, v := range m {
		switch v.(type) {
		case map[string]any:
			msa := v.(map[string]any)
			stripEmptyFields(msa)
			if len(msa) == 0 {
				delete(m, k)
			}
		case string:
			if v.(string) == "" {
				delete(m, k)
			}
		case nil:
			delete(m, k)
		}
	}
}

func convertYAMLtoKubernetes(w io.Writer, r io.Reader) error {
	var yamlBuf bytes.Buffer
	if _, err := io.Copy(&yamlBuf, r); err != nil {
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

	convert, ok := resourceConfig[o.Kind]
	if !ok {
		return trace.Errorf("converting %v to a Kubernetes operator resource is not supported", o.Kind)
	}

	res, err := convert.toTeleport(jsonbytes)
	if err != nil {
		return trace.Errorf("unable to convert %v to a Teleport resource: %w", o.Kind, err)
	}

	crd, err := convert.toKubernetes(res)
	if err != nil {
		return trace.Errorf("unable to convert %v to a Kubernetes operator resource: %w", o.Kind, err)
	}

	outbytes, err := marshalToYAMLWithNoEmptyFields(crd)
	if err != nil {
		return trace.Errorf("could not encode %v as YAML: %w", o.Kind, err)
	}

	if _, err := w.Write(outbytes); err != nil {
		return trace.Errorf("unable to process the converted Kubernetes resource: %w", err)
	}
	return nil
}

func main() {
}
