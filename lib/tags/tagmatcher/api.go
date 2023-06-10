package tagmatcher

import (
	"github.com/Cloud-Foundations/Dominator/lib/tags"
)

type TagMatcher struct {
	matchTags map[string]map[string]struct{} // K: tag key, V: values to match.
}

// New will create a new *TagMatcher. If makeIfEmpty is false and matchTags is
// empty (length zero), nil is returned.
func New(matchTags tags.MatchTags, makeIfEmpty bool) *TagMatcher {
	return newTagMatcher(matchTags, makeIfEmpty)
}

// MatchEach will iterate over each tag key in the *TagMatcher and will return
// false if the corresponding value in tags does not match one of the values for
// the key, otherwise true is returned.
// If the *TagMatcher is nil or is empty, true is returned.
func (tm *TagMatcher) MatchEach(tags tags.Tags) bool {
	return tm.matchEach(tags)
}
