package storage

import (
	"context"
	"crypto/sha1"
	"encoding/pem"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go/x509"
)

type FilesystemDatabase struct {
	backend   StorageBackend
	extCache  RemoteCache
	metaMutex *sync.RWMutex
	meta      map[string]*IssuerMetadata
}

func NewFilesystemDatabase(aBackend StorageBackend, aExtCache RemoteCache) (*FilesystemDatabase, error) {
	db := &FilesystemDatabase{
		backend:   aBackend,
		extCache:  aExtCache,
		metaMutex: &sync.RWMutex{},
		meta:      make(map[string]*IssuerMetadata),
	}

	return db, nil
}

func (db *FilesystemDatabase) GetIssuerMetadata(aIssuer Issuer) *IssuerMetadata {
	db.metaMutex.RLock()

	im, ok := db.meta[aIssuer.ID()]
	if ok {
		db.metaMutex.RUnlock()
		return im
	}

	db.metaMutex.RUnlock()
	db.metaMutex.Lock()

	im = NewIssuerMetadata(aIssuer, db.extCache)
	db.meta[aIssuer.ID()] = im

	db.metaMutex.Unlock()
	return im
}

func (db *FilesystemDatabase) ListExpirationDates(aNotBefore time.Time) ([]string, error) {
	return db.backend.ListExpirationDates(context.Background(), aNotBefore)
}

func (db *FilesystemDatabase) ListIssuersForExpirationDate(expDate string) ([]Issuer, error) {
	return db.backend.ListIssuersForExpirationDate(context.Background(), expDate)
}

func (db *FilesystemDatabase) SaveLogState(aLogObj *CertificateLog) error {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	return db.backend.StoreLogState(ctx, aLogObj)
}

func (db *FilesystemDatabase) GetLogState(aUrl *url.URL) (*CertificateLog, error) {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	shortUrl := fmt.Sprintf("%s%s", aUrl.Host, aUrl.Path)
	return db.backend.LoadLogState(ctx, shortUrl)
}

func (db *FilesystemDatabase) markDirty(aExpiration *time.Time) error {
	subdirName := aExpiration.Format(kExpirationFormat)
	return db.backend.MarkDirty(subdirName)
}

func getSpki(aCert *x509.Certificate) SPKI {
	if len(aCert.SubjectKeyId) < 8 {
		digest := sha1.Sum(aCert.RawSubjectPublicKeyInfo)

		glog.V(2).Infof("[issuer: %s] SPKI is short: %v, using %v instead.", aCert.Issuer.String(), aCert.SubjectKeyId, digest[0:])
		return SPKI{digest[0:]}
	}

	return SPKI{aCert.SubjectKeyId}
}

func (db *FilesystemDatabase) Store(aCert *x509.Certificate, aIssuer *x509.Certificate, aLogURL string, aEntryId int64) error {
	expDate := aCert.NotAfter.Format(kExpirationFormat)
	issuer := NewIssuer(aIssuer)
	knownCerts := db.GetKnownCertificates(expDate, issuer)

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	headers := make(map[string]string)
	headers["Log"] = aLogURL
	headers["Recorded-at"] = time.Now().Format(time.RFC3339)
	headers["Entry-id"] = strconv.FormatInt(aEntryId, 10)
	pemblock := pem.Block{
		Type:    "CERTIFICATE",
		Headers: headers,
		Bytes:   aCert.Raw,
	}

	serialNum := NewSerial(aCert)

	certWasUnknown, err := knownCerts.WasUnknown(serialNum)
	if err != nil {
		return err
	}

	if certWasUnknown {
		issuerSeenBefore, err := db.GetIssuerMetadata(issuer).Accumulate(aCert)
		if err != nil {
			return err
		}
		if !issuerSeenBefore {
			// if the issuer/expdate was unknown in the cache
			errAlloc := db.backend.AllocateExpDateAndIssuer(ctx, expDate, issuer)
			if errAlloc != nil {
				return errAlloc
			}
			knownCerts.SetExpiryFlag()
		}

		errStore := db.backend.StoreCertificatePEM(ctx, serialNum, expDate, issuer, pem.EncodeToMemory(&pemblock))
		if errStore != nil {
			return errStore
		}
	}

	// Mark the directory dirty
	err = db.markDirty(&aCert.NotAfter)
	if err != nil {
		return err
	}

	return nil
}

func (db *FilesystemDatabase) GetKnownCertificates(aExpDate string, aIssuer Issuer) *KnownCertificates {
	return NewKnownCertificates(aExpDate, aIssuer, db.extCache)
}

func (db *FilesystemDatabase) Cleanup() error {
	// TODO: Remove
	return nil
}
