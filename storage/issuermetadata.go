package storage

import (
	"encoding/json"
	"io/ioutil"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go/x509"
)

type IssuerMetadata struct {
	mutex    *sync.Mutex
	metadata metadata
	filePath string
	perms    os.FileMode
}

type metadata struct {
	Crls []*string `json:"crls"`
}

func NewIssuerMetadata(aMetadataPath string, aPerms os.FileMode) *IssuerMetadata {
	metadata := metadata{
		Crls: make([]*string, 0, 10),
	}
	return &IssuerMetadata{
		mutex:    &sync.Mutex{},
		filePath: aMetadataPath,
		perms:    aPerms,
		metadata: metadata,
	}
}

func (im *IssuerMetadata) Load() error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	fd, err := os.Open(im.filePath)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadAll(fd)
	if err != nil {
		glog.Errorf("Error reading issuer metadata %s: %s", im.filePath, err)
	}

	err = json.Unmarshal(data, &im.metadata)
	if err != nil {
		glog.Errorf("Error unmarshaling issuer metadata %s: %s", im.filePath, err)
	}

	if err = fd.Close(); err != nil {
		glog.Errorf("Error loading issuer metadata %s: %s", im.filePath, err)
	}
	return err
}

func (im *IssuerMetadata) Save() error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	fd, err := os.OpenFile(im.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, im.perms)
	if err != nil {
		glog.Errorf("Error opening issuer metadata %s: %s", im.filePath, err)
		return err
	}

	enc := json.NewEncoder(fd)

	if err := enc.Encode(im.metadata); err != nil {
		glog.Errorf("Error marshaling issuer metadata %s: %s", im.filePath, err)
	}

	if err = fd.Close(); err != nil {
		glog.Errorf("Error storing issuer metadata %s: %s", im.filePath, err)
	}

	return err
}

func (im *IssuerMetadata) addCRL(aCRL string) {
	// Assume that im.mutex is locked
	count := len(im.metadata.Crls)

	idx := sort.Search(count, func(i int) bool {
		return strings.Compare(aCRL, *im.metadata.Crls[i]) <= 0
	})

	var cmp int
	if idx < count {
		cmp = strings.Compare(aCRL, *im.metadata.Crls[idx])
	}

	if idx < count && cmp == 0 {
		glog.V(3).Infof("[%s] CRL already known: %s (pos=%d)", im.filePath, aCRL, idx)
		return
	}

	// Non-allocating insert, see https://github.com/golang/go/wiki/SliceTricks
	glog.V(3).Infof("[%s] CRL unknown: %s (pos=%d)", im.filePath, aCRL, idx)
	im.metadata.Crls = append(im.metadata.Crls, nil)
	copy(im.metadata.Crls[idx+1:], im.metadata.Crls[idx:])
	im.metadata.Crls[idx] = &aCRL
}

// Must tolerate duplicate information
func (im *IssuerMetadata) Accumulate(aCert *x509.Certificate) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	for _, dp := range aCert.CRLDistributionPoints {
		url, err := url.Parse(dp)
		if err != nil {
			glog.Warningf("Not a valid CRL DP URL: %s %s", dp, err)
			continue
		}

		if url.Scheme == "http" || url.Scheme == "https" {
			im.addCRL(dp)
		} else if url.Scheme == "ldap" || url.Scheme == "ldaps" {
			return
		} else {
			glog.V(3).Infof("Ignoring unknown CRL scheme: %v", url)
		}
	}
}