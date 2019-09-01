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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vmware/octant/internal/testutil"
	"github.com/vmware/octant/pkg/store"
	storefake "github.com/vmware/octant/pkg/store/fake"
	"github.com/vmware/octant/pkg/view/component"
)

func Test_ReplicationControllerListHandler(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tpo := newTestPrinterOptions(controller)
	printOptions := tpo.ToOptions()

	validReplicationControllerLabels := map[string]string{
		"foo": "bar",
	}

	validReplicationControllerCreationTime := testutil.Time()

	validReplicationController := &corev1.ReplicationController{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ReplicationController",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rc-test",
			Namespace: "default",
			CreationTimestamp: metav1.Time{
				Time: validReplicationControllerCreationTime,
			},
			Labels: validReplicationControllerLabels,
		},
		Status: corev1.ReplicationControllerStatus{
			Replicas:          3,
			AvailableReplicas: 0,
		},
		Spec: corev1.ReplicationControllerSpec{
			Selector: map[string]string{
				"app": "myapp",
			},
			Template: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.15",
						},
					},
				},
			},
		},
	}

	validReplicationControllerList := &corev1.ReplicationControllerList{
		Items: []corev1.ReplicationController{
			*validReplicationController,
		},
	}

	tpo.PathForObject(validReplicationController, validReplicationController.Name, "/rc")

	ctx := context.Background()
	got, err := ReplicationControllerListHandler(ctx, validReplicationControllerList, printOptions)
	require.NoError(t, err)

	containers := component.NewContainers()
	containers.Add("nginx", "nginx:1.15")

	cols := component.NewTableCols("Name", "Labels", "Status", "Age", "Containers", "Selector")
	expected := component.NewTable("ReplicationControllers", "We couldn't find any replication controllers!", cols)
	expected.Add(component.TableRow{
		"Name":       component.NewLink("", "rc-test", "/rc"),
		"Labels":     component.NewLabels(validReplicationControllerLabels),
		"Status":     component.NewText("0/3"),
		"Age":        component.NewTimestamp(validReplicationControllerCreationTime),
		"Containers": containers,
		"Selector":   component.NewSelectors([]component.Selector{component.NewLabelSelector("app", "myapp")}),
	})

	component.AssertEqual(t, expected, got)
}

func TestReplicationControllerStatus(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	o := storefake.NewMockStore(controller)

	replicationController := testutil.CreateReplicationController("rc")
	replicationController.Labels = map[string]string{
		"foo": "bar",
	}
	replicationController.Spec.Selector = map[string]string{
		"foo": "bar",
	}
	replicationController.Namespace = "testing"

	p1 := *createPodWithPhase(
		"nginx-g7f72",
		replicationController.Labels,
		corev1.PodRunning,
		metav1.NewControllerRef(replicationController, replicationController.GroupVersionKind()))

	p2 := *createPodWithPhase(
		"nginx-p64jr",
		replicationController.Labels,
		corev1.PodRunning,
		metav1.NewControllerRef(replicationController, replicationController.GroupVersionKind()))

	p3 := *createPodWithPhase(
		"nginx-x8nrk",
		replicationController.Labels,
		corev1.PodRunning,
		metav1.NewControllerRef(replicationController, replicationController.GroupVersionKind()))

	pods := &corev1.PodList{
		Items: []corev1.Pod{p1, p2, p3},
	}

	podList := &unstructured.UnstructuredList{}
	for _, p := range pods.Items {
		podList.Items = append(podList.Items, *testutil.ToUnstructured(t, &p))
	}
	key := store.Key{
		Namespace:  "testing",
		APIVersion: "v1",
		Kind:       "Pod",
	}

	o.EXPECT().List(gomock.Any(), gomock.Eq(key)).Return(podList, false, nil)
	rcs := NewReplicationControllerStatus(replicationController)
	ctx := context.Background()
	got, err := rcs.Create(ctx, o)
	require.NoError(t, err)

	expected := component.NewQuadrant("Status")
	require.NoError(t, expected.Set(component.QuadNW, "Running", "3"))
	require.NoError(t, expected.Set(component.QuadNE, "Waiting", "0"))
	require.NoError(t, expected.Set(component.QuadSW, "Succeeded", "0"))
	require.NoError(t, expected.Set(component.QuadSE, "Failed", "0"))

	assert.Equal(t, expected, got)
}
