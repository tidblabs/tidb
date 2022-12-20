// Copyright 2018 PingCAP, Inc.
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

package metrics

import (
	"strconv"
	"sync"

	"github.com/pingcap/log"
	"github.com/pingcap/tidb/util/logutil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	tikvmetrics "github.com/tikv/client-go/v2/metrics"
	"go.uber.org/zap"
)

var (
	// EnvRegisterMetricsAtInit flags that register metrics on init
	EnvRegisterMetricsAtInit = "true"

	// PanicCounter measures the count of panics.
	PanicCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tidb",
			Subsystem: "server",
			Name:      "panic_total",
			Help:      "Counter of panic.",
		}, []string{LblType})

	// MemoryUsage measures the usage gauge of memory.
	MemoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tidb",
			Subsystem: "server",
			Name:      "memory_usage",
			Help:      "Memory Usage",
		}, []string{LblModule, LblType})

	// IsRegisterMetricsAtInit default value is true, if we want to use tidb standby mode, it need to set system env EnvRegisterMetricsAtInit=false
	IsRegisterMetricsAtInit = getEnvIsMetricsRegisterAtInit()

	// InitializedCollector used to mark the metics collector are initialized or not
	InitializedCollector = false

	// Execute the default metrics initialization process
	_ = InitRegisterMetrics()
)

// metrics labels.
const (
	LabelSession   = "session"
	LabelDomain    = "domain"
	LabelDDLOwner  = "ddl-owner"
	LabelDDL       = "ddl"
	LabelDDLWorker = "ddl-worker"
	LabelDDLSyncer = "ddl-syncer"
	LabelGCWorker  = "gcworker"
	LabelAnalyze   = "analyze"

	LabelBatchRecvLoop = "batch-recv-loop"
	LabelBatchSendLoop = "batch-send-loop"

	opSucc   = "ok"
	opFailed = "err"

	TiDB         = "tidb"
	LabelScope   = "scope"
	ScopeGlobal  = "global"
	ScopeSession = "session"
	Server       = "server"
	TiKVClient   = "tikvclient"
)

// RetLabel returns "ok" when err == nil and "err" when err != nil.
// This could be useful when you need to observe the operation result.
func RetLabel(err error) string {
	if err == nil {
		return opSucc
	}
	return opFailed
}

func getEnvIsMetricsRegisterAtInit() bool {
	doMustRegister, err := strconv.ParseBool(EnvRegisterMetricsAtInit)
	if err != nil {
		log.Panic("getEnvIsMetricsRegisterAtInit strconv.ParseBool error.", zap.String("EnvRegisterMetricsAtInit", EnvRegisterMetricsAtInit), zap.Error(err))
	}
	log.Info("getEnvIsMetricsRegisterAtInit", zap.Bool("doMustRegister", doMustRegister))
	return doMustRegister
}

// RegisterCollector will register metrics collector in prometheus.
func RegisterCollector() {
	// use new go collector
	if !InitializedCollector {
		prometheus.DefaultRegisterer.Unregister(prometheus.NewGoCollector())
		prometheus.MustRegister(collectors.NewGoCollector(collectors.WithGoCollections(collectors.GoRuntimeMetricsCollection | collectors.GoRuntimeMemStatsCollection)))
		InitializedCollector = true
	}
}

// InitRegisterMetrics registers the metrics which are ONLY used in TiDB server.
func InitRegisterMetrics() bool {
	// use new go collector
	RegisterCollector()

	DefineMetrics()
	// If it's a `make gotest` or run a `go test` it's need to register at init, the `IsRegisterMetricsAtInit` is true.
	// If it's a real TiDB server and run in serverless cluster, it need to set the system env `export REGISTER_METRICS_INIT=false`,
	// and the metrics will register later when exit serverless standby mode.
	if IsRegisterMetricsAtInit {
		log.Info("register metrics when metrics init.")
		RegisterMetrics()
	}
	return true
}

// DefineMetrics is used to define metrics
func DefineMetrics() {
	DefineBindInfoMetrics()
	DefineDDLMetrics()
	DefineDistSQLMetrics()
	DefineDomainMetrics()
	DefineExecutorMetrics()
	DefineGCWorkerMetrics()
	DefineLogBackupMetrics()
	DefineMetaMetrics()
	DefineOwnerMetrics()
	DefineServerMetrics()
	DefineSessionMetrics()
	DefineSliMetrics()
	DefineStatsMetrics()
	DefineTopSQLMetrics()
}

// RegisterMetrics registers the metrics which are ONLY used in TiDB server.
func RegisterMetrics() {
	prometheus.MustRegister(AutoAnalyzeCounter)
	prometheus.MustRegister(AutoAnalyzeHistogram)
	prometheus.MustRegister(AutoIDHistogram)
	prometheus.MustRegister(BatchAddIdxHistogram)
	prometheus.MustRegister(BindUsageCounter)
	prometheus.MustRegister(BindTotalGauge)
	prometheus.MustRegister(BindMemoryUsage)
	prometheus.MustRegister(CampaignOwnerCounter)
	prometheus.MustRegister(ConnGauge)
	prometheus.MustRegister(DisconnectionCounter)
	prometheus.MustRegister(PreparedStmtGauge)
	prometheus.MustRegister(CriticalErrorCounter)
	prometheus.MustRegister(DDLCounter)
	prometheus.MustRegister(BackfillTotalCounter)
	prometheus.MustRegister(BackfillProgressGauge)
	prometheus.MustRegister(DDLWorkerHistogram)
	prometheus.MustRegister(DDLJobTableDuration)
	prometheus.MustRegister(DDLRunningJobCount)
	prometheus.MustRegister(DeploySyncerHistogram)
	prometheus.MustRegister(DistSQLPartialCountHistogram)
	prometheus.MustRegister(DistSQLCoprCacheCounter)
	prometheus.MustRegister(DistSQLCoprClosestReadCounter)
	prometheus.MustRegister(DistSQLCoprRespBodySize)
	prometheus.MustRegister(DistSQLQueryHistogram)
	prometheus.MustRegister(DistSQLScanKeysHistogram)
	prometheus.MustRegister(DistSQLScanKeysPartialHistogram)
	prometheus.MustRegister(DumpFeedbackCounter)
	prometheus.MustRegister(ExecuteErrorCounter)
	prometheus.MustRegister(ExecutorCounter)
	prometheus.MustRegister(GetTokenDurationHistogram)
	prometheus.MustRegister(HandShakeErrorCounter)
	prometheus.MustRegister(HandleJobHistogram)
	prometheus.MustRegister(SignificantFeedbackCounter)
	prometheus.MustRegister(FastAnalyzeHistogram)
	prometheus.MustRegister(SyncLoadCounter)
	prometheus.MustRegister(SyncLoadTimeoutCounter)
	prometheus.MustRegister(SyncLoadHistogram)
	prometheus.MustRegister(ReadStatsHistogram)
	prometheus.MustRegister(JobsGauge)
	prometheus.MustRegister(KeepAliveCounter)
	prometheus.MustRegister(LoadPrivilegeCounter)
	prometheus.MustRegister(InfoCacheCounters)
	prometheus.MustRegister(LoadSchemaCounter)
	prometheus.MustRegister(LoadSchemaDuration)
	prometheus.MustRegister(MetaHistogram)
	prometheus.MustRegister(NewSessionHistogram)
	prometheus.MustRegister(OwnerHandleSyncerHistogram)
	prometheus.MustRegister(PanicCounter)
	prometheus.MustRegister(PlanCacheCounter)
	prometheus.MustRegister(PlanCacheMissCounter)
	prometheus.MustRegister(PlanCacheInstanceMemoryUsage)
	prometheus.MustRegister(PlanCacheInstancePlanNumCounter)
	prometheus.MustRegister(PseudoEstimation)
	prometheus.MustRegister(PacketIOCounter)
	prometheus.MustRegister(QueryDurationHistogram)
	prometheus.MustRegister(QueryTotalCounter)
	prometheus.MustRegister(AffectedRowsCounter)
	prometheus.MustRegister(SchemaLeaseErrorCounter)
	prometheus.MustRegister(ServerEventCounter)
	prometheus.MustRegister(SessionExecuteCompileDuration)
	prometheus.MustRegister(SessionExecuteParseDuration)
	prometheus.MustRegister(SessionExecuteRunDuration)
	prometheus.MustRegister(SessionRestrictedSQLCounter)
	prometheus.MustRegister(SessionRetry)
	prometheus.MustRegister(SessionRetryErrorCounter)
	prometheus.MustRegister(StatementPerTransaction)
	prometheus.MustRegister(StatsInaccuracyRate)
	prometheus.MustRegister(StmtNodeCounter)
	prometheus.MustRegister(DbStmtNodeCounter)
	prometheus.MustRegister(ExecPhaseDuration)
	prometheus.MustRegister(StoreQueryFeedbackCounter)
	prometheus.MustRegister(TimeJumpBackCounter)
	prometheus.MustRegister(TransactionDuration)
	prometheus.MustRegister(StatementDeadlockDetectDuration)
	prometheus.MustRegister(StatementPessimisticRetryCount)
	prometheus.MustRegister(StatementLockKeysCount)
	prometheus.MustRegister(ValidateReadTSFromPDCount)
	prometheus.MustRegister(UpdateSelfVersionHistogram)
	prometheus.MustRegister(UpdateStatsCounter)
	prometheus.MustRegister(WatchOwnerCounter)
	prometheus.MustRegister(GCActionRegionResultCounter)
	prometheus.MustRegister(GCConfigGauge)
	prometheus.MustRegister(GCHistogram)
	prometheus.MustRegister(GCJobFailureCounter)
	prometheus.MustRegister(GCRegionTooManyLocksCounter)
	prometheus.MustRegister(GCWorkerCounter)
	prometheus.MustRegister(TotalQueryProcHistogram)
	prometheus.MustRegister(TotalCopProcHistogram)
	prometheus.MustRegister(TotalCopWaitHistogram)
	prometheus.MustRegister(HandleSchemaValidate)
	prometheus.MustRegister(MaxProcs)
	prometheus.MustRegister(GOGC)
	prometheus.MustRegister(ConnIdleDurationHistogram)
	prometheus.MustRegister(ServerInfo)
	prometheus.MustRegister(TokenGauge)
	prometheus.MustRegister(ConfigStatus)
	prometheus.MustRegister(TiFlashQueryTotalCounter)
	prometheus.MustRegister(SmallTxnWriteDuration)
	prometheus.MustRegister(TxnWriteThroughput)
	prometheus.MustRegister(LoadSysVarCacheCounter)
	prometheus.MustRegister(TopSQLIgnoredCounter)
	prometheus.MustRegister(TopSQLReportDurationHistogram)
	prometheus.MustRegister(TopSQLReportDataHistogram)
	prometheus.MustRegister(PDAPIExecutionHistogram)
	prometheus.MustRegister(PDAPIRequestCounter)
	prometheus.MustRegister(CPUProfileCounter)
	prometheus.MustRegister(ReadFromTableCacheCounter)
	prometheus.MustRegister(LoadTableCacheDurationHistogram)
	prometheus.MustRegister(NonTransactionalDMLCount)
	prometheus.MustRegister(MemoryUsage)
	prometheus.MustRegister(StatsCacheLRUCounter)
	prometheus.MustRegister(StatsCacheLRUGauge)
	prometheus.MustRegister(StatsHealthyGauge)
	prometheus.MustRegister(TxnStatusEnteringCounter)
	prometheus.MustRegister(TxnDurationHistogram)
	prometheus.MustRegister(LastCheckpoint)
	prometheus.MustRegister(AdvancerOwner)
	prometheus.MustRegister(AdvancerTickDuration)
	prometheus.MustRegister(GetCheckpointBatchSize)
	prometheus.MustRegister(RegionCheckpointRequest)
	prometheus.MustRegister(RegionCheckpointFailure)
	prometheus.MustRegister(AutoIDReqDuration)
	prometheus.MustRegister(RCCheckTSWriteConfilictCounter)

	tikvmetrics.InitMetrics(TiDB, TiKVClient)
	tikvmetrics.RegisterMetrics()
	tikvmetrics.TiKVPanicCounter = PanicCounter // reset tidb metrics for tikv metrics
}

var mode struct {
	sync.Mutex
	isSimplified bool
}

// ToggleSimplifiedMode is used to register/unregister the metrics that unused by grafana.
func ToggleSimplifiedMode(simplified bool) {
	var unusedMetricsByGrafana = []prometheus.Collector{
		StatementDeadlockDetectDuration,
		ValidateReadTSFromPDCount,
		LoadTableCacheDurationHistogram,
		TxnWriteThroughput,
		SmallTxnWriteDuration,
		InfoCacheCounters,
		ReadFromTableCacheCounter,
		TiFlashQueryTotalCounter,
		CampaignOwnerCounter,
		NonTransactionalDMLCount,
		MemoryUsage,
		TokenGauge,
		tikvmetrics.TiKVRawkvSizeHistogram,
		tikvmetrics.TiKVRawkvCmdHistogram,
		tikvmetrics.TiKVReadThroughput,
		tikvmetrics.TiKVSmallReadDuration,
		tikvmetrics.TiKVBatchWaitOverLoad,
		tikvmetrics.TiKVBatchClientRecycle,
		tikvmetrics.TiKVRequestRetryTimesHistogram,
		tikvmetrics.TiKVStatusDuration,
	}
	mode.Lock()
	defer mode.Unlock()
	if mode.isSimplified == simplified {
		return
	}
	mode.isSimplified = simplified
	if simplified {
		for _, m := range unusedMetricsByGrafana {
			prometheus.Unregister(m)
		}
	} else {
		for _, m := range unusedMetricsByGrafana {
			err := prometheus.Register(m)
			if err != nil {
				logutil.BgLogger().Error("cannot register metrics", zap.Error(err))
				break
			}
		}
	}
}
