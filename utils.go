package gitcfg

import (
    "strings"
)

func parseConfigKey(key string) (section, keyName string, err error) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return "", "", ErrInvalidKeyFormat
	}

	section, remaining := parts[0], parts[1]

	// remote.origin.url -> section: remote.origin, key: url
	if strings.Contains(remaining, ".") {
		subparts := strings.SplitN(remaining, ".", 2)
		if len(subparts) == 2 {
			section = section + "." + subparts[0]
			remaining = subparts[1]
		}
	}

	return section, remaining, nil
}
