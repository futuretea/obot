package proxy

import "testing"

func TestProxyManagerSingleSessionOption(t *testing.T) {
	disabled := NewProxyManager(nil, nil, WithSingleSession(&recordingSingleSessionClient{}, nil, false))
	if disabled.singleSession != nil {
		t.Fatal("single-session enforcer should be disabled by default")
	}

	enabled := NewProxyManager(nil, nil, WithSingleSession(&recordingSingleSessionClient{}, nil, true))
	if enabled.singleSession == nil {
		t.Fatal("single-session enforcer should be configured when enabled")
	}
}
