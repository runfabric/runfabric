package local

import (
	"testing"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

func TestReceiptBackend_Save_Load_Delete_ListReleases(t *testing.T) {
	root := t.TempDir()
	b := NewReceiptBackend(root)
	r := &statetypes.Receipt{
		Service: "svc", Stage: "dev", Provider: "aws-lambda", DeploymentID: "dep-1",
	}
	if err := b.Save(r); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.Load("dev")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Service != "svc" || loaded.Stage != "dev" {
		t.Errorf("loaded: %+v", loaded)
	}
	list, err := b.ListReleases()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Stage != "dev" {
		t.Errorf("ListReleases: %+v", list)
	}
	if err := b.Delete("dev"); err != nil {
		t.Fatal(err)
	}
	_, err = b.Load("dev")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestReceiptBackend_Save_Nil(t *testing.T) {
	b := NewReceiptBackend(t.TempDir())
	if err := b.Save(nil); err == nil {
		t.Fatal("expected error for nil receipt")
	}
}

func TestReceiptBackend_Kind(t *testing.T) {
	b := NewReceiptBackend(t.TempDir())
	if b.Kind() != "local" {
		t.Errorf("Kind: got %q", b.Kind())
	}
}
