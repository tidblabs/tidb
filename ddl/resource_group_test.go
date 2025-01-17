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
// See the License for the specific language governing permissions and
// limitations under the License.

package ddl_test

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/pingcap/tidb/ddl"
	"github.com/pingcap/tidb/domain"
	"github.com/pingcap/tidb/parser/model"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/testkit"
	"github.com/stretchr/testify/require"
)

func TestResourceGroup(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	// clearAllBundles(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")

	hook := &ddl.TestDDLCallback{Do: dom}
	var groupID int64
	onJobUpdatedExportedFunc := func(job *model.Job) {
		debug.PrintStack()
		if groupID != 0 {
			return
		}
		// job.SchemaID will be assigned when the policy is created.
		if job.SchemaName == "x" && job.Type == model.ActionCreateResourceGroup && job.SchemaID != 0 {
			groupID = job.SchemaID
			return
		}
	}
	hook.OnJobUpdatedExported.Store(&onJobUpdatedExportedFunc)
	dom.DDL().SetHook(hook)

	tk.MustExec("create resource group x " +
		"CPU='12c' " +
		"IO_BANDWIDTH='1000MB/s'")

	fmt.Println("groupID====", groupID)

	checkFunc := func(groupInfo *model.ResourceGroupInfo) {
		require.Equal(t, true, groupInfo.ID != 0)
		require.Equal(t, "x", groupInfo.Name.L)
		require.Equal(t, groupID, groupInfo.ID)
		require.Equal(t, "12c", groupInfo.CPULimiter)
		require.Equal(t, "1000MB/s", groupInfo.IOBandwidth)
	}

	// Check the policy is correctly reloaded in the information schema.
	g := testResourceGroupNameFromIS(t, tk.Session(), "x")
	checkFunc(g)
}

func testResourceGroupNameFromIS(t *testing.T, ctx sessionctx.Context, name string) *model.ResourceGroupInfo {
	dom := domain.GetDomain(ctx)
	// Make sure the table schema is the new schema.
	err := dom.Reload()
	require.NoError(t, err)
	g, ok := dom.InfoSchema().ResourceGroupByName(model.NewCIStr(name))
	require.Equal(t, true, ok)
	return g
}
