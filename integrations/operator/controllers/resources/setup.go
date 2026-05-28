/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resources

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/gravitational/trace"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/integrations/operator/controllers"
)

// ReconcilerFactory is a function that creates a reconciler from Kubernetes and Teleport clients.
type ReconcilerFactory func(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error)

// Add new reconcilers here.
var supportedReconcilers = []ReconcilerFactory{
	NewAccessListReconciler,
	NewAccessMonitoringRuleV1Reconciler,
	NewAppV3Reconciler,
	NewAutoUpdateConfigV1Reconciler,
	NewAutoUpdateVersionV1Reconciler,
	NewBotV1Reconciler,
	NewDatabaseV3Reconciler,
	NewGithubConnectorReconciler,
	NewInferenceModelReconciler,
	NewInferencePolicyReconciler,
	NewInferenceSecretReconciler,
	NewLockV2Reconciler,
	NewLoginRuleReconciler,
	NewOIDCConnectorReconciler,
	NewOktaImportRuleReconciler,
	NewOpenSSHEICEServerV2Reconciler,
	NewOpenSSHServerV2Reconciler,
	NewProvisionTokenReconciler,
	NewRetrievalModelV1Reconciler,
	NewRoleReconciler,
	NewRoleV6Reconciler,
	NewRoleV7Reconciler,
	NewRoleV8Reconciler,
	NewSAMLConnectorReconciler,
	NewSAMLIdPServiceProviderV1Reconciler,
	NewScopedRoleV1Reconciler,
	NewScopedRoleAssignmentV1Reconciler,
	NewScopedTokenV1Reconciler,
	NewTrustedClusterV2Reconciler,
	NewUserReconciler,
	NewWorkloadIdentityV1Reconciler,
}

// SetupAllControllers sets up all controllers.
// A reconciler is enabled if:
// - its CRD exists in the clusters (supports a newer operator running against odler CRDs)
// - the operator is not running in scoped mode OR the operator is in scoped mode and the reconciler is scoped.
// - the reconciler support the cluster features (e.g. don't start a enterprise reconciler against an OSS cluster)
func SetupAllControllers(config Config, mgr manager.Manager) error {
	reconcilers, err := filterEnabledReconcilers(config, supportedReconcilers)
	if err != nil {
		return trace.Wrap(err)
	}

	// Setup all enabled reconcilers.
	if len(reconcilers) == 0 {
		return trace.NotFound("No reconciler enabled, this is likely a mistake")
	}
	for _, reconciler := range reconcilers {
		if err := reconciler.SetupWithManager(mgr); err != nil {
			return trace.Wrap(err, "failed to setup controller for: %s", reconciler.GVK().Kind)
		}
		config.Log.Info("Reconciler setup successfully", "kubernetes_kind", reconciler.GVK().Kind, "teleport_kind", reconciler.TeleportKind())
	}

	return nil
}

// Config contains the configuration required to setup the resource reconcilers.
type Config struct {
	// Log is the logger used to send logs regarding the controller setup.
	// The controllers themselves use the logger from the query context.
	Log logr.Logger
	// TeleportClient is passed to controllers so they can interact with the Teleport cluster.
	TeleportClient *client.Client
	// KubeClient is used by the setup process to detect which CRDs are deployed in the cluster.
	// This is also passed to the controllers so they can get resources and write their status back.
	KubeClient kclient.Client
	// Scoped indicates that the operator is running in scoped mode.
	Scoped bool
	// Features are the features advertised by the Teleport cluster.
	// This is used to know which reconcilers should be started.
	Features *proto.Features
}

func filterEnabledReconcilers(c Config, reconcilers []ReconcilerFactory) ([]controllers.Reconciler, error) {
	// list CRDs deployed in the cluster
	var existingCRDs apiextv1.CustomResourceDefinitionList
	if err := c.KubeClient.List(context.TODO(), &existingCRDs); err != nil {
		return nil, trace.Wrap(err, "listing existing CRDs")
	}

	var enabledReconcilers []controllers.Reconciler
	crds := make(map[string]struct{})
	for _, crd := range existingCRDs.Items {
		crds[crd.Name] = struct{}{}
	}

	// Check which reconcilers can and should be enabled.
	for i, factory := range reconcilers {
		reconciler, err := factory(c.KubeClient, c.TeleportClient)
		if err != nil {
			return nil, trace.Wrap(err, "creating reconciler", "index", i)
		}
		if _, ok := crds[reconciler.GVK().Kind]; !ok {
			c.Log.Info("CRD %q not deployed in the cluster, reconciler skipped")
			continue
		}
		if c.Scoped && !reconciler.Scoped() {
			if _, ok := crds[reconciler.GVK().Kind]; !ok {
				c.Log.Info("CRD %q deployed but operator running in scoped mode, CRD will not be reconciled")
			}
			continue
		}
		if !reconciler.CheckFeatures(c.Features) {
			continue
		}
		enabledReconcilers = append(enabledReconcilers, reconciler)
	}
	return enabledReconcilers, nil
}
