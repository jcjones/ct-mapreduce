package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/ascii85"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"time"

	"github.com/google/certificate-transparency-go/x509"
)

const (
	kExpirationFormat = "2006-01-02"
)

type CertificateLog struct {
	ShortURL      string    `db:"url"`           // URL to the log
	MaxEntry      int64     `db:"maxEntry"`      // The most recent entryID logged
	LastEntryTime time.Time `db:"lastEntryTime"` // Date when we completed the last update
}

func (o *CertificateLog) String() string {
	return fmt.Sprintf("[%s] MaxEntry=%d, LastEntryTime=%s", o.ShortURL, o.MaxEntry, o.LastEntryTime)
}

func CertificateLogIDFromShortURL(shortURL string) string {
	return base64.URLEncoding.EncodeToString([]byte(shortURL))
}

func (o *CertificateLog) ID() string {
	return CertificateLogIDFromShortURL(o.ShortURL)
}

type SerialUseType string

func (s SerialUseType) ID() string {
	return (string)(s)
}

const (
	Known   SerialUseType = "known"
	Revoked SerialUseType = "revoked"
)

type DocumentType int

type StorageBackend interface {
	MarkDirty(id string) error

	StoreCertificatePEM(serial Serial, expDate string, issuer Issuer, b []byte) error
	StoreLogState(log *CertificateLog) error
	StoreKnownCertificateList(useType SerialUseType, issuer Issuer, serials []Serial) error

	LoadCertificatePEM(serial Serial, expDate string, issuer Issuer) ([]byte, error)
	LoadLogState(logURL string) (*CertificateLog, error)

	AllocateExpDateAndIssuer(expDate string, issuer Issuer) error

	ListExpirationDates(aNotBefore time.Time) ([]string, error)
	ListIssuersForExpirationDate(expDate string) ([]Issuer, error)

	ListSerialsForExpirationDateAndIssuer(expDate string, issuer Issuer) ([]Serial, error)
	StreamSerialsForExpirationDateAndIssuer(expDate string, issuer Issuer) (<-chan Serial, error)
}

type CertDatabase interface {
	Cleanup() error
	SaveLogState(aLogObj *CertificateLog) error
	GetLogState(url *url.URL) (*CertificateLog, error)
	Store(aCert *x509.Certificate, aIssuer *x509.Certificate, aURL string, aEntryId int64) error
	ListExpirationDates(aNotBefore time.Time) ([]string, error)
	ListIssuersForExpirationDate(expDate string) ([]Issuer, error)
	ReconstructIssuerMetadata(expDate string, issuer Issuer) error
	GetKnownCertificates(aExpDate string, aIssuer Issuer) *KnownCertificates
	GetIssuerMetadata(aIssuer Issuer) *IssuerMetadata
}

type RemoteCache interface {
	Exists(key string) (bool, error)
	SortedInsert(key string, aEntry string) (bool, error)
	SortedContains(key string, aEntry string) (bool, error)
	SortedList(key string) ([]string, error)
	ExpireAt(key string, aExpTime time.Time) error
}

type Issuer struct {
	id   *string
	spki SPKI
}

func NewIssuer(aCert *x509.Certificate) Issuer {
	obj := Issuer{
		id:   nil,
		spki: SPKI{aCert.RawSubjectPublicKeyInfo},
	}
	return obj
}

func NewIssuerFromString(aStr string) Issuer {
	obj := Issuer{
		id: &aStr,
	}
	return obj
}

func (o *Issuer) ID() string {
	if o.id == nil {
		encodedDigest := o.spki.Sha256DigestURLEncodedBase64()
		o.id = &encodedDigest
	}
	return *o.id
}

func (o *Issuer) MarshalJSON() ([]byte, error) {
	if o.id == nil {
		_ = o.ID()
	}
	return json.Marshal(o.id)
}

func (o *Issuer) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &o.id)
}

type SPKI struct {
	spki []byte
}

func (o SPKI) ID() string {
	return base64.URLEncoding.EncodeToString(o.spki)
}

func (o SPKI) String() string {
	return hex.EncodeToString(o.spki)
}

func (o SPKI) Sha256DigestURLEncodedBase64() string {
	binaryDigest := sha256.Sum256(o.spki)
	encodedDigest := base64.URLEncoding.EncodeToString(binaryDigest[:])
	return encodedDigest
}

type Serial struct {
	serial []byte
}

type tbsCertWithRawSerial struct {
	Raw          asn1.RawContent
	Version      asn1.RawValue `asn1:"optional,explicit,default:0,tag:0"`
	SerialNumber asn1.RawValue
}

func NewSerial(aCert *x509.Certificate) Serial {
	var tbsCert tbsCertWithRawSerial
	_, err := asn1.Unmarshal(aCert.RawTBSCertificate, &tbsCert)
	if err != nil {
		panic(err)
	}
	return NewSerialFromBytes(tbsCert.SerialNumber.Bytes)
}

func NewSerialFromBytes(b []byte) Serial {
	obj := Serial{
		serial: b,
	}
	return obj
}

func NewSerialFromHex(s string) Serial {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return Serial{
		serial: b,
	}
}

func NewSerialFromIDString(s string) (Serial, error) {
	bytes, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return Serial{}, err
	}
	return NewSerialFromBytes(bytes), nil
}

func NewSerialFromAscii85(s string) (Serial, error) {
	dst := make([]byte, 4*len(s))
	ndst, nsrc, err := ascii85.Decode(dst, []byte(s), true)
	if err != nil {
		return Serial{}, err
	}
	if nsrc != len(s) {
		return Serial{}, fmt.Errorf("Problem decoding Ascii85 str=[%s]: decoded %d/%d: %+v", s, nsrc, ndst, dst)
	}
	return Serial{
		serial: dst[0:ndst],
	}, nil
}

func (s Serial) ID() string {
	return base64.URLEncoding.EncodeToString(s.serial)
}

func (s Serial) String() string {
	return s.HexString()
}

func (s Serial) HexString() string {
	return hex.EncodeToString(s.serial)
}

func (s Serial) Ascii85() string {
	dst := make([]byte, ascii85.MaxEncodedLen(len(s.serial)))
	n := ascii85.Encode(dst, s.serial)
	return string(dst[0:n])
}

func (s Serial) Cmp(o Serial) int {
	return bytes.Compare(s.serial, o.serial)
}

func (s Serial) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.HexString())
}

func (s *Serial) UnmarshalJSON(data []byte) error {
	if data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("Expected surrounding quotes")
	}
	b, err := hex.DecodeString(string(data[1 : len(data)-1]))
	s.serial = b
	return err
}

func (s Serial) MarshalBinary() ([]byte, error) {
	return s.MarshalJSON()
}

func (s *Serial) UnmarshalBinary(data []byte) error {
	return s.UnmarshalJSON(data)
}

func (s *Serial) AsBigInt() *big.Int {
	serialBigInt := big.NewInt(0)
	serialBigInt.SetBytes(s.serial)
	return serialBigInt
}
