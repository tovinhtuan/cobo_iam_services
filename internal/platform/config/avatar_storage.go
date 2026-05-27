package config

import (
	"path/filepath"
	"strings"
)

// ResolveUserAvatarStorageDir returns the filesystem root for user avatar objects.
// When UserAvatarStorageDir is set, it wins; otherwise {CMSMediaStorageDir}/user-avatars
// or ./var/cms-media/user-avatars when CMS dir is default.
func (c Config) ResolveUserAvatarStorageDir() string {
	if v := strings.TrimSpace(c.UserAvatarStorageDir); v != "" {
		return filepath.Clean(v)
	}
	cmsRoot := strings.TrimSpace(c.CMSMediaStorageDir)
	if cmsRoot == "" {
		cmsRoot = "./var/cms-media"
	}
	return filepath.Clean(filepath.Join(cmsRoot, "user-avatars"))
}

// UserAvatarAllowedContentTypesSet returns a set of allowed MIME types.
func (c Config) UserAvatarAllowedContentTypesSet() map[string]struct{} {
	out := make(map[string]struct{})
	for _, ct := range c.UserAvatarAllowedTypes {
		ct = strings.ToLower(strings.TrimSpace(ct))
		if ct != "" {
			out[ct] = struct{}{}
		}
	}
	return out
}

func parseCommaSeparatedList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
