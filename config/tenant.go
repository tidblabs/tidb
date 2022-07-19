package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/tikv/client-go/v2/config"
	"github.com/tikv/client-go/v2/util"
	pd "github.com/tikv/pd/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func runWithPDClient(ctx context.Context, cfg *Config, fn func(pdCli pd.Client) error) error {
	path := fmt.Sprintf("tikv://%s", cfg.Path)
	etcdAddrs, _, err := config.ParsePath(path)
	if err != nil {
		return errors.WithStack(err)
	}
	pdCli, err := pd.NewClient(etcdAddrs, pd.SecurityOption{
		CAPath:   cfg.Security.ClusterSSLCA,
		CertPath: cfg.Security.ClusterSSLCert,
		KeyPath:  cfg.Security.ClusterSSLKey,
	},
		pd.WithGRPCDialOptions(
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:    time.Duration(cfg.TiKVClient.GrpcKeepAliveTime) * time.Second,
				Timeout: time.Duration(cfg.TiKVClient.GrpcKeepAliveTimeout) * time.Second,
			}),
		),
		pd.WithCustomTimeoutOption(time.Duration(cfg.PDClient.PDServerTimeout)*time.Second),
		pd.WithForwardingOption(config.GetGlobalConfig().EnableForwarding))
	if err != nil {
		return errors.WithStack(err)
	}
	err = fn(util.InterceptedPDClient{Client: pdCli})
	pdCli.Close()
	return err
}

func loadTenantIDFromClusterName(cfg *Config) error {
	clusterName := strings.TrimSpace(os.Getenv("TIDB_CLUSTER_NAME"))
	if len(clusterName) == 0 {
		return nil
	}

	return runWithPDClient(context.Background(), cfg, func(pdCli pd.Client) error {
		if tenantID, err := pdCli.CreateTenantID(context.Background(), clusterName); err != nil {
			return errors.WithStack(err)
		} else {
			cfg.Tenant.IsTenant = true
			cfg.Tenant.TenantId = tenantID
			return nil
		}
	})
}
