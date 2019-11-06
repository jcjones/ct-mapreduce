/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package engine

import (
	"context"
	"os"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/armon/go-metrics"
	"github.com/golang/glog"
	"github.com/google/go-metrics-stackdriver"
	"github.com/jcjones/ct-mapreduce/config"
	"github.com/jcjones/ct-mapreduce/storage"
	"github.com/jcjones/ct-mapreduce/telemetry"
)

func GetConfiguredStorage(ctx context.Context, ctconfig *config.CTConfig) (storage.CertDatabase, storage.RemoteCache, storage.StorageBackend) {
	var err error
	var storageDB storage.CertDatabase
	var backend storage.StorageBackend

	hasLocalDiskConfig := ctconfig.CertPath != nil && len(*ctconfig.CertPath) > 0
	hasGoogleConfig := ctconfig.GoogleProjectId != nil && len(*ctconfig.GoogleProjectId) > 0

	if hasLocalDiskConfig && hasGoogleConfig {
		glog.Fatal("Local Disk and Google configurations both found. Exiting.")
	}

	redisTimeoutDuration, err := time.ParseDuration(*ctconfig.RedisTimeout)
	if err != nil {
		glog.Fatalf("Could not parse RedisTimeout: %v", err)
	}

	remoteCache, err := storage.NewRedisCache(*ctconfig.RedisHost, redisTimeoutDuration)
	if err != nil {
		glog.Fatalf("Unable to configure Redis cache for host %v", *ctconfig.RedisHost)
	}

	if hasLocalDiskConfig {
		glog.Fatalf("Local Disk Backend currently disabled")
	}

	if hasGoogleConfig {
		backend, err = storage.NewFirestoreBackend(ctx, *ctconfig.GoogleProjectId)
		if err != nil {
			glog.Fatalf("Unable to configure Firestore for %s: %v", *ctconfig.GoogleProjectId, err)
		}

		storageDB, err = storage.NewFilesystemDatabase(backend, remoteCache)
		if err != nil {
			glog.Fatalf("Unable to construct Firestore DB for %s: %v", *ctconfig.GoogleProjectId, err)
		}
	}

	if storageDB == nil {
		ctconfig.Usage()
		os.Exit(2)
	}

	return storageDB, remoteCache, backend
}

func PrepareTelemetry(utilName string, ctconfig *config.CTConfig) {
	val, ok := os.LookupEnv("stackdriverMetrics")
	if ok && val == "true" {
		client, err := monitoring.NewMetricClient(context.Background())
		if err != nil {
			glog.Fatal(err)
		}

		metricsSink := stackdriver.NewSink(client, &stackdriver.Config{
			ProjectID: *ctconfig.GoogleProjectId,
		})
		_, err = metrics.NewGlobal(metrics.DefaultConfig(utilName), metricsSink)
		if err != nil {
			glog.Fatal(err)
		}

		glog.Infof("%s is starting. Statistics are being reported to the Stackdriver project %s",
			utilName, *ctconfig.GoogleProjectId)

		return
	}

	infoDumpPeriod, err := time.ParseDuration(*ctconfig.StatsRefreshPeriod)
	if err != nil {
		glog.Fatalf("Could not parse StatsRefreshPeriod: %v", err)
	}

	glog.Infof("%s is starting. Local statistics will emit every: %s",
		utilName, infoDumpPeriod)

	metricsSink := metrics.NewInmemSink(infoDumpPeriod, 5*infoDumpPeriod)
	telemetry.NewMetricsDumper(metricsSink, infoDumpPeriod)
	_, err = metrics.NewGlobal(metrics.DefaultConfig(utilName), metricsSink)
	if err != nil {
		glog.Fatal(err)
	}
}
