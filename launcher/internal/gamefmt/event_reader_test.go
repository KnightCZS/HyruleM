package gamefmt

import (
	"encoding/binary"
	"os"
	"reflect"
	"testing"
)

func TestFlowEntriesReadsIndependentEmptyQuest(t *testing.T) {
	b, e := os.ReadFile("testdata/synthetic-empty-quest.bfevfl")
	if e != nil {
		t.Fatal(e)
	}
	name, entries, e := FlowEntries(b)
	if e != nil {
		t.Fatalf("valid empty quest flow was rejected (issue #1): %v", e)
	}
	if name != "SyntheticEmptyQuest" || len(entries) != 0 {
		t.Fatal(name, entries)
	}
}
func TestFlowEntriesReadsIndependentEntryPoints(t *testing.T) {
	b, e := os.ReadFile("testdata/synthetic-entry-flow.bfevfl")
	if e != nil {
		t.Fatal(e)
	}
	name, entries, e := FlowEntries(b)
	if e != nil || name != "SyntheticMain" || !reflect.DeepEqual(entries, []string{"Start", "InternalHelper"}) {
		t.Fatal(name, entries, e)
	}
}

func TestFlowEntriesEmptyQuestStillRejectsMalformedTables(t *testing.T) {
	original, e := os.ReadFile("testdata/synthetic-empty-quest.bfevfl")
	if e != nil {
		t.Fatal(e)
	}
	le := binary.LittleEndian
	flow := le.Uint64(original[le.Uint64(original[0x28:]):])
	dic := le.Uint64(original[flow+0x38:])
	cases := map[string]func([]byte){
		"unknown version":                 func(b []byte) { le.PutUint16(b[8:], 0x400) },
		"mismatched counts":               func(b []byte) { le.PutUint16(b[flow+0x18:], 1) },
		"dictionary claims missing entry": func(b []byte) { le.PutUint32(b[dic+4:], 1) },
		"invalid dictionary pointer":      func(b []byte) { le.PutUint64(b[flow+0x38:], ^uint64(0)) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			b := append([]byte(nil), original...)
			mutate(b)
			if _, _, e := FlowEntries(b); e == nil {
				t.Fatal("accepted malformed table")
			}
		})
	}
	for length := 0; length < len(original); length++ {
		b := original[:length]
		_, _, _ = FlowEntries(b)
	}
}
