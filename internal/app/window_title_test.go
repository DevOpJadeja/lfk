package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestWindowTitle_ShowsActiveContext(t *testing.T) {
	m := basePush80Model()

	assert.Equal(t, "lfk - test-ctx", m.View().WindowTitle)
}

func TestWindowTitle_ClusterListShowsAppNameOnly(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelClusters}

	assert.Equal(t, "lfk", m.View().WindowTitle)
}

func TestWindowTitle_UnionViewListsMemberContexts(t *testing.T) {
	m := basePush80Model()
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	m.unionContexts = []string{"dev", "qa"}

	assert.Equal(t, "lfk - [dev+qa]", m.View().WindowTitle)
}

func TestWindowTitle_FollowsContextSwitch(t *testing.T) {
	m := ctxKeepViewModel(t)

	ret, _ := m.executeBuiltinCommand("ctx qa")

	assert.Equal(t, "lfk - qa", ret.(Model).View().WindowTitle)
}

func TestWindowTitle_StripsControlCharacters(t *testing.T) {
	m := basePush80Model()
	m.nav.Context = "evil\x07\x1b]2;pwned\x1b\\\u009cctx"

	assert.Equal(t, "lfk - evil]2;pwned\\ctx", m.View().WindowTitle)
}
