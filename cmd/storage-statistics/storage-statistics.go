/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/jcjones/ct-mapreduce/config"
	"github.com/jcjones/ct-mapreduce/storage"
	"golang.org/x/net/context"
)

var (
	ctconfig = config.NewCTConfig()
)

func main() {
	var err error
	var storageDB storage.CertDatabase
	var backend storage.StorageBackend

	if ctconfig.CertPath != nil && len(*ctconfig.CertPath) > 0 {
		backend := storage.NewLocalDiskBackend(0644, *ctconfig.CertPath)
		glog.Infof("Saving to disk at %s", *ctconfig.CertPath)
		storageDB, err = storage.NewFilesystemDatabase(*ctconfig.CacheSize, backend)
		if err != nil {
			glog.Fatalf("unable to open Certificate Path: %+v: %+v", ctconfig.CertPath, err)
		}
	} else if ctconfig.FirestoreProjectId != nil && len(*ctconfig.FirestoreProjectId) > 0 {
		ctx := context.Background()

		backend, err = storage.NewFirestoreBackend(ctx, *ctconfig.FirestoreProjectId)
		if err != nil {
			glog.Fatalf("Unable to configure Firestore for %s: %v", *ctconfig.FirestoreProjectId, err)
		}

		storageDB, err = storage.NewFilesystemDatabase(*ctconfig.CacheSize, backend)
		if err != nil {
			glog.Fatalf("Unable to construct Firestore DB for %s: %v", *ctconfig.FirestoreProjectId, err)
		}
	}

	if storageDB == nil {
		ctconfig.Usage()
		os.Exit(2)
	}

	expDateList, err := storageDB.ListExpirationDates(time.Now())
	if err != nil {
		glog.Fatalf("Couldn't list expiration dates: %v", err)
	}

	totalSerials := 0
	totalIssuers := 0
	totalCRLs := 0

	for _, expDate := range expDateList {
		dateTotalSerials := 0
		dateTotalIssuers := 0
		dateTotalCRLs := 0

		fmt.Printf("%s: \n", expDate)
		issuers, err := storageDB.ListIssuersForExpirationDate(expDate)
		if err != nil {
			glog.Errorf("Couldn't list issuers for %s: %v", expDate, err)
			continue
		}

		for _, issuer := range issuers {
			knownCerts := storage.NewKnownCertificates(expDate, issuer, backend)
			err = knownCerts.Load()
			if err != nil {
				glog.Errorf("Couldn't get known certs for %s-%s: %v", expDate, issuer, err)
				continue
			}

			issuerMetadata := storage.NewIssuerMetadata(expDate, issuer, backend)
			err = issuerMetadata.Load()
			if err != nil {
				glog.Errorf("Couldn't get issuer metadata for %s-%s: %v", expDate, issuer, err)
				continue
			}

			countSerials := len(knownCerts.Known())
			countCRLs := len(issuerMetadata.Metadata.Crls)

			dateTotalSerials = dateTotalSerials + countSerials
			dateTotalIssuers = dateTotalIssuers + 1
			dateTotalCRLs = dateTotalCRLs + countCRLs

			totalSerials = totalSerials + countSerials
			totalCRLs = totalCRLs + countCRLs
			totalIssuers = totalIssuers + 1

			fmt.Printf(" * %s: %d serials known, %d crls known, %d issuerDNs known\n", *issuerMetadata.Metadata.IssuerDNs[0], countSerials, countCRLs, len(issuerMetadata.Metadata.IssuerDNs))
		}

		fmt.Printf("%s totals: %d issuers, %d serials, %d crls\n", expDate, dateTotalIssuers, dateTotalSerials, dateTotalCRLs)
	}

	fmt.Printf("overall totals: %d issuers, %d serials, %d crls\n", totalIssuers, totalSerials, totalCRLs)

	fmt.Println("\nLog status:")

	if ctconfig.LogUrlList != nil && len(*ctconfig.LogUrlList) > 5 {
		for _, part := range strings.Split(*ctconfig.LogUrlList, ",") {
			ctLogUrl, err := url.Parse(strings.TrimSpace(part))
			if err != nil {
				glog.Fatalf("unable to set Certificate Log: %s", err)
			}

			state, err := storageDB.GetLogState(ctLogUrl)
			if err != nil {
				glog.Fatalf("unable to GetLogState: %s %v", ctLogUrl, err)
			}
			fmt.Println(state.String())
		}
	}
}
