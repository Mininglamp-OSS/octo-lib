package config

import (
	"encoding/json"
	"testing"

	"github.com/Mininglamp-OSS/octo-lib/common"
)

func decodePayload(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("payload not JSON: %v (raw=%s)", err, string(b))
	}
	return out
}

// senderSpaceID non-empty → server overrides any client-supplied value.
func TestNewPersonalMsgSendReq_OverridesClientWhenSenderSpaceIDProvided(t *testing.T) {
	req := NewPersonalMsgSendReq(
		"peer_uid",
		"sender_uid",
		map[string]interface{}{
			"content":  "hi",
			"type":     1,
			"space_id": "client_forged_space",
		},
		"server_authoritative_space",
		PersonalMsgOptions{Header: MsgHeader{RedDot: 1}},
	)
	if req == nil {
		t.Fatal("nil req")
	}
	if req.ChannelType != common.ChannelTypePerson.Uint8() {
		t.Errorf("ChannelType=%d, want PERSONAL=%d", req.ChannelType, common.ChannelTypePerson.Uint8())
	}
	if req.ChannelID != "peer_uid" || req.FromUID != "sender_uid" {
		t.Errorf("identity fields not preserved: %+v", req)
	}
	got := decodePayload(t, req.Payload)
	if got["space_id"] != "server_authoritative_space" {
		t.Errorf("space_id=%v, want server_authoritative_space (client value MUST be overridden)", got["space_id"])
	}
	if got["content"] != "hi" {
		t.Errorf("content not preserved, got %v", got["content"])
	}
}

// senderSpaceID non-empty → injects when payload has no space_id at all.
func TestNewPersonalMsgSendReq_InjectsWhenAbsent(t *testing.T) {
	req := NewPersonalMsgSendReq(
		"peer_uid",
		"sender_uid",
		map[string]interface{}{"content": "hi"},
		"server_authoritative_space",
		PersonalMsgOptions{},
	)
	got := decodePayload(t, req.Payload)
	if got["space_id"] != "server_authoritative_space" {
		t.Errorf("space_id=%v, want server_authoritative_space", got["space_id"])
	}
}

// senderSpaceID empty + client space_id present → STRIP (fail-closed).
// This is the YUJ-660 High-3 fix encoded into the builder.
func TestNewPersonalMsgSendReq_StripsClientSpaceIDWhenSenderEmpty(t *testing.T) {
	req := NewPersonalMsgSendReq(
		"peer_uid",
		"sender_uid",
		map[string]interface{}{
			"content":  "hi",
			"space_id": "client_forged_space",
		},
		"",
		PersonalMsgOptions{},
	)
	got := decodePayload(t, req.Payload)
	if _, present := got["space_id"]; present {
		t.Errorf("space_id MUST be stripped when senderSpaceID empty, got %v", got["space_id"])
	}
	if got["content"] != "hi" {
		t.Errorf("content not preserved, got %v", got["content"])
	}
}

// senderSpaceID empty + no client space_id → still no space_id in output.
func TestNewPersonalMsgSendReq_LeavesAbsentWhenBothEmpty(t *testing.T) {
	req := NewPersonalMsgSendReq(
		"peer_uid",
		"sender_uid",
		map[string]interface{}{"content": "hi"},
		"",
		PersonalMsgOptions{},
	)
	got := decodePayload(t, req.Payload)
	if _, present := got["space_id"]; present {
		t.Errorf("space_id appeared from nowhere, got %v", got["space_id"])
	}
}

// nil payload is materialised to an empty map; all other invariants hold.
func TestNewPersonalMsgSendReq_NilPayloadInitialised(t *testing.T) {
	req := NewPersonalMsgSendReq("peer_uid", "sender_uid", nil, "spaceA", PersonalMsgOptions{})
	got := decodePayload(t, req.Payload)
	if got["space_id"] != "spaceA" {
		t.Errorf("space_id=%v, want spaceA", got["space_id"])
	}
}

// Optional fields propagate end-to-end.
func TestNewPersonalMsgSendReq_OptionalFieldsPropagate(t *testing.T) {
	req := NewPersonalMsgSendReq(
		"peer_uid",
		"sender_uid",
		map[string]interface{}{"content": "hi"},
		"spaceA",
		PersonalMsgOptions{
			Header:      MsgHeader{NoPersist: 1, RedDot: 1, SyncOnce: 1},
			Setting:     0x10,
			StreamNo:    "stream-1",
			Subscribers: []string{"a", "b"},
		},
	)
	if req.Header.RedDot != 1 || req.Header.NoPersist != 1 || req.Header.SyncOnce != 1 {
		t.Errorf("Header lost: %+v", req.Header)
	}
	if req.Setting != 0x10 {
		t.Errorf("Setting lost: %d", req.Setting)
	}
	if req.StreamNo != "stream-1" {
		t.Errorf("StreamNo lost: %s", req.StreamNo)
	}
	if len(req.Subscribers) != 2 || req.Subscribers[0] != "a" || req.Subscribers[1] != "b" {
		t.Errorf("Subscribers lost: %v", req.Subscribers)
	}
}

// ChannelType is always PERSONAL regardless of caller intent — defensive.
func TestNewPersonalMsgSendReq_AlwaysPersonalChannelType(t *testing.T) {
	req := NewPersonalMsgSendReq("peer_uid", "sender_uid", nil, "", PersonalMsgOptions{})
	if req.ChannelType != common.ChannelTypePerson.Uint8() {
		t.Errorf("ChannelType=%d, want PERSONAL=%d", req.ChannelType, common.ChannelTypePerson.Uint8())
	}
}
