package nativeassets

import (
	"hash/crc32"
	f "hyrulelauncher/internal/gamefmt"
	"testing"
)

func TestResourceGrowthAndSingleNodeBudget(t *testing.T) {
	source := "Actor/ASList/Synthetic.baslist"
	dst := "Actor/ASList/Private.baslist"
	r := &f.RSTB{Hashes: map[uint32]uint32{crc32.ChecksumIEEE([]byte(source)): 200000}, Names: map[string]uint32{}}
	if err := reserveProxyResource(r, dst, source, 2000, 1000, false); err != nil {
		t.Fatal(err)
	}
	if resourceBudget(r, dst) < 201000 || resourceBudget(r, source) != 200000 {
		t.Fatal("growth not reserved or source modified")
	}
	if err := reserveProxyResource(r, "Actor/AS/Private.bas", "Actor/AS/OldPlayer.bas", 140, 9000, true); err != nil {
		t.Fatal(err)
	}
	v := resourceBudget(r, "Actor/AS/Private.bas")
	if v < 2176 || v >= 0x10000 {
		t.Fatal("single node assigned a full Player graph budget", v)
	}
}
