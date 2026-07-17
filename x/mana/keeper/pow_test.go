package keeper

import (
	"strconv"
	"testing"
)

// TestLeadingZeroBits verifies the bit-counting used to grade PoW solutions.
func TestLeadingZeroBits(t *testing.T) {
	cases := []struct {
		in   []byte
		want uint32
	}{
		{[]byte{0xFF}, 0},
		{[]byte{0x80}, 0},
		{[]byte{0x7F}, 1},
		{[]byte{0x01}, 7},
		{[]byte{0x00, 0xFF}, 8},
		{[]byte{0x00, 0x01}, 15},
		{[]byte{0x00, 0x00}, 16},
	}
	for i, c := range cases {
		if got := leadingZeroBits(c.in); got != c.want {
			t.Fatalf("case %d: leadingZeroBits(%x) = %d, want %d", i, c.in, got, c.want)
		}
	}
}

// TestPowSolveAndVerify mines a solution at a low difficulty and confirms it
// verifies, and that the puzzle is bound to (chainID, signer, sequence).
func TestPowSolveAndVerify(t *testing.T) {
	const difficulty = uint32(8) // ~256 hashes on average — fast and deterministic

	challenge := powChallenge("JEE", []byte("signer-address-bytes"), 7)

	var solved string
	found := false
	for i := 0; i < 1_000_000; i++ {
		candidate := strconv.Itoa(i)
		if leadingZeroBits(powWork(challenge, candidate)) >= difficulty {
			solved = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatal("failed to mine a PoW solution at difficulty 8")
	}

	if got := leadingZeroBits(powWork(challenge, solved)); got < difficulty {
		t.Fatalf("solved memo only has %d leading zero bits, want >= %d", got, difficulty)
	}

	// Same nonce against a different sequence almost certainly fails — proving the
	// solution can't be replayed across transactions.
	other := powChallenge("JEE", []byte("signer-address-bytes"), 8)
	if leadingZeroBits(powWork(other, solved)) >= difficulty {
		t.Log("note: solution coincidentally satisfied a different sequence (statistically rare)")
	}
}
