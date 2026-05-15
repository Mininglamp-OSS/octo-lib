package config

import (
	"github.com/Mininglamp-OSS/octo-lib/common"
	"github.com/Mininglamp-OSS/octo-lib/pkg/util"
)

// PersonalMsgOptions carries the optional MsgSendReq fields beyond the
// mandatory authoritative-payload contract enforced by NewPersonalMsgSendReq.
//
// Only fields a PERSONAL DM dispatcher legitimately controls are exposed.
// channel_type is set to ChannelTypePerson by the builder unconditionally;
// payload is encoded by the builder after authoritative space_id semantics
// have been applied.
type PersonalMsgOptions struct {
	Header      MsgHeader
	Setting     uint8
	StreamNo    string
	Subscribers []string
}

// NewPersonalMsgSendReq returns a *MsgSendReq for a PERSONAL (channel_type=1)
// dispatch with authoritative payload.space_id semantics applied to payload
// before encoding.
//
// Contract:
//   - senderSpaceID != ""  → payload["space_id"] = senderSpaceID (server overrides
//     any client-supplied value; this is the only authoritative source the
//     receiver-side SpaceFilter can trust on PERSONAL channels because WuKongIM
//     has no Space concept on channel_type=1).
//   - senderSpaceID == ""  → payload["space_id"] is stripped (fail-closed).
//     The empty case means the dispatcher has no Space context (orphan bot,
//     non-Space deployment, missing X-Space-ID middleware); we MUST NOT trust
//     a client-supplied value in that branch — see Mininglamp-OSS/octo-server
//     PR#35 R3 review (YUJ-660 High-3) for the fail-open hole this closes.
//
// Channel type is always set to ChannelTypePerson; callers MUST verify the
// channel is in fact PERSONAL before invoking this builder. Use the regular
// MsgSendReq literal for GROUP / COMMUNITY_TOPIC / RTC / etc.
//
// payload may be nil; an empty map is materialised. The map is mutated in
// place (consistent with octo-server's enrichPayloadWithSpaceID helpers) and
// then JSON-encoded into MsgSendReq.Payload.
//
// This builder is the defense-in-depth layer for Mininglamp-OSS/octo-server#37
// (PR#35 follow-up Medium-1). New PERSONAL DM call sites added in either
// repository SHOULD use this builder so payload.space_id correctness becomes
// a property of construction rather than caller diligence.
func NewPersonalMsgSendReq(
	channelID string,
	fromUID string,
	payload map[string]interface{},
	senderSpaceID string,
	opts PersonalMsgOptions,
) *MsgSendReq {
	if payload == nil {
		payload = make(map[string]interface{})
	}
	if senderSpaceID != "" {
		payload["space_id"] = senderSpaceID
	} else {
		// Fail-closed: server has no authoritative Space context, so any
		// client-supplied space_id is untrusted. Strip it. Receivers will
		// fall through to their own (cache-warmed) SpaceFilter logic.
		delete(payload, "space_id")
	}
	return &MsgSendReq{
		Header:      opts.Header,
		Setting:     opts.Setting,
		FromUID:     fromUID,
		ChannelID:   channelID,
		ChannelType: common.ChannelTypePerson.Uint8(),
		StreamNo:    opts.StreamNo,
		Subscribers: opts.Subscribers,
		Payload:     []byte(util.ToJson(payload)),
	}
}
