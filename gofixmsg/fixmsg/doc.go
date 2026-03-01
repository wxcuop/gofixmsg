// Package fixmsg provides FIX protocol message representation.
//
// The core types are [FixMessage] (a top-level FIX message), [FixFragment]
// (a tag→value map used for both messages and repeating-group members), and
// [RepeatingGroup] (an ordered list of [FixFragment]s for group fields).
//
// Wire-format parsing and serialisation is handled by the [fixmsg/codec]
// sub-package; QuickFIX XML specification loading by [fixmsg/spec].
package fixmsg
