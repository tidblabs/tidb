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
	"github.com/ngaut/pools"
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/util/logutil"
	"go.uber.org/atomic"
)

// workerPool is used to new worker.
type workerPool struct {
	exit    atomic.Bool
	resPool *pools.ResourcePool
}

func newDDLWorkerPool(resPool *pools.ResourcePool) *workerPool {
	return &workerPool{
		exit:    *atomic.NewBool(false),
		resPool: resPool,
	}
}

// get gets workerPool from context resource pool.
// Please remember to call put after you finished using workerPool.
func (sg *workerPool) get() (*worker, error) {
	if sg.resPool == nil {
		return nil, nil
	}

	if sg.exit.Load() {
		return nil, errors.Errorf("workerPool is closed.")
	}

	// no need to protect sg.resPool
	resource, err := sg.resPool.Get()
	if err != nil {
		return nil, errors.Trace(err)
	}

	worker := resource.(*worker)
	return worker, nil
}

// put returns workerPool to context resource pool.
func (sg *workerPool) put(wk *worker) {
	if sg.resPool == nil {
		return
	}

	// no need to protect sg.resPool, even the sg.resPool is closed, the ctx still need to
	// put into resPool, because when resPool is closing, it will wait all the ctx returns, then resPool finish closing.
	sg.resPool.Put(wk)
}

// close clean up the workerPool.
func (sg *workerPool) close() {
	// prevent closing resPool twice.
	if sg.exit.Load() || sg.resPool == nil {
		return
	}
	sg.exit.Store(true)
	logutil.BgLogger().Info("[ddl] closing workerPool")
	sg.resPool.Close()
}