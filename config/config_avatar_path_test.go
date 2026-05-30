package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestGetGroupAvatarFilePath_BucketPrefix verifies that the group avatar path
// lands in the "avatar" bucket (publicly readable), not the "group" bucket
// (private). This is the regression guard for GH#21 / octo-server#103.
func TestGetGroupAvatarFilePath_BucketPrefix(t *testing.T) {
	cfg := New()
	vp := viper.New()
	cfg.ConfigureWithViper(vp)

	path := cfg.GetGroupAvatarFilePath("g_test_12345")

	// Must start with "avatar/" so splitBucketAndObject routes to the avatar bucket.
	if !strings.HasPrefix(path, "avatar/") {
		t.Errorf("GetGroupAvatarFilePath should start with 'avatar/' to route to the public avatar bucket, got %q", path)
	}

	// Must NOT start with "group/" — that routes to the private group bucket.
	if strings.HasPrefix(path, "group/") {
		t.Errorf("GetGroupAvatarFilePath must not start with 'group/' (private bucket), got %q", path)
	}

	// Must end with ".png" and contain the group number.
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("GetGroupAvatarFilePath should end with '.png', got %q", path)
	}
	if !strings.Contains(path, "g_test_12345") {
		t.Errorf("GetGroupAvatarFilePath should contain the group number, got %q", path)
	}

	// Must contain a "group/" sub-path to namespace group avatars within the avatar bucket.
	if !strings.Contains(path, "/group/") {
		t.Errorf("GetGroupAvatarFilePath should contain '/group/' namespace, got %q", path)
	}
}

// TestGetGroupAvatarFilePath_Deterministic verifies the same groupNo always
// produces the same path (partition is deterministic via CRC32).
func TestGetGroupAvatarFilePath_Deterministic(t *testing.T) {
	cfg := New()
	vp := viper.New()
	cfg.ConfigureWithViper(vp)

	p1 := cfg.GetGroupAvatarFilePath("g_abc")
	p2 := cfg.GetGroupAvatarFilePath("g_abc")
	if p1 != p2 {
		t.Errorf("GetGroupAvatarFilePath should be deterministic: %q != %q", p1, p2)
	}
}

// TestGetGroupAvatarFilePath_DifferentGroups verifies distinct groups get
// distinct paths (collision-free at the groupNo level).
func TestGetGroupAvatarFilePath_DifferentGroups(t *testing.T) {
	cfg := New()
	vp := viper.New()
	cfg.ConfigureWithViper(vp)

	p1 := cfg.GetGroupAvatarFilePath("g_alice_group")
	p2 := cfg.GetGroupAvatarFilePath("g_bob_group")
	if p1 == p2 {
		t.Errorf("different groups should produce different paths: both %q", p1)
	}
}
