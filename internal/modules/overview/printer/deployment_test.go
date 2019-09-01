/*
Copyright (c) 2019 VMware, Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package printer

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/vmware/octant/internal/conversion"
	"github.com/vmware/octant/internal/testutil"
	"github.com/vmware/octant/pkg/store"
	"github.com/vmware/octant/pkg/view/component"
)

func Test_DeploymentListHandler(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tpo := newTestPrinterOptions(controller)
	printOptions := tpo.ToOptions()

	objectLabels := map[string]string{
		"foo": "bar",
	}

	now := testutil.Time()

	object := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment",
			Namespace: "default",
			CreationTimestamp: metav1.Time{
				Time: now,
			},
			Labels: objectLabels,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            3,
			AvailableReplicas:   2,
			UnavailableReplicas: 1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: conversion.PtrInt32(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "my_app",
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.15",
						},
						{
							Name:  "kuard",
							Image: "gcr.io/kuar-demo/kuard-amd64:1",
						},
					},
				},
			},
		},
	}

	tpo.PathForObject(object, object.Name, "/path")

	list := &appsv1.DeploymentList{
		Items: []appsv1.Deployment{*object},
	}

	ctx := context.Background()
	got, err := DeploymentListHandler(ctx, list, printOptions)
	require.NoError(t, err)

	containers := component.NewContainers()
	containers.Add("nginx", "nginx:1.15")
	containers.Add("kuard", "gcr.io/kuar-demo/kuard-amd64:1")

	cols := component.NewTableCols("Name", "Labels", "Status", "Age", "Containers", "Selector")
	expected := component.NewTable("Deployments", "We couldn't find any deployments!", cols)
	expected.Add(component.TableRow{
		"Name":       component.NewLink("", "deployment", "/path"),
		"Labels":     component.NewLabels(objectLabels),
		"Age":        component.NewTimestamp(now),
		"Selector":   component.NewSelectors([]component.Selector{component.NewLabelSelector("app", "my_app")}),
		"Status":     component.NewText("2/3"),
		"Containers": containers,
	})

	component.AssertEqual(t, expected, got)
}

func Test_deploymentConfiguration(t *testing.T) {
	var rhl int32 = 5
	validDeployment := testutil.CreateDeployment("deployment")
	validDeployment.Spec = appsv1.DeploymentSpec{
		Replicas:             conversion.PtrInt32(3),
		RevisionHistoryLimit: &rhl,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "my_app",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "key",
					Operator: "In",
					Values:   []string{"value1", "value2"},
				},
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
			},
		},
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.15",
					},
					{
						Name:  "kuard",
						Image: "gcr.io/kuar-demo/kuard-amd64:1",
					},
				},
			},
		},
	}

	cases := []struct {
		name       string
		deployment *appsv1.Deployment
		isErr      bool
		expected   *component.Summary
	}{
		{
			name:       "deployment",
			deployment: validDeployment,
			expected: component.NewSummary("Configuration", []component.SummarySection{
				{
					Header:  "Deployment Strategy",
					Content: component.NewText("RollingUpdate"),
				},
				{
					Header:  "Rolling Update Strategy",
					Content: component.NewText("Max Surge 25%, Max Unavailable 25%"),
				},
				{
					Header: "Selectors",
					Content: component.NewSelectors(
						[]component.Selector{
							component.NewExpressionSelector("key", component.OperatorIn, []string{"value1", "value2"}),
							component.NewLabelSelector("app", "my_app"),
						},
					),
				},
				{
					Header:  "Min Ready Seconds",
					Content: component.NewText("0"),
				},
				{
					Header:  "Revision History Limit",
					Content: component.NewText("5"),
				},
				{
					Header:  "Replicas",
					Content: component.NewText("3"),
				},
			}...),
		},
		{
			name:       "deployment is nil",
			deployment: nil,
			isErr:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dc := NewDeploymentConfiguration(tc.deployment)
			dc.actionGenerators = []actionGeneratorFunction{}

			summary, err := dc.Create()
			if tc.isErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			component.AssertEqual(t, tc.expected, summary)
		})
	}
}

func Test_deploymentStatus(t *testing.T) {
	deployment := &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			UpdatedReplicas:     1,
			Replicas:            2,
			UnavailableReplicas: 3,
			AvailableReplicas:   4,
			ReadyReplicas:       5,
		},
	}

	got, err := deploymentStatus(deployment)
	require.NoError(t, err)

	expected := component.NewSummary("Status", []component.SummarySection{
		{
			Header:  "Available Replicas",
			Content: component.NewText("4"),
		},
		{
			Header:  "Ready Replicas",
			Content: component.NewText("5"),
		},
		{
			Header:  "Total Replicas",
			Content: component.NewText("2"),
		},
		{
			Header:  "Unavailable Replicas",
			Content: component.NewText("3"),
		},
		{
			Header:  "Updated Replicas",
			Content: component.NewText("1"),
		},
	}...)

	component.AssertEqual(t, expected, got)
}

func Test_deploymentConditions(t *testing.T) {
	now := time.Now()

	deployment := &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:               "type1",
					Status:             "status1",
					LastUpdateTime:     metav1.Time{Time: now},
					LastTransitionTime: metav1.Time{Time: now},
					Reason:             "reason1",
					Message:            "message1",
				},
				{
					Type:               "type2",
					Status:             "status2",
					LastUpdateTime:     metav1.Time{Time: now},
					LastTransitionTime: metav1.Time{Time: now},
					Reason:             "reason2",
					Message:            "message2",
				},
			},
		},
	}

	got, err := deploymentConditions(deployment)
	require.NoError(t, err)

	expected := component.NewTableWithRows("Conditions", "There are no deployment conditions!", deploymentConditionColumns, []component.TableRow{
		{
			"Type":            component.NewText("type1"),
			"Reason":          component.NewText("reason1"),
			"Status":          component.NewText("status1"),
			"Message":         component.NewText("message1"),
			"Last Update":     component.NewTimestamp(now),
			"Last Transition": component.NewTimestamp(now),
		},
		{
			"Type":            component.NewText("type2"),
			"Reason":          component.NewText("reason2"),
			"Status":          component.NewText("status2"),
			"Message":         component.NewText("message2"),
			"Last Update":     component.NewTimestamp(now),
			"Last Transition": component.NewTimestamp(now),
		},
	})

	component.AssertEqual(t, expected, got)
}

func Test_deploymentPods(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tpo := newTestPrinterOptions(controller)
	printOptions := tpo.ToOptions()

	podLabels := map[string]string{
		"foo": "bar",
	}

	deployment := testutil.CreateDeployment("deployment")
	deployment.Spec.Template.ObjectMeta.Labels = podLabels

	now := testutil.Time()
	pod := testutil.CreatePod("pod")
	pod.ObjectMeta.CreationTimestamp = metav1.Time{Time: now}

	tpo.PathForObject(pod, pod.Name, "/pod")

	selector := labels.Set(podLabels)
	key := store.Key{
		Namespace:  "namespace",
		APIVersion: "v1",
		Kind:       "Pod",
		Selector:   &selector,
	}
	tpo.objectStore.EXPECT().
		List(gomock.Any(), key).
		Return(testutil.ToUnstructuredList(t, pod), false, nil)

	ctx := context.Background()

	got, err := deploymentPods(ctx, deployment, printOptions)
	require.NoError(t, err)

	expected := component.NewTableWithRows("Pods", "We couldn't find any pods!", podColsWithOutLabels, []component.TableRow{
		{
			"Name":     component.NewLink("", pod.Name, "/pod"),
			"Age":      component.NewTimestamp(now),
			"Ready":    component.NewText("0/0"),
			"Restarts": component.NewText("0"),
			"Phase":    component.NewText(""),
			"Node":     component.NewText("<not scheduled>"),
		},
	})

	component.AssertEqual(t, expected, got)
}

func Test_editDeploymentAction(t *testing.T) {
	deployment := testutil.CreateDeployment("deployment")
	deployment.Spec.Replicas = pointer.Int32Ptr(3)

	actions := editDeploymentAction(deployment)
	assert.Len(t, actions, 1)

	got := actions[0]

	gvk := deployment.GroupVersionKind()

	expected := component.Action{
		Name:  "Edit",
		Title: "Deployment Editor",
		Form: component.Form{
			Fields: []component.FormField{
				component.NewFormFieldNumber("Replicas", "replicas", "3"),
				component.NewFormFieldHidden("group", gvk.Group),
				component.NewFormFieldHidden("version", gvk.Version),
				component.NewFormFieldHidden("kind", gvk.Kind),
				component.NewFormFieldHidden("name", deployment.Name),
				component.NewFormFieldHidden("namespace", deployment.Namespace),
				component.NewFormFieldHidden("action", "deployment/configuration"),
			},
		},
	}

	assert.Equal(t, expected, got)
}
