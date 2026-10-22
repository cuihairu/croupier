package pg

import (
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

func TestParseLSN(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    LSN
		wantErr bool
	}{
		{name: "标准十六进制形态", in: "16/3C219F48", want: LSN(0x16)<<32 | LSN(0x3C219F48)},
		{name: "高位为零", in: "0/16B37D0", want: LSN(0x16B37D0)},
		{name: "十进制形态（Decimal 回放）", in: "2833169768", want: LSN(2833169768)},
		{name: "空串", in: "", wantErr: true},
		{name: "空白", in: "   ", wantErr: true},
		{name: "十六进制段溢出", in: "100000000/0", wantErr: true},
		{name: "非数垃圾", in: "abc/def!", wantErr: true},
		{name: "十进制垃圾", in: "12x3", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLSN(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseLSN(%q) = %v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLSN(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseLSN(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestLSNFormats(t *testing.T) {
	// 三形态互逆：wire 展示 "X/Y" ↔ uint64 ↔ checkpoint 十进制串。
	orig := LSN(0x16)<<32 | LSN(0x3C219F48)
	if got := orig.String(); got != "16/3C219F48" {
		t.Fatalf("String() = %q, want 16/3C219F48", got)
	}
	back, err := ParseLSN(orig.String())
	if err != nil || back != orig {
		t.Fatalf("ParseLSN(String): %v, %d", err, back)
	}
	dec, err := ParseLSN(orig.Decimal())
	if err != nil || dec != orig {
		t.Fatalf("ParseLSN(Decimal): %v, %d", err, dec)
	}
}

func TestDecoderCheckpoint(t *testing.T) {
	d := NewDecoder("src-1")
	if got := d.Checkpoint(); got.Valid() {
		t.Fatalf("fresh decoder checkpoint = %+v, want zero position", got)
	}
	d.curLSN = LSN(0x10)<<32 | 0xABC
	want := connector.Position{Kind: "lsn", Value: (LSN(0x10)<<32 | 0xABC).Decimal()}
	if got := d.Checkpoint(); got != want {
		t.Fatalf("Checkpoint() = %+v, want %+v", got, want)
	}
	if _, err := connector.ParsePosition(want.String()); err != nil {
		t.Fatalf("checkpoint roundtrip: %v", err)
	}
}
