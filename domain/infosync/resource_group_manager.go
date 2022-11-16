// Copyright 2022 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions andasd
// limitations under the License.

package infosync

import (
	"bytes"
	"context"
	"encoding/json"
	"path"
	"sync"

	"github.com/pingcap/tidb/parser/model"
	"github.com/pingcap/tidb/util/pdapi"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ResourceGroup is used to conmunicate with ResourGroup Manager.
type ResourceGroup struct {
	Name             string `json:"name"`
	CPU              string `json:"cpu"`
	IOBandwidth      string `json:"io_bandwidth"`
	IOReadBandwidth  string `json:"io_read_bandwidth"`
	IOWriteBandwidth string `json:"io_write_bandwidth"`
}

func ConvertAPIResourceGroup(modelGroup *model.ResourceGroupInfo) *ResourceGroup {
	return &ResourceGroup{
		Name:             modelGroup.Name.L,
		CPU:              modelGroup.CPULimiter,
		IOBandwidth:      modelGroup.IOBandwidth,
		IOReadBandwidth:  modelGroup.IOReadBandwidth,
		IOWriteBandwidth: modelGroup.IOWriteBandwidth,
	}
}

// ResourceGroupManager manages resource group settings
type ResourceGroupManager interface {
	// GetResourceGroup is used to get one specific rule bundle from ResourGroup Manager.
	GetResourceGroup(ctx context.Context, name string) (*ResourceGroup, error)
	// GetAllResourceGroups is used to get all rule bundles from ResourGroup Manager. It is used to load full rules from PD while fullload infoschema.
	GetAllResourceGroups(ctx context.Context) ([]*ResourceGroup, error)
	// PutResourceGroup is used to post specific rule bundles to ResourGroup Manager.
	PutResourceGroup(ctx context.Context, group *ResourceGroup) error
}

// ExtenalResourceGroupManager manages placement with pd
type ExtenalResourceGroupManager struct {
	etcdCli *clientv3.Client
}

// GetResourceGroup is used to get one specific rule bundle from ResourGroup Manager.
func (m *ExtenalResourceGroupManager) GetResourceGroup(ctx context.Context, name string) (*ResourceGroup, error) {
	group := &ResourceGroup{Name: name}
	res, err := doRequest(ctx, "GetResourceGroup", m.etcdCli.Endpoints(), path.Join(pdapi.ResourceGroupConfig, "group", name), "GET", nil)
	if err == nil && res != nil {
		err = json.Unmarshal(res, group)
	}
	return group, err
}

// GetAllResourceGroups is used to get all resource group from ResourGroup Manager. It is used to load full resource groups from PD while fullload infoschema.
func (m *ExtenalResourceGroupManager) GetAllResourceGroups(ctx context.Context) ([]*ResourceGroup, error) {
	res, err := doRequest(ctx, "GetAllResourceGroups", m.etcdCli.Endpoints(), path.Join(pdapi.ResourceGroupConfig, "groups"), "GET", nil)
	if err != nil {
		return nil, err
	}
	var groups []*ResourceGroup
	err = json.Unmarshal(res, &groups)
	return groups, err
}

// PutResourceGroup is used to post specific resource group to ResourGroup Manager.
func (m *ExtenalResourceGroupManager) PutResourceGroup(ctx context.Context, group *ResourceGroup) error {
	data, err := json.Marshal(group)
	if err != nil {
		return err
	}
	_, err = doRequest(ctx, "PutResourceGroup", m.etcdCli.Endpoints(), path.Join(pdapi.ResourceGroupConfig, "group"), "POST", bytes.NewReader(data))
	return err
}

type mockResourceGroupManager struct {
	sync.Mutex
	groups map[string]*ResourceGroup
}

func (m *mockResourceGroupManager) GetResourceGroup(ctx context.Context, name string) (*ResourceGroup, error) {
	m.Lock()
	defer m.Unlock()
	group, ok := m.groups[name]
	if !ok {
		return nil, nil
	}
	return group, nil
}

func (m *mockResourceGroupManager) GetAllResourceGroups(ctx context.Context) ([]*ResourceGroup, error) {
	m.Lock()
	defer m.Unlock()
	var groups []*ResourceGroup
	for _, group := range m.groups {
		groups = append(groups, group)
	}
	return groups, nil
}

func (m *mockResourceGroupManager) PutResourceGroup(ctx context.Context, group *ResourceGroup) error {
	m.Lock()
	defer m.Unlock()
	m.groups[group.Name] = group
	return nil
}
