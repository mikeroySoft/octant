/*
Copyright (c) 2019 VMware, Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package configuration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/require"

	configFake "github.com/vmware/octant/internal/config/fake"
	"github.com/vmware/octant/internal/describer"
	dashPlugin "github.com/vmware/octant/pkg/plugin"
	"github.com/vmware/octant/pkg/plugin/fake"
	pluginFake "github.com/vmware/octant/pkg/plugin/fake"
	"github.com/vmware/octant/pkg/view/component"
)

func TestPluginDescriber(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	name := "plugin-test"
	namespace := "default"
	metadata := &dashPlugin.Metadata{
		Name:         name,
		Description:  "this is a test",
		Capabilities: dashPlugin.Capabilities{},
	}

	store := dashPlugin.NewDefaultStore()
	client := newFakePluginClient(name, controller)
	require.NoError(t, store.Store(name, client, metadata, "cmd"))

	pluginManager := pluginFake.NewMockManagerInterface(controller)
	pluginManager.EXPECT().Store().Return(store).AnyTimes()

	dashConfig := configFake.NewMockDash(controller)
	dashConfig.EXPECT().PluginManager().Return(pluginManager)

	p := NewPluginListDescriber()

	options := describer.Options{
		Dash: dashConfig,
	}

	ctx := context.Background()
	cResponse, err := p.Describe(ctx, "/plugins", namespace, options)
	require.NoError(t, err)

	capabilities := dashPlugin.Capabilities{}
	capabilitiesData, err := json.Marshal(capabilities)
	require.NoError(t, err)

	list := component.NewList("Plugins", nil)
	tableCols := component.NewTableCols("Name", "Description", "Capabilities")
	table := component.NewTable("Plugins", tableCols)
	table.Add(component.TableRow{
		"Name":        component.NewText(name),
		"Description": component.NewText("this is a test"),
		"Capability":  component.NewText(string(capabilitiesData)),
	})

	list.Add(table)

	require.Len(t, cResponse.Components, 1)
	component.AssertEqual(t, list, cResponse.Components[0])
}

func newFakePluginClient(name string, controller *gomock.Controller) *fakePluginClient {
	service := fake.NewMockService(controller)
	metadata := dashPlugin.Metadata{
		Name: name,
	}
	service.EXPECT().Register(gomock.Any(), gomock.Eq("localhost:54321")).Return(metadata, nil).AnyTimes()

	clientProtocol := fake.NewMockClientProtocol(controller)
	clientProtocol.EXPECT().Dispense("plugin").Return(service, nil).AnyTimes()

	return &fakePluginClient{
		service:        service,
		clientProtocol: clientProtocol,
		name:           name,
	}
}

type fakePluginClient struct {
	clientProtocol *fake.MockClientProtocol
	service        *fake.MockService
	name           string
}

var _ dashPlugin.Client = (*fakePluginClient)(nil)

func (c *fakePluginClient) Client() (plugin.ClientProtocol, error) {
	return c.clientProtocol, nil
}

func (c *fakePluginClient) Kill() {}
