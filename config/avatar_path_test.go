package config

import "testing"

// newAvatarTestConfig 返回一个固定 Avatar.Partition 的配置,使路径分区可被确定性断言。
func newAvatarTestConfig() *Config {
	cfg := New()
	cfg.Avatar.Partition = 100
	return cfg
}

// crc32 % 100 的确定性结果(Partition=100):
//
//	"G123" -> 60, "C456" -> 84, "C789" -> 89
func TestGetGroupAvatarFilePath(t *testing.T) {
	cfg := newAvatarTestConfig()

	tests := []struct {
		name    string
		groupNo string
		version []int64
		want    string
	}{
		{"no version returns legacy path", "G123", nil, "group/60/G123.png"},
		{"zero version returns legacy path", "G123", []int64{0}, "group/60/G123.png"},
		{"negative version returns legacy path", "G123", []int64{-1}, "group/60/G123.png"},
		{"positive version returns versioned path", "G123", []int64{7}, "group/60/G123/7.png"},
		{"large positive version", "G123", []int64{1717171717}, "group/60/G123/1717171717.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetGroupAvatarFilePath(tt.groupNo, tt.version...)
			if got != tt.want {
				t.Errorf("GetGroupAvatarFilePath(%q, %v) = %q, want %q", tt.groupNo, tt.version, got, tt.want)
			}
		})
	}
}

func TestGetCommunityAvatarFilePath(t *testing.T) {
	cfg := newAvatarTestConfig()

	tests := []struct {
		name        string
		communityNo string
		version     []int64
		want        string
	}{
		{"no version returns legacy path", "C456", nil, "community/84/C456.png"},
		{"zero version returns legacy path", "C456", []int64{0}, "community/84/C456.png"},
		{"negative version returns legacy path", "C456", []int64{-5}, "community/84/C456.png"},
		{"positive version returns versioned path", "C456", []int64{42}, "community/84/C456/42.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetCommunityAvatarFilePath(tt.communityNo, tt.version...)
			if got != tt.want {
				t.Errorf("GetCommunityAvatarFilePath(%q, %v) = %q, want %q", tt.communityNo, tt.version, got, tt.want)
			}
		})
	}
}

func TestGetCommunityCoverFilePath(t *testing.T) {
	cfg := newAvatarTestConfig()

	tests := []struct {
		name        string
		communityNo string
		version     []int64
		want        string
	}{
		{"no version returns legacy path", "C789", nil, "community/89/C789_cover.png"},
		{"zero version returns legacy path", "C789", []int64{0}, "community/89/C789_cover.png"},
		{"negative version returns legacy path", "C789", []int64{-1}, "community/89/C789_cover.png"},
		{"positive version returns versioned path", "C789", []int64{9}, "community/89/C789/cover/9.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetCommunityCoverFilePath(tt.communityNo, tt.version...)
			if got != tt.want {
				t.Errorf("GetCommunityCoverFilePath(%q, %v) = %q, want %q", tt.communityNo, tt.version, got, tt.want)
			}
		})
	}
}

// TestAvatarVersionedPathDiffersFromLegacy 确认带版本路径与旧版路径不同(对象键确实变化),
// 这是规避忽略 query string 的 CDN 缓存的关键属性。
func TestAvatarVersionedPathDiffersFromLegacy(t *testing.T) {
	cfg := newAvatarTestConfig()

	cases := []struct {
		name      string
		legacy    string
		versioned string
	}{
		{"group", cfg.GetGroupAvatarFilePath("G123"), cfg.GetGroupAvatarFilePath("G123", 1)},
		{"community", cfg.GetCommunityAvatarFilePath("C456"), cfg.GetCommunityAvatarFilePath("C456", 1)},
		{"cover", cfg.GetCommunityCoverFilePath("C789"), cfg.GetCommunityCoverFilePath("C789", 1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.legacy == tc.versioned {
				t.Errorf("versioned path should differ from legacy, both = %q", tc.legacy)
			}
		})
	}
}

// TestAvatarDistinctVersionsProduceDistinctPaths 锁定「每个版本号产生唯一对象键」契约,
// 这是缓存失效生效的前提:版本变化时对象键必须变化。
func TestAvatarDistinctVersionsProduceDistinctPaths(t *testing.T) {
	cfg := newAvatarTestConfig()

	if v1, v2 := cfg.GetGroupAvatarFilePath("G123", 1), cfg.GetGroupAvatarFilePath("G123", 2); v1 == v2 {
		t.Errorf("distinct versions should produce distinct paths, both = %q", v1)
	}
	if v1, v2 := cfg.GetCommunityAvatarFilePath("C456", 1), cfg.GetCommunityAvatarFilePath("C456", 2); v1 == v2 {
		t.Errorf("distinct versions should produce distinct paths, both = %q", v1)
	}
	if v1, v2 := cfg.GetCommunityCoverFilePath("C789", 1), cfg.GetCommunityCoverFilePath("C789", 2); v1 == v2 {
		t.Errorf("distinct versions should produce distinct paths, both = %q", v1)
	}
}

// TestAvatarVersionFirstWins 验证变参的「仅取第一个参数」语义:多传的版本号被忽略。
func TestAvatarVersionFirstWins(t *testing.T) {
	cfg := newAvatarTestConfig()

	got := cfg.GetGroupAvatarFilePath("G123", 7, 99, 100)
	want := "group/60/G123/7.png"
	if got != want {
		t.Errorf("GetGroupAvatarFilePath first-wins: got %q, want %q", got, want)
	}
}
