/*
Copyright (c) 2019 VMware, Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package printer

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/vmware/octant/pkg/view/component"
)

var (
	secretTableCols = component.NewTableCols("Name", "Labels", "Type", "Data", "Age")
	secretDataCols  = component.NewTableCols("Key")
)

// SecretListHandler is a printFunc that lists secrets.
func SecretListHandler(ctx context.Context, list *corev1.SecretList, options Options) (component.Component, error) {
	if list == nil {
		return nil, errors.New("list of secrets is nil")
	}

	table := component.NewTable("Secrets", "We couldn't find any secrets!", secretTableCols)

	for _, secret := range list.Items {
		row := component.TableRow{}
		nameLink, err := options.Link.ForObject(&secret, secret.Name)
		if err != nil {
			return nil, err
		}

		row["Name"] = nameLink

		row["Labels"] = component.NewLabels(secret.ObjectMeta.Labels)
		row["Type"] = component.NewText(string(secret.Type))
		row["Data"] = component.NewText(fmt.Sprintf("%d", len(secret.Data)))
		row["Age"] = component.NewTimestamp(secret.ObjectMeta.CreationTimestamp.Time)

		table.Add(row)
	}

	return table, nil
}

// SecretHandler is a printFunc for printing a secret summary.
func SecretHandler(ctx context.Context, secret *corev1.Secret, options Options) (component.Component, error) {
	if secret == nil {
		return nil, errors.New("secret is nil")
	}

	o := NewObject(secret)

	configSummary, err := secretConfiguration(*secret)
	if err != nil {
		return nil, err
	}

	o.RegisterConfig(configSummary)

	o.RegisterItems(ItemDescriptor{
		Func: func() (component.Component, error) {
			return secretData(*secret)
		},
		Width: component.WidthFull,
	})

	return o.ToComponent(ctx, options)
}

func secretConfiguration(secret corev1.Secret) (*component.Summary, error) {
	var sections []component.SummarySection

	sections = append(sections, component.SummarySection{
		Header:  "Type",
		Content: component.NewText(string(secret.Type)),
	})

	summary := component.NewSummary("Configuration", sections...)
	return summary, nil
}

func secretData(secret corev1.Secret) (*component.Table, error) {
	table := component.NewTable("Data", "This secret has no data!", secretDataCols)

	for key := range secret.Data {
		row := component.TableRow{}
		row["Key"] = component.NewText(key)

		table.Add(row)
	}

	return table, nil
}
