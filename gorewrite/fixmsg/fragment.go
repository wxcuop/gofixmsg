package fixmsg

import "fmt"

// FixFragment is the base tag→value map for FIX protocol fields.
//
// Keys are integer FIX tag numbers. Values are either string (for scalar
// fields) or *[RepeatingGroup] (for repeating-group delimiter tags).
//
// FixFragment is a named map type so it can carry methods while remaining
// directly usable as a map (range, len, delete all work as normal).
type FixFragment map[int]any

// Get returns the string value stored under tag.
// Returns ("", false) if the tag is absent or holds a *RepeatingGroup.
func (f FixFragment) Get(tag int) (string, bool) {
	v, ok := f[tag]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// MustGet returns the string value for tag.
// Panics if the tag is absent or holds a *RepeatingGroup.
func (f FixFragment) MustGet(tag int) string {
	s, ok := f.Get(tag)
	if !ok {
		panic(fmt.Sprintf("fixmsg: tag %d not present or is a group", tag))
	}
	return s
}

// Set stores value as the string value for tag.
func (f FixFragment) Set(tag int, value string) {
	f[tag] = value
}

// GetGroup returns the *RepeatingGroup stored under tag.
// Returns (nil, false) if the tag is absent or holds a string.
func (f FixFragment) GetGroup(tag int) (*RepeatingGroup, bool) {
	v, ok := f[tag]
	if !ok {
		return nil, false
	}
	g, ok := v.(*RepeatingGroup)
	return g, ok
}

// SetGroup stores group under tag.
func (f FixFragment) SetGroup(tag int, group *RepeatingGroup) {
	f[tag] = group
}

// Contains reports whether tag is present (string or group).
func (f FixFragment) Contains(tag int) bool {
	_, ok := f[tag]
	return ok
}

// AllTags returns every tag number present directly in this fragment.
// Does not recurse into nested groups.
func (f FixFragment) AllTags() []int {
	tags := make([]int, 0, len(f))
	for t := range f {
		tags = append(tags, t)
	}
	return tags
}

// AllTagsDeep returns every tag number in this fragment and any nested groups.
// Each tag appears at most once.
func (f FixFragment) AllTagsDeep() []int {
	seen := make(map[int]struct{})
	f.collectTags(seen)
	tags := make([]int, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	return tags
}

func (f FixFragment) collectTags(seen map[int]struct{}) {
	for tag, val := range f {
		seen[tag] = struct{}{}
		if g, ok := val.(*RepeatingGroup); ok {
			for _, member := range g.Members {
				member.collectTags(seen)
			}
		}
	}
}

// FindAll returns all paths to tag within this fragment and any nested groups.
// Each path is a []any where each element is an int (tag or group member index).
// Mirrors Python FixFragment.find_all semantics.
func (f FixFragment) FindAll(tag int) [][]any {
	var paths [][]any
	if _, ok := f[tag]; ok {
		paths = append(paths, []any{tag})
	}
	for innerTag, val := range f {
		g, ok := val.(*RepeatingGroup)
		if !ok {
			continue
		}
		for i, member := range g.Members {
			for _, sub := range member.FindAll(tag) {
				path := make([]any, 0, len(sub)+2)
				path = append(path, innerTag, i)
				path = append(path, sub...)
				paths = append(paths, path)
			}
		}
	}
	return paths
}

// Anywhere reports whether tag is present in this fragment or any nested group.
func (f FixFragment) Anywhere(tag int) bool {
	if _, ok := f[tag]; ok {
		return true
	}
	for _, val := range f {
		if g, ok := val.(*RepeatingGroup); ok {
			for _, member := range g.Members {
				if member.Anywhere(tag) {
					return true
				}
			}
		}
	}
	return false
}
