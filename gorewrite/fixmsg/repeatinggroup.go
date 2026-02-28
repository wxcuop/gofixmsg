package fixmsg

// RepeatingGroup is an ordered list of [FixFragment] members representing a
// FIX repeating group (e.g. NoLegs, NoPartyIDs).
//
// A *RepeatingGroup is stored inside its parent [FixFragment] under the count
// tag (e.g. tag 78 = NoAllocs). During serialisation the count tag's value is
// written as len(Members), followed by each member's fields.
type RepeatingGroup struct {
	// NumberTag is the FIX count tag that owns this group (e.g. 78 = NoAllocs).
	NumberTag int
	// FirstTag is the delimiter tag that begins each new group member.
	FirstTag int
	// Members holds the individual group entries in order.
	Members []FixFragment
}

// NewRepeatingGroup creates a RepeatingGroup with the given count tag.
func NewRepeatingGroup(numberTag int) *RepeatingGroup {
	return &RepeatingGroup{NumberTag: numberTag}
}

// Len returns the number of group members.
func (g *RepeatingGroup) Len() int {
	if g == nil {
		return 0
	}
	return len(g.Members)
}

// Add appends an empty FixFragment to the group and returns it for population.
//
//	member := group.Add()
//	member.Set(79, "AccountA")
//	member.Set(80, "100")
func (g *RepeatingGroup) Add() FixFragment {
	f := make(FixFragment)
	g.Members = append(g.Members, f)
	return f
}

// At returns the member fragment at index i.
func (g *RepeatingGroup) At(i int) FixFragment {
	return g.Members[i]
}

// FindAll returns all paths to tag across all group members.
// See [FixFragment.FindAll] for path format.
func (g *RepeatingGroup) FindAll(tag int) [][]any {
	var paths [][]any
	for i, member := range g.Members {
		for _, sub := range member.FindAll(tag) {
			path := make([]any, 0, len(sub)+1)
			path = append(path, i)
			path = append(path, sub...)
			paths = append(paths, path)
		}
	}
	return paths
}
