package storage

import (
	"encoding/json"
	"math/big"
	"testing"
)

func Test_MergeSmall(t *testing.T) {
	left := NewKnownCertificates("", 0644)
	right := NewKnownCertificates("", 0644)

	left.known = []*big.Int{big.NewInt(1), big.NewInt(3), big.NewInt(5)}
	right.known = []*big.Int{big.NewInt(4)}

	origText, err := json.Marshal(left.known)
	if err != nil {
		t.Error(err)
	}
	origTextR, err := json.Marshal(right.known)
	if err != nil {
		t.Error(err)
	}

	if string(origText) != "[1,3,5]" {
		t.Error("Invalid initial: left")
	}
	if string(origTextR) != "[4]" {
		t.Error("Invalid initial: right")
	}

	err = left.Merge(right)
	if err != nil {
		t.Error(err)
	}

	mergedText, err := json.Marshal(left.known)
	if err != nil {
		t.Error(err)
	}
	if string(mergedText) != "[1,3,4,5]" {
		t.Error("Invalid initial: right")
	}
}

func Test_MergeOutOfOrder(t *testing.T) {
	left := NewKnownCertificates("", 0644)
	right := NewKnownCertificates("", 0644)

	left.known = []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(0)}
	right.known = []*big.Int{big.NewInt(4)}

	origText, err := json.Marshal(left.known)
	if err != nil {
		t.Error(err)
	}
	origTextR, err := json.Marshal(right.known)
	if err != nil {
		t.Error(err)
	}

	if string(origText) != "[1,2,3,0]" {
		t.Error("Invalid initial: left")
	}
	if string(origTextR) != "[4]" {
		t.Error("Invalid initial: right")
	}

	err = left.Merge(right)
	if err.Error() != "Unsorted merge" {
		t.Errorf("Expected unsorted error!: %s", err)
	}
}

func Test_MergeDescending(t *testing.T) {
	left := NewKnownCertificates("", 0644)
	right := NewKnownCertificates("", 0644)

	left.known = []*big.Int{big.NewInt(4), big.NewInt(3), big.NewInt(2), big.NewInt(1)}
	right.known = []*big.Int{big.NewInt(0)}

	origText, err := json.Marshal(left.known)
	if err != nil {
		t.Error(err)
	}
	origTextR, err := json.Marshal(right.known)
	if err != nil {
		t.Error(err)
	}

	if string(origText) != "[4,3,2,1]" {
		t.Error("Invalid initial: left")
	}
	if string(origTextR) != "[0]" {
		t.Error("Invalid initial: right")
	}

	err = left.Merge(right)
	if err.Error() != "Unsorted merge" {
		t.Errorf("Expected unsorted error!: %s", err)
	}
}

func Test_Unknown(t *testing.T) {
	kc := NewKnownCertificates("", 0644)

	kc.known = []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4)}

	origText, err := json.Marshal(kc.known)
	if err != nil {
		t.Error(err)
	}

	if string(origText) != "[1,2,3,4]" {
		t.Error("Invalid initial")
	}

	for _, bi := range kc.known {
		if u, _ := kc.WasUnknown(bi); u == true {
			t.Errorf("%v should have been known", bi)
		}
	}

	if u, _ := kc.WasUnknown(big.NewInt(5)); u == false {
		t.Error("5 should not have been known")
	}

	if u, _ := kc.WasUnknown(big.NewInt(5)); u == true {
		t.Error("5 should now have been known")
	}

	endText, err := json.Marshal(kc.known)
	if err != nil {
		t.Error(err)
	}

	if string(endText) != "[1,2,3,4,5]" {
		t.Error("Invalid end")
	}
}

func Test_IsSorted(t *testing.T) {
	kc := NewKnownCertificates("", 0644)
	kc.known = []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4)}

	if kc.IsSorted() != true {
		t.Error("Should be sorted")
	}

	kc.known = []*big.Int{big.NewInt(1), big.NewInt(3), big.NewInt(2), big.NewInt(4)}

	if kc.IsSorted() != false {
		t.Error("Should not be sorted")
	}
}

func BenchmarkMerge(b *testing.B) {
	b.StopTimer()

	left := NewKnownCertificates("", 0644)
	right := NewKnownCertificates("", 0644)

	var i int64
	for i = 0; i < 128*1024*1024; i++ {
		if i%2 == 0 {
			left.known = append(left.known, big.NewInt(i))
		} else {
			right.known = append(right.known, big.NewInt(i))
		}
	}

	b.StartTimer()

	err := left.Merge(right)
	if err != nil {
		b.Error(err)
	}
}
