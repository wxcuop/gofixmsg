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
Values map[string]string
}

// MessageSpec describes the fields and repeating groups of one FIX message type.
type MessageSpec struct {
Name    string
MsgType string
Groups  map[int]*GroupSpec
}

// GroupSpec describes a single repeating group within a message or nested group.
type GroupSpec struct {
NumberTag    int
FirstTag     int
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
XMLName xml.Name `xml:"fix"`
Major   string   `xml:"major,attr"`
Minor   string   `xml:"minor,attr"`
Type    string   `xml:"type,attr"`
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
Name   string        `xml:"name,attr"`
Fields []xmlFieldRef `xml:"field"`
Groups []xmlGroup    `xml:"group"`
Comps  []xmlCompRef  `xml:"component"`
}

type xmlFieldRef struct {
Name string `xml:"name,attr"`
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
Name:    m.Name,
MsgType: m.MsgType,
Groups:  make(map[int]*GroupSpec),
}
collectGroups(m.Groups, m.Comps, compMap, nameToNum, ms.Groups)
s.MessagesByType[m.MsgType] = ms
s.MessagesByName[m.Name] = ms
}
return s, nil
}

func collectGroups(groups []xmlGroup, comps []xmlCompRef, compMap map[string]*xmlComponent, nameToNum map[string]int, dest map[int]*GroupSpec) {
for _, ref := range comps {
if c, ok := compMap[ref.Name]; ok {
collectGroups(c.Groups, c.Comps, compMap, nameToNum, dest)
}
}
for _, g := range groups {
num, ok := nameToNum[g.Name]
if !ok {
continue
}
dest[num] = buildGroupSpec(num, g, compMap, nameToNum)
}
}

func buildGroupSpec(numberTag int, g xmlGroup, compMap map[string]*xmlComponent, nameToNum map[string]int) *GroupSpec {
gs := &GroupSpec{
NumberTag:    numberTag,
Tags:         make(map[int]struct{}),
NestedGroups: make(map[int]*GroupSpec),
}
allFields := expandFields(g.Fields, g.Comps, compMap)
for i, ref := range allFields {
tn, ok := nameToNum[ref.Name]
if !ok {
continue
}
if i == 0 {
gs.FirstTag = tn
}
gs.Tags[tn] = struct{}{}
}
for _, nested := range g.Groups {
nt, ok := nameToNum[nested.Name]
if !ok {
continue
}
gs.Tags[nt] = struct{}{}
gs.NestedGroups[nt] = buildGroupSpec(nt, nested, compMap, nameToNum)
}
return gs
}

func expandFields(fields []xmlFieldRef, comps []xmlCompRef, compMap map[string]*xmlComponent) []xmlFieldRef {
result := append([]xmlFieldRef{}, fields...)
for _, ref := range comps {
if c, ok := compMap[ref.Name]; ok {
result = append(result, expandFields(c.Fields, c.Comps, compMap)...)
}
}
return result
}
