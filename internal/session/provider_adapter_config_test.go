// white-box-reason: tests unexported assembly helper for ProviderAdapterConfig field omission guard
package session

import (
	"reflect"
	"testing"
)

func TestAdapterConfigFromAmadeusFields_AllFieldsPopulated(t *testing.T) {
	pac := AdapterConfigFromAmadeusFields("test-claude", "test-model", 42, "/test/base")

	v := reflect.ValueOf(pac)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			t.Errorf("field %s is zero — helper omitted it", v.Type().Field(i).Name)
		}
	}
}
