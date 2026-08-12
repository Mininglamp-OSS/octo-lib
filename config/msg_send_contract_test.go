package config

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendDurableMessageWithResult_PassesStableClientMessageNumber(t *testing.T) {
	const clientMsgNo = "fedg0-event-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/message/send/durable" {
			t.Fatalf("path = %s, want /message/send/durable", r.URL.Path)
		}

		var request MsgSendReq
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.ClientMsgNo != clientMsgNo {
			t.Fatalf("client_msg_no = %q, want %q", request.ClientMsgNo, clientMsgNo)
		}

		writeMessageSendResponse(t, w, http.StatusOK, map[string]any{
			"status": http.StatusOK,
			"data": map[string]any{
				"message_id":    101,
				"message_seq":   7,
				"client_msg_no": clientMsgNo,
			},
		})
	}))
	defer server.Close()

	result, err := newMessageSendTestContext(server.URL).SendDurableMessageWithResult(&MsgSendReq{
		ChannelID:   "fedg0_channel",
		ChannelType: 2,
		ClientMsgNo: clientMsgNo,
		Payload:     []byte(`{"type":1,"content":"fedg0"}`),
	})
	if err != nil {
		t.Fatalf("SendDurableMessageWithResult() error = %v", err)
	}
	if result.MessageID != 101 || result.MessageSeq != 7 || result.ClientMsgNo != clientMsgNo {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSendDurableMessageWithResult_RejectsInvalidSuccessResponse(t *testing.T) {
	const clientMsgNo = "fedg0-event-456"

	testCases := []struct {
		name string
		data map[string]any
	}{
		{
			name: "missing message sequence",
			data: map[string]any{
				"message_id":    101,
				"client_msg_no": clientMsgNo,
			},
		},
		{
			name: "zero message sequence",
			data: map[string]any{
				"message_id":    101,
				"message_seq":   0,
				"client_msg_no": clientMsgNo,
			},
		},
		{
			name: "mismatched client message number",
			data: map[string]any{
				"message_id":    101,
				"message_seq":   7,
				"client_msg_no": "different-event",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeMessageSendResponse(t, w, http.StatusOK, map[string]any{
					"status": http.StatusOK,
					"data":   testCase.data,
				})
			}))
			defer server.Close()

			_, err := newMessageSendTestContext(server.URL).SendDurableMessageWithResult(&MsgSendReq{
				ChannelID:   "fedg0_channel",
				ChannelType: 2,
				ClientMsgNo: clientMsgNo,
				Payload:     []byte(`{"type":1}`),
			})
			assertMessageSendErrorKind(t, err, "invalid_success_response", ErrMessageSendInvalidSuccessResponse)
		})
	}
}

func TestSendDurableMessageWithResult_ClassifiesNonSuccessOutcomes(t *testing.T) {
	const clientMsgNo = "fedg0-event-789"

	testCases := []struct {
		name       string
		statusCode int
		wantKind   string
		wantError  error
	}{
		{name: "idempotency conflict", statusCode: http.StatusConflict, wantKind: "idempotency_conflict", wantError: ErrMessageSendIdempotencyConflict},
		{name: "http rejection", statusCode: http.StatusForbidden, wantKind: "http_rejected", wantError: ErrMessageSendHTTPRejected},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeMessageSendResponse(t, w, testCase.statusCode, map[string]any{"status": testCase.statusCode})
			}))
			defer server.Close()

			_, err := newMessageSendTestContext(server.URL).SendDurableMessageWithResult(&MsgSendReq{
				ChannelID:   "fedg0_channel",
				ChannelType: 2,
				ClientMsgNo: clientMsgNo,
			})
			assertMessageSendErrorKind(t, err, testCase.wantKind, testCase.wantError)
		})
	}
}

func TestSendDurableMessageWithResult_ClassifiesTransportUnknown(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	serverURL := server.URL
	server.Close()

	_, err := newMessageSendTestContext(serverURL).SendDurableMessageWithResult(&MsgSendReq{
		ChannelID:   "fedg0_channel",
		ChannelType: 2,
		ClientMsgNo: "fedg0-event-transport",
	})
	assertMessageSendErrorKind(t, err, "transport_unknown", ErrMessageSendTransportUnknown)
}

func TestSendMessagePreservesLegacyEndpointForNoPersist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message/send" {
			t.Fatalf("path = %s, want /message/send", r.URL.Path)
		}
		writeMessageSendResponse(t, w, http.StatusOK, map[string]any{
			"message_id":  0,
			"message_seq": 0,
			"reason":      1,
		})
	}))
	defer server.Close()

	err := newMessageSendTestContext(server.URL).SendMessage(&MsgSendReq{
		Header:      MsgHeader{NoPersist: 1, SyncOnce: 1},
		FromUID:     "u1",
		ChannelID:   "u2",
		ChannelType: 1,
		Payload:     []byte(`{"type":1}`),
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
}

func TestSendMessageWithResultAcceptsLegacyTopLevelResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message/send" {
			t.Fatalf("path = %s, want /message/send", r.URL.Path)
		}
		writeMessageSendResponse(t, w, http.StatusOK, map[string]any{
			"message_id":    202,
			"message_seq":   8,
			"client_msg_no": "legacy-key",
			"reason":        1,
		})
	}))
	defer server.Close()

	result, err := newMessageSendTestContext(server.URL).SendMessageWithResult(&MsgSendReq{
		ClientMsgNo: "legacy-key",
	})
	if err != nil {
		t.Fatalf("SendMessageWithResult() error = %v", err)
	}
	if result.MessageID != 202 || result.MessageSeq != 8 || result.ClientMsgNo != "legacy-key" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSendDurableMessageWithResultRejectsMissingClientMessageNumberBeforeSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("durable request without client_msg_no must not be sent")
	}))
	defer server.Close()

	_, err := newMessageSendTestContext(server.URL).SendDurableMessageWithResult(&MsgSendReq{})
	assertMessageSendErrorKind(t, err, "invalid_request", ErrMessageSendInvalidRequest)
}

func newMessageSendTestContext(apiURL string) *Context {
	cfg := &Config{}
	cfg.WuKongIM.APIURL = apiURL
	return &Context{cfg: cfg}
}

func writeMessageSendResponse(t *testing.T, w http.ResponseWriter, statusCode int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func assertMessageSendErrorKind(t *testing.T, err error, want string, sentinel error) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want kind %q", want)
	}
	kind, ok := err.(interface{ MessageSendErrorKind() string })
	if !ok {
		t.Fatalf("error %T does not expose MessageSendErrorKind(): %v", err, err)
	}
	if got := kind.MessageSendErrorKind(); got != want {
		t.Fatalf("error kind = %q, want %q", got, want)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("errors.Is(%v, %v) = false", err, sentinel)
	}
}
