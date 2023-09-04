package tagmatcher

import (
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
)

func newTagMatcher(matchTags tags.MatchTags, makeIfEmpty bool) *TagMatcher {
	if len(matchTags) < 1 {
		if makeIfEmpty {
			return &TagMatcher{}
		}
		return nil
	}
	tagMatcher := &TagMatcher{}
	tagMatcher.matchTags = make(map[string]map[string]struct{}, len(matchTags))
	for key, values := range matchTags {
		tagMatcher.matchTags[key] = stringutil.ConvertListToMap(values, false)
	}
	return tagMatcher
}

func (tm *TagMatcher) matchEach(tags tags.Tags) bool {
	if tm == nil {
		return true
	}
	for key, values := range tm.matchTags {
		if tagValue, ok := tags[key]; !ok {
			return false
		} else if _, ok := values[tagValue]; !ok {
			return false
		}
	}
	return true
}
