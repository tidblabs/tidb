package session

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"runtime/debug"
	"strings"
	"text/template"
	"time"

	"github.com/pingcap/tidb/config"
	"github.com/pingcap/tidb/domain"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/util/logutil"
	"github.com/tikv/client-go/v2/oracle"
	"go.uber.org/zap"
)

var mBootstrapSQLKey = []byte("BootstrapSQLKey")

func fatalLogSQL(msg string, err error, sql string) {
	debug.PrintStack()
	logutil.BgLogger().Fatal(msg, zap.Error(err), zap.String("sql", sql))
}

func fatalLog(msg string, err error) {
	debug.PrintStack()
	logutil.BgLogger().Fatal(msg, zap.Error(err))
}

func getBootstrapSQLTimestamp(store kv.Storage) uint64 {
	var ver int64
	// check in kv store
	ctx := kv.WithInternalSourceType(context.Background(), kv.InternalTxnBootstrap)
	if err := kv.RunInNewTxn(ctx, store, false,
		func(ctx context.Context, txn kv.Transaction) error {
			var err error
			ver, err = meta.NewMeta(txn).GetInt64Key(mBootstrapSQLKey)
			return err
		}); err != nil {
		logutil.BgLogger().Fatal("Failed to check bootstrap sql",
			zap.Error(err))
	}
	return uint64(ver)
}

func doBootstrapSQL(s Session, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		if sql := strings.TrimSpace(scanner.Text()); len(sql) > 0 && !strings.HasPrefix(sql, "#") {
			ctx := kv.WithInternalSourceType(context.Background(), kv.InternalTxnBootstrap)
			if rs, err := s.ExecuteInternal(ctx, sql); err != nil {
				fatalLogSQL("Failed to execute", err, sql)
			} else if rs != nil {
				rs.Close()
			}
		}
	}
}

func runBootstrapSQL(s Session, params map[string]string) bool {
	startTime := time.Now()
	dom := domain.GetDomain(s)

	executed := false
	for {
		tpl, err := template.ParseFiles(config.GetGlobalConfig().BootstrapSQLFile)
		if err != nil {
			fatalLog("Failed to open bootstrap sql file", err)
		}

		var buf bytes.Buffer
		err = tpl.Execute(&buf, params)
		if err != nil {
			fatalLog("Failed to execute template", err)
		}

		if ts := getBootstrapSQLTimestamp(s.GetStore()); ts != notBootstrapped {
			logutil.BgLogger().Info("Bootstrap SQL executed successfully by other TiDB server",
				zap.Duration("take time", time.Since(startTime)),
				zap.Time("at", oracle.GetTimeFromTS(ts)))
			break
		}

		// To reduce conflict when multiple TiDB-server start at the same time.
		// Actually only one server need to execute bootstrap sql. So we chose DDL owner to do this.
		if dom.DDL().OwnerManager().IsOwner() {
			doBootstrapSQL(s, &buf)
			logutil.BgLogger().Info("Bootstrap SQL executed successfully",
				zap.Duration("take time", time.Since(startTime)))
			executed = true
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	return executed
}

// RunBootstrapSQL loads the bootstrap sql file to database and execute it.
func RunBootstrapSQL(storage kv.Storage) {
	cfg := config.GetGlobalConfig()
	if len(cfg.BootstrapSQLFile) == 0 {
		return
	}

	if ts := getBootstrapSQLTimestamp(storage); ts != notBootstrapped {
		logutil.BgLogger().Info("Bootstrap SQL already executed",
			zap.Time("at", oracle.GetTimeFromTS(uint64(ts))))
		return
	}

	s, err := CreateSession(storage)
	if err != nil {
		// Bootstrap SQL fail will cause program exit.
		logutil.BgLogger().Fatal("CreateSession error when executing bootstrap sql", zap.Error(err))
	}

	if runBootstrapSQL(s, cfg.BootstrapSQLParams) {
		ctx := kv.WithInternalSourceType(context.Background(), kv.InternalTxnBootstrap)
		err = kv.RunInNewTxn(ctx, storage, true, func(ctx context.Context, txn kv.Transaction) error {
			return meta.NewMeta(txn).SetInt64Key(mBootstrapSQLKey, int64(txn.StartTS()))
		})
		if err != nil {
			logutil.BgLogger().Fatal("finish bootstrap failed",
				zap.Error(err))
		}
	}
	s.Close()
}
