/*
Copyright (c) 2019 VMware, Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package printer

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vmware/octant/internal/testutil"
	"github.com/vmware/octant/pkg/plugin"
	"github.com/vmware/octant/pkg/store"
	storefake "github.com/vmware/octant/pkg/store/fake"
	"github.com/vmware/octant/pkg/view/component"
	"github.com/vmware/octant/pkg/view/flexlayout"
)

func Test_ServiceAccountListHandler(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tpo := newTestPrinterOptions(controller)
	printOptions := tpo.ToOptions()

	labels := map[string]string{
		"foo": "bar",
	}

	now := testutil.Time()

	object := testutil.CreateServiceAccount("sa")
	object.CreationTimestamp = metav1.Time{Time: now}
	object.Labels = labels
	object.Secrets = []corev1.ObjectReference{{Name: "secret"}}

	tpo.PathForObject(object, object.Name, "/path")

	list := &corev1.ServiceAccountList{
		Items: []corev1.ServiceAccount{*object},
	}

	ctx := context.Background()
	got, err := ServiceAccountListHandler(ctx, list, printOptions)
	require.NoError(t, err)

	cols := component.NewTableCols("Name", "Labels", "Secrets", "Age")
	expected := component.NewTable("Service Accounts", "We couldn't find any service accounts!", cols)
	expected.Add(component.TableRow{
		"Name":    component.NewLink("", object.Name, "/path"),
		"Labels":  component.NewLabels(labels),
		"Secrets": component.NewText("1"),
		"Age":     component.NewTimestamp(now),
	})

	component.AssertEqual(t, expected, got)
}

func Test_serviceAccountHandler(t *testing.T) {
	ctx := context.Background()

	controller := gomock.NewController(t)
	defer controller.Finish()

	tpo := newTestPrinterOptions(controller)
	printOptions := tpo.ToOptions()

	now := testutil.Time()
	object := testutil.CreateServiceAccount("sa")
	object.CreationTimestamp = metav1.Time{Time: now}

	mockObjectsEvents(t, tpo.objectStore, object.Namespace)

	serviceAccountPrintResponse := &plugin.PrintResponse{}
	tpo.pluginManager.EXPECT().Print(ctx, object).Return(serviceAccountPrintResponse, nil)

	h, err := newServiceAccountHandler(ctx, object, printOptions)
	require.NoError(t, err)

	summaryConfig := component.NewSummary("config", component.SummarySection{
		Header: "foo", Content: component.NewText("bar")})

	h.configFunc = func(ctx context.Context, serviceAccount corev1.ServiceAccount, options Options) (*component.Summary, error) {
		return summaryConfig, nil
	}

	policyTable := component.NewTable("policyTable", "", component.NewTableCols("col1"))
	h.policyRulesFunc = func(ctx context.Context, serviceAccount corev1.ServiceAccount, appObjectStore store.Store) (*component.Table, error) {
		return policyTable, nil
	}

	got, err := h.run()
	require.NoError(t, err)

	fl := flexlayout.New()
	summarySection := fl.AddSection()
	require.NoError(t, summarySection.Add(summaryConfig, component.WidthHalf))

	stubMetadataForObject(t, object, fl)

	policyRulesSection := fl.AddSection()
	require.NoError(t, policyRulesSection.Add(policyTable, component.WidthFull))

	expected := fl.ToComponent("Summary")

	component.AssertEqual(t, expected, got)
}

func Test_printServiceAccountConfig(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tpo := newTestPrinterOptions(controller)
	printOptions := tpo.ToOptions()

	now := testutil.Time()

	object := testutil.CreateServiceAccount("sa")
	object.CreationTimestamp = metav1.Time{Time: now}
	object.Secrets = []corev1.ObjectReference{{Name: "secret"}}
	object.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "secret"}}

	key := store.Key{
		Namespace:  object.Namespace,
		APIVersion: "v1",
		Kind:       "Secret",
	}

	secret := testutil.CreateSecret("secret")
	secret.Type = corev1.SecretTypeServiceAccountToken
	secret.Annotations = map[string]string{
		corev1.ServiceAccountNameKey: object.Name,
		corev1.ServiceAccountUIDKey:  string(object.UID),
	}

	tpo.PathForGVK(object.Namespace, "v1", "Secret", "secret", "secret", "/secret")

	tpo.objectStore.EXPECT().List(gomock.Any(), gomock.Eq(key)).
		Return(testutil.ToUnstructuredList(t, testutil.ToUnstructured(t, secret)), false, nil)

	ctx := context.Background()
	got, err := printServiceAccountConfig(ctx, *object, printOptions)
	require.NoError(t, err)

	sections := component.SummarySections{}

	pullSecretsList := component.NewList("", []component.Component{
		component.NewLink("", "secret", "/secret"),
	})
	sections.Add("Image Pull Secrets", pullSecretsList)

	mountSecretsList := component.NewList("", []component.Component{
		component.NewLink("", "secret", "/secret"),
	})
	sections.Add("Mountable Secrets", mountSecretsList)

	tokenSecretsList := component.NewList("", []component.Component{
		component.NewLink("", "secret", "/secret"),
	})
	sections.Add("Tokens", tokenSecretsList)

	expected := component.NewSummary("Configuration", sections...)

	component.AssertEqual(t, expected, got)
}

func Test_serviceAccountPolicyRules(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	serviceAccount := testutil.CreateServiceAccount("sa")

	appObjectStore := storefake.NewMockStore(controller)

	roleBindingKey := store.Key{
		Namespace:  serviceAccount.Namespace,
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
	}

	role1 := testutil.CreateRole("role1")
	role2 := testutil.CreateClusterRole("role2")

	subjects1 := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      serviceAccount.Name,
			Namespace: serviceAccount.Namespace,
		},
	}

	roleBinding := testutil.CreateRoleBinding("rb1", role1.Name, subjects1)
	roleBindingObjects := testutil.ToUnstructuredList(t, roleBinding)

	appObjectStore.EXPECT().
		List(gomock.Any(), roleBindingKey).
		Return(roleBindingObjects, false, nil)

	clusterRoleBindingKey := store.Key{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRoleBinding",
	}

	subjects2 := []rbacv1.Subject{
		{
			Kind:     "Group",
			Name:     "system:serviceaccounts",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	clusterRoleBinding := testutil.CreateClusterRoleBinding("crb1", role2.Name, subjects2)
	clusterRoleBinding.RoleRef.Kind = "ClusterRole"
	clusterRoleBindingObjects := testutil.ToUnstructuredList(t, clusterRoleBinding)

	appObjectStore.EXPECT().
		List(gomock.Any(), clusterRoleBindingKey).
		Return(clusterRoleBindingObjects, false, nil)

	role1Key, err := store.KeyFromObject(role1)
	require.NoError(t, err)

	appObjectStore.EXPECT().
		Get(gomock.Any(), role1Key).
		Return(testutil.ToUnstructured(t, role1), true, nil)

	role2Key, err := store.KeyFromObject(role2)
	require.NoError(t, err)

	appObjectStore.EXPECT().
		Get(gomock.Any(), role2Key).
		Return(testutil.ToUnstructured(t, role2), true, nil)

	ctx := context.Background()

	s := newServiceAccountPolicyRules(ctx, *serviceAccount, appObjectStore)

	var policyRules []rbacv1.PolicyRule
	s.printPolicyRulesFunc = func(rules []rbacv1.PolicyRule) (*component.Table, error) {
		policyRules = rules
		return nil, nil
	}

	_, err = s.run()
	require.NoError(t, err)

	var expected []rbacv1.PolicyRule
	expected = append(expected, role1.Rules...)
	expected = append(expected, role2.Rules...)

	assert.Equal(t, expected, policyRules)
}
