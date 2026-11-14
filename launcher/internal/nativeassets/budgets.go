package nativeassets

import (
	"fmt"
	"hash/crc32"
	f "hyrulelauncher/internal/gamefmt"
)

func resourceBudget(t *f.RSTB, path string) uint32 {
	path = f.Canonical(path)
	v := t.Hashes[crc32.ChecksumIEEE([]byte(path))]
	if n := t.Names[path]; n > v {
		v = n
	}
	return v
}

// A canonical proxy AS has exactly one skeletal node (3 lists/1 object/3
// parameters). Its overhead is bounded independently of the old Player graph.
// The owner's 1.9 single-node animal resources reserve ~2 KiB plus file data;
// use 4 KiB here, rather than adding 64 KiB to every one of ~400 tiny files.
func reserveProxyResource(t *f.RSTB, path, source string, size, oldSize int, skeletal bool) error {
	if size < 0 || oldSize < 0 || size > f.MaxData || oldSize > f.MaxData {
		return fmt.Errorf("orçamento de recurso inválido")
	}
	align := func(n int) uint64 { return uint64((n + 31) &^ 31) }
	needed := align(size) + 0x1000
	if !skeletal {
		base := uint64(resourceBudget(t, source))
		growth := uint64(0)
		if size > oldSize {
			growth = align(size) - align(oldSize)
		}
		if base+growth+0x100 > needed {
			needed = base + growth + 0x100
		}
	}
	if needed > 0xffffffff {
		return fmt.Errorf("orçamento de recurso excessivo")
	}
	path = f.Canonical(path)
	if old, ok := t.Names[path]; ok {
		if uint64(old) < needed {
			t.Names[path] = uint32(needed)
		}
		return nil
	}
	h := crc32.ChecksumIEEE([]byte(path))
	if uint64(t.Hashes[h]) < needed {
		t.Hashes[h] = uint32(needed)
	}
	return nil
}
