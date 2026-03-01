package fixmsg

// Well-known FIX tag numbers referenced throughout the engine.
const (
	TagBeginString  = 8
	TagBodyLength   = 9
	TagMsgType      = 35
	TagSenderCompID = 49
	TagTargetCompID = 56
	TagOnBehalfOf   = 115
	TagDeliverTo    = 128
	TagMsgSeqNum    = 34
	TagSendingTime  = 52
	TagPossDupFlag  = 43
	TagOrigSendTime = 122
	TagCheckSum     = 10

	// Session-level tags
	TagHeartBtInt          = 108
	TagTestReqID           = 112
	TagEncryptMethod       = 98
	TagResetSeqNumFlag     = 141
	TagText                = 58
	TagRefSeqNum           = 45
	TagRefMsgType          = 372
	TagSessionRejectReason = 373
	TagNewSeqNo            = 36
	TagGapFillFlag         = 123
	TagBeginSeqNo          = 7
	TagEndSeqNo            = 16

	// Common application tags
	TagClOrdID  = 11
	TagSymbol   = 55
	TagSide     = 54
	TagOrdType  = 40
	TagOrderQty = 38
	TagPrice    = 44
)

// HeaderTags is the canonical ordering of FIX header tags.
// Tags present in a message are written in this order before the body.
// Mirrors pyfixmsg/reference.py HEADER_TAGS.
var HeaderTags = []int{
	8, 9, 35, 1128, 1156, 1129, 49, 56, 115, 128,
	90, 91, 34, 50, 142, 57, 143, 116, 144, 129, 145,
	43, 97, 52, 122, 212, 213, 347, 369, 370,
}

// TrailerTags is the canonical ordering of FIX trailer tags.
// 93 (SignatureLength) and 89 (Signature) precede 10 (CheckSum).
var TrailerTags = []int{93, 89, 10}

// HeaderSortMap maps each header tag to its sort priority (lower = earlier).
var HeaderSortMap map[int]int

// trailerSortMap maps trailer tags to very high sort keys so they appear last.
var trailerSortMap map[int]int

func init() {
	HeaderSortMap = make(map[int]int, len(HeaderTags))
	for i, t := range HeaderTags {
		HeaderSortMap[t] = i
	}
	trailerSortMap = map[int]int{
		93: 1<<30 - 3,
		89: 1<<30 - 2,
		10: 1<<30 - 1,
	}
}

// tagSortKey returns the canonical FIX sort key for tag t.
// Header tags sort first (by position in HeaderTags), body tags sort by
// numeric value, trailer tags sort last.
func tagSortKey(t int) int {
	if k, ok := trailerSortMap[t]; ok {
		return k
	}
	if k, ok := HeaderSortMap[t]; ok {
		return k
	}
	return len(HeaderTags) + t
}
