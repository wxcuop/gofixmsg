// Package spec loads QuickFIX XML specification files.
package spec

import (
	"encoding/xml"
	"fmt"
	"os"
)

// FixSpec is the in-memory representation of a QuickFIX XML specification.
type FixSpec struct {
	Version        string
	Tags           map[int]*FixTag
	MessagesByType map[string]*MessageSpec
	MessagesByName map[string]*MessageSpec
}

// FixTag describes a single FIX tag from the specification.
type FixTag struct {
	Number int
	Name   string
	Type   string
	Values map[string]string // Enum values
}

// MessageSpec describes the fields and repeating groups of one FIX message type.
type MessageSpec struct {
	Name         string
	MsgType      string
	RequiredTags map[int]struct{}
	Tags         map[int]struct{}
	Groups       map[int]*GroupSpec
}

// GroupSpec describes a single repeating group within a message or nested group.
type GroupSpec struct {
	NumberTag    int
	FirstTag     int
	RequiredTags map[int]struct{}
	Tags         map[int]struct{}
	NestedGroups map[int]*GroupSpec
}

// MessageByType returns the MessageSpec for the given MsgType (e.g. "D").
func (s *FixSpec) MessageByType(msgType string) *MessageSpec {
	if s == nil {
		return nil
	}
	return s.MessagesByType[msgType]
}

// TagByNumber returns the FixTag for tag number n.
func (s *FixSpec) TagByNumber(n int) *FixTag {
	if s == nil {
		return nil
	}
	return s.Tags[n]
}

// TagNumber returns the tag number for tagName, (0, false) if unknown.
func (s *FixSpec) TagNumber(tagName string) (int, bool) {
	for n, t := range s.Tags {
		if t.Name == tagName {
			return n, true
		}
	}
	return 0, false
}

// ---- XML unmarshalling structs ----

type xmlFix struct {
	XMLName  xml.Name `xml:"fix"`
	Major    string   `xml:"major,attr"`
	Minor    string   `xml:"minor,attr"`
	Type     string   `xml:"type,attr"`
	Messages struct {
		Messages []xmlMessage `xml:"message"`
	} `xml:"messages"`
	Components struct {
		Components []xmlComponent `xml:"component"`
	} `xml:"components"`
	Fields struct {
		Fields []xmlField `xml:"field"`
	} `xml:"fields"`
}

type xmlMessage struct {
	Name    string        `xml:"name,attr"`
	MsgType string        `xml:"msgtype,attr"`
	Fields  []xmlFieldRef `xml:"field"`
	Groups  []xmlGroup    `xml:"group"`
	Comps   []xmlCompRef  `xml:"component"`
}

type xmlComponent struct {
	Name   string        `xml:"name,attr"`
	Fields []xmlFieldRef `xml:"field"`
	Groups []xmlGroup    `xml:"group"`
	Comps  []xmlCompRef  `xml:"component"`
}

type xmlGroup struct {
	Name     string        `xml:"name,attr"`
	Required string        `xml:"required,attr"`
	Fields   []xmlFieldRef `xml:"field"`
	Groups   []xmlGroup    `xml:"group"`
	Comps    []xmlCompRef  `xml:"component"`
}

type xmlFieldRef struct {
	Name     string `xml:"name,attr"`
	Required string `xml:"required,attr"`
}

type xmlCompRef struct {
	Name string `xml:"name,attr"`
}

type xmlField struct {
	Number int    `xml:"number,attr"`
	Name   string `xml:"name,attr"`
	Type   string `xml:"type,attr"`
	Values []struct {
		Enum        string `xml:"enum,attr"`
		Description string `xml:"description,attr"`
	} `xml:"value"`
}

// Load parses a QuickFIX XML spec file.
func Load(filename string) (*FixSpec, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("spec: open %q: %w", filename, err)
	}
	defer f.Close()
	var raw xmlFix
	if err := xml.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("spec: decode %q: %w", filename, err)
	}
	return buildSpec(&raw)
}

// LoadBytes parses a QuickFIX XML spec from in-memory bytes (useful in tests).
func LoadBytes(data []byte) (*FixSpec, error) {
	var raw xmlFix
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("spec: parse xml: %w", err)
	}
	return buildSpec(&raw)
}

func buildSpec(raw *xmlFix) (*FixSpec, error) {
	s := &FixSpec{
		Version:        fmt.Sprintf("%s.%s.%s", raw.Type, raw.Major, raw.Minor),
		Tags:           make(map[int]*FixTag),
		MessagesByType: make(map[string]*MessageSpec),
		MessagesByName: make(map[string]*MessageSpec),
	}
	nameToNum := make(map[string]int, len(raw.Fields.Fields))
	for _, f := range raw.Fields.Fields {
		ft := &FixTag{
			Number: f.Number,
			Name:   f.Name,
			Type:   f.Type,
			Values: make(map[string]string, len(f.Values)),
		}
		for _, v := range f.Values {
			ft.Values[v.Enum] = v.Description
		}
		s.Tags[f.Number] = ft
		nameToNum[f.Name] = f.Number
	}
	compMap := make(map[string]*xmlComponent, len(raw.Components.Components))
	for i := range raw.Components.Components {
		c := &raw.Components.Components[i]
		compMap[c.Name] = c
	}
	for _, m := range raw.Messages.Messages {
		ms := &MessageSpec{
			Name:         m.Name,
			MsgType:      m.MsgType,
			RequiredTags: make(map[int]struct{}),
			Tags:         make(map[int]struct{}),
			Groups:       make(map[int]*GroupSpec),
		}
		collectFields(m.Fields, m.Groups, m.Comps, compMap, nameToNum, ms.Tags, ms.RequiredTags, ms.Groups)
		s.MessagesByType[m.MsgType] = ms
		s.MessagesByName[m.Name] = ms
	}
	return s, nil
}

func collectFields(fields []xmlFieldRef, groups []xmlGroup, comps []xmlCompRef, compMap map[string]*xmlComponent, nameToNum map[string]int, destTags map[int]struct{}, destReq map[int]struct{}, destGroups map[int]*GroupSpec) {
	for _, f := range fields {
		if num, ok := nameToNum[f.Name]; ok {
			destTags[num] = struct{}{}
			if f.Required == "Y" {
				destReq[num] = struct{}{}
			}
		}
	}
	for _, g := range groups {
		if num, ok := nameToNum[g.Name]; ok {
			destTags[num] = struct{}{}
			if g.Required == "Y" {
				destReq[num] = struct{}{}
			}
			destGroups[num] = buildGroupSpec(num, g, compMap, nameToNum)
		}
	}
	for _, ref := range comps {
		if c, ok := compMap[ref.Name]; ok {
			collectFields(c.Fields, c.Groups, c.Comps, compMap, nameToNum, destTags, destReq, destGroups)
		}
	}
}

func buildGroupSpec(numberTag int, g xmlGroup, compMap map[string]*xmlComponent, nameToNum map[string]int) *GroupSpec {
	gs := &GroupSpec{
		NumberTag:    numberTag,
		RequiredTags: make(map[int]struct{}),
		Tags:         make(map[int]struct{}),
		NestedGroups: make(map[int]*GroupSpec),
	}
	
	// Expanding group is slightly different as it has its own scope for fields
	populateGroupSpec(gs, g.Fields, g.Groups, g.Comps, compMap, nameToNum)
	
	return gs
}

func populateGroupSpec(gs *GroupSpec, fields []xmlFieldRef, groups []xmlGroup, comps []xmlCompRef, compMap map[string]*xmlComponent, nameToNum map[string]int) {
	// The first field in the XML defines the FirstTag
	firstSet := false
	
	for _, f := range fields {
		if num, ok := nameToNum[f.Name]; ok {
			gs.Tags[num] = struct{}{}
			if f.Required == "Y" {
				gs.RequiredTags[num] = struct{}{}
			}
			if !firstSet {
				gs.FirstTag = num
				firstSet = true
			}
		}
	}
	for _, g := range groups {
		if num, ok := nameToNum[g.Name]; ok {
			gs.Tags[num] = struct{}{}
			if g.Required == "Y" {
				gs.RequiredTags[num] = struct{}{}
			}
			gs.NestedGroups[num] = buildGroupSpec(num, g, compMap, nameToNum)
			if !firstSet {
				gs.FirstTag = num
				firstSet = true
			}
		}
	}
	for _, ref := range comps {
		if c, ok := compMap[ref.Name]; ok {
			populateGroupSpec(gs, c.Fields, c.Groups, c.Comps, compMap, nameToNum)
		}
	}
}
