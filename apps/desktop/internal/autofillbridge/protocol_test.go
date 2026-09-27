package autofillbridge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestAMessageTravelsAsItsLengthThenItsJSON(t *testing.T) {
	var wire bytes.Buffer
	if err := writeMessage(&wire, maxAnswerBytes, answer{User: "alex", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	want := `{"user":"alex","password":"secret"}`
	if got := binary.BigEndian.Uint32(wire.Bytes()[:4]); got != uint32(len(want)) {
		t.Fatalf("length prefix = %d, want %d", got, len(want))
	}
	if got := wire.String()[4:]; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
	var read answer
	if err := readMessage(&wire, maxAnswerBytes, &read); err != nil {
		t.Fatal(err)
	}
	if read.User != "alex" || read.Password != "secret" {
		t.Fatalf("read %+v", read)
	}
}

func TestAMessageLargerThanTheLimitIsRefusedBeforeItsBodyIsRead(t *testing.T) {
	var wire bytes.Buffer
	wire.Write(binary.BigEndian.AppendUint32(nil, maxRequestBytes+1))
	var asked request
	if err := readMessage(&wire, maxRequestBytes, &asked); !errors.Is(err, errFrame) {
		t.Fatalf("got %v, want errFrame", err)
	}
}

func TestAnEmptyMessageIsRefused(t *testing.T) {
	var wire bytes.Buffer
	wire.Write(binary.BigEndian.AppendUint32(nil, 0))
	var asked request
	if err := readMessage(&wire, maxRequestBytes, &asked); !errors.Is(err, errFrame) {
		t.Fatalf("got %v, want errFrame", err)
	}
}

func TestAMessageCutShortIsRefused(t *testing.T) {
	var wire bytes.Buffer
	wire.Write(binary.BigEndian.AppendUint32(nil, 20))
	wire.WriteString(`{"op":"status"}`)
	var asked request
	if err := readMessage(&wire, maxRequestBytes, &asked); err == nil {
		t.Fatal("a message shorter than its length was read")
	}
}

func TestAnAnswerLargerThanTheLimitIsNotWritten(t *testing.T) {
	var wire bytes.Buffer
	large := answer{Password: string(make([]byte, 64))}
	if err := writeMessage(&wire, 32, large); !errors.Is(err, errFrame) {
		t.Fatalf("got %v, want errFrame", err)
	}
	if wire.Len() != 0 {
		t.Fatalf("wrote %d bytes of a refused answer", wire.Len())
	}
}
