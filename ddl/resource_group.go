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

package ddl

import (
	"context"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/domain/infosync"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/parser/model"
	"github.com/pingcap/tidb/util/dbterror"
	"github.com/pingcap/tidb/util/logutil"
	"go.uber.org/zap"
)

func onCreateOrAlterResourceGroup(d *ddlCtx, t *meta.Meta, job *model.Job) (ver int64, _ error) {
	groupInfo := &model.ResourceGroupInfo{}
	if err := job.DecodeArgs(groupInfo); err != nil {
		job.State = model.JobStateCancelled
		return ver, errors.Trace(err)
	}
	groupInfo.State = model.StateNone

	// TODO: check if the policy exists.
	// err := checkPlacementPolicyNotExistAndCancelExistJob(d, t, job, policyInfo)
	// if err != nil {
	// 	return ver, errors.Trace(err)
	// }

	// TODO: add resource group with ResourGroup Manager API.

	switch groupInfo.State {
	case model.StateNone:
		// none -> public
		groupInfo.State = model.StatePublic
		// TODO: save resource group to PD.
		err := t.CreateOrUpdateResourceGroup(groupInfo)
		if err != nil {
			return ver, errors.Trace(err)
		}
		job.SchemaID = groupInfo.ID
		err = infosync.PutResourceGroup(context.TODO(), infosync.ConvertAPIResourceGroup(groupInfo))
		if err != nil {
			logutil.BgLogger().Error("put resource group to etcd failed", zap.Error(err))
			job.State = model.JobStateCancelled
			return ver, errors.Trace(err)
		}
		ver, err = updateSchemaVersion(d, t, job)
		if err != nil {
			return ver, errors.Trace(err)
		}
		// Finish this job.
		job.FinishDBJob(model.JobStateDone, model.StatePublic, ver, nil)
		return ver, nil
	default:
		// We can't enter here.
		return ver, dbterror.ErrInvalidDDLState.GenWithStackByArgs("resource_group", groupInfo.State)
	}
}
