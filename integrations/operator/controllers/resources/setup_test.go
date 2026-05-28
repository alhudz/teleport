/*
Copyright 2026 Gravitational, Inc.

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

package resources

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type fakeReconciler struct {
	reconcile.Reconciler
	gvk          schema.GroupVersionKind
	teleportKind string
	scoped       bool
	featureGate  controllers.CheckFeaturesFunc
}

func (f *fakeReconciler) SetupWithManager(mgr manager.Manager) error {
	return nil
}

func (f *fakeReconciler) GVK() schema.GroupVersionKind {
	return f.gvk
}

func (f *fakeReconciler) TeleportKind() string {
	return f.teleportKind
}

func (f *fakeReconciler) Scoped() bool {
	return f.scoped
}

func (f *fakeReconciler) CheckFeatures(features *proto.Features) bool {
	return f.featureGate(features)
}

// Factory is a ReconcilerFactory for the given fakeReconciler.
func (f *fakeReconciler) Factory(_ kclient.Client, _ *client.Client) (controllers.Reconciler, error) {
	return f, nil
}

func TestFilterEnabledReconcilers(t *testing.T) {
	log := logr.FromSlogHandler(logtest.NewLogger().Handler())
	// Test setup: create CRD fixtures and a fake kubernetes client to look them up
	installedGVK := schema.GroupVersionKind{Group: "resources.teleport.dev", Version: "v1", Kind: "Installed"}
	missingGVK := schema.GroupVersionKind{Group: "resources.teleport.dev", Version: "v1", Kind: "NotInstalled"}
	kubeClient := fake.NewFakeClient()
	crd := apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: installedGVK.Kind,
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: installedGVK.Group,
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:   "installeds",
				Singular: "installed",
				Kind:     installedGVK.Kind,
			},
		},
	}
	require.NoError(t, kubeClient.Create(t.Context(), &crd))

	// Test setup: create a list will various kinds of controllers so we can check which ones get selected.
	unscopedReconciler := &fakeReconciler{
		gvk: installedGVK,
		// note: we use teleport_kind
		teleportKind: "unscoped_reconciler",
		scoped:       false,
		featureGate:  controllers.AlwaysEnabled,
	}

	scopedReconciler := &fakeReconciler{
		gvk:         installedGVK,
		scoped:      true,
		featureGate: controllers.AlwaysEnabled,
	}

	missingReconciler := &fakeReconciler{
		gvk:         missingGVK,
		scoped:      false,
		featureGate: controllers.AlwaysEnabled,
	}

	missingScopedReconciler := &fakeReconciler{
		gvk:         missingGVK,
		scoped:      true,
		featureGate: controllers.AlwaysEnabled,
	}

	unscopedEnterpriseReconciler := &fakeReconciler{
		gvk:         installedGVK,
		scoped:      false,
		featureGate: controllers.RequireEnterprise,
	}

	scopedEnterpriseReconciler := &fakeReconciler{
		gvk:         installedGVK,
		scoped:      true,
		featureGate: controllers.RequireEnterprise,
	}

	reconcilers := []ReconcilerFactory{
		unscopedReconciler.Factory,
		scopedReconciler.Factory,
		missingReconciler.Factory,
		missingScopedReconciler.Factory,
		unscopedEnterpriseReconciler.Factory,
		scopedEnterpriseReconciler.Factory,
	}

	tests := []struct {
		name     string
		scoped   bool
		features *proto.Features
		expected []controllers.Reconciler
	}{
		{
			name:     "unscoped OSS",
			scoped:   false,
			features: &proto.Features{},
			expected: []controllers.Reconciler{
				unscopedReconciler,
				scopedReconciler,
			},
		},
		{
			name:     "scoped OSS",
			scoped:   true,
			features: &proto.Features{},
			expected: []controllers.Reconciler{
				scopedReconciler,
			},
		},
		{
			name:   "unscoped enterprise",
			scoped: false,
			features: &proto.Features{
				AdvancedAccessWorkflows: true,
			},
			expected: []controllers.Reconciler{
				unscopedReconciler,
				scopedReconciler,
				unscopedEnterpriseReconciler,
				scopedEnterpriseReconciler,
			},
		},
		{
			name:   "scoped enterprise",
			scoped: true,
			features: &proto.Features{
				AdvancedAccessWorkflows: true,
			},
			expected: []controllers.Reconciler{
				scopedReconciler,
				scopedEnterpriseReconciler,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test execution: filter reconciler and check that the picked ones are the ones we expect.
			result, err := filterEnabledReconcilers(
				Config{
					KubeClient: kubeClient,
					Log:        log,
					Scoped:     tt.scoped,
					Features:   tt.features,
				},
				reconcilers)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}
