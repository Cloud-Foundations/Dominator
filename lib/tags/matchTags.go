package tags

import (
	"errors"
	"strings"
)

func (mtags *MatchTags) string() string {
	pairs := make([]string, 0, len(*mtags))
	for key, values := range *mtags {
		for _, value := range values {
			pairs = append(pairs, key+"="+value)
		}
	}
	return strings.Join(pairs, ",")
}

func (mtags *MatchTags) set(value string) error {
	newTags := make(MatchTags)
	if value == "" {
		*mtags = newTags
		return nil
	}
	tags := make(map[Tag]struct{})
	for _, tag := range strings.Split(value, ",") {
		if len(tag) < 1 {
			return errors.New(`malformed tag: "` + tag + `"`)
		}
		splitTag := strings.Split(tag, "=")
		if len(splitTag) != 2 {
			return errors.New(`malformed tag: "` + tag + `"`)
		}
		tag := Tag{Key: splitTag[0], Value: splitTag[1]}
		if _, ok := tags[tag]; !ok {
			newTags[tag.Key] = append(newTags[tag.Key], tag.Value)
		}
	}
	*mtags = newTags
	return nil
}
