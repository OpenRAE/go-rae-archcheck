package core_test

import (
	"testing"

	"example.com/fixture/internal/core"
	"example.com/fixture/internal/core/coretest"
)

func TestName(t *testing.T) {
	if core.Name() != coretest.Want() {
		t.Skip("fixture")
	}
}
