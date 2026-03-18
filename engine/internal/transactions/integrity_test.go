package transactions

import (
	"testing"
)

func TestComputeChecksum_VerifyChecksum(t *testing.T) {
	j := &JournalFile{
		Service: "svc", Stage: "dev", Operation: "deploy", Status: StatusActive,
		Version: 1, Checksum: "ignored",
	}
	sum, err := ComputeChecksum(j)
	if err != nil {
		t.Fatal(err)
	}
	if sum == "" {
		t.Fatal("checksum empty")
	}
	j.Checksum = sum
	ok, err := VerifyChecksum(j)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("VerifyChecksum expected true")
	}
	j.Checksum = "wrong"
	ok, err = VerifyChecksum(j)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("VerifyChecksum expected false for wrong checksum")
	}
}
