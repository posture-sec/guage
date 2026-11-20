package model_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestAccountSnapshotSequenceIsAString(t *testing.T) {
	t.Parallel()

	t.Run("wire token", func(t *testing.T) {
		t.Parallel()
		snapshot := model.AccountSnapshot{Sequence: "23472095056887808"}
		data, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte(`"sequence":"23472095056887808"`)) {
			t.Fatalf("marshalled snapshot = %s, want a string sequence token", data)
		}
	})

	t.Run("numeric sequence is refused", func(t *testing.T) {
		t.Parallel()
		var snapshot model.AccountSnapshot
		if err := json.Unmarshal([]byte(`{"sequence":23472095056887808}`), &snapshot); err == nil {
			t.Fatalf("numeric sequence accepted as %q", snapshot.Sequence)
		}
	})

	t.Run("field kind is string", func(t *testing.T) {
		t.Parallel()
		field, ok := reflect.TypeOf(model.AccountSnapshot{}).FieldByName("Sequence")
		if !ok {
			t.Fatal("AccountSnapshot has no Sequence field")
		}
		if field.Type.Kind() != reflect.String {
			t.Fatalf("Sequence kind = %s, want string", field.Type.Kind())
		}
	})
}
