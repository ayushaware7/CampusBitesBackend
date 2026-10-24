package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidSignature(t *testing.T) {
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("order_1|pay_1"))
	good := hex.EncodeToString(mac.Sum(nil))

	if !validSignature("secret", "order_1", "pay_1", good) {
		t.Error("valid signature rejected")
	}
	if validSignature("secret", "order_1", "pay_2", good) {
		t.Error("signature for a different payment accepted")
	}
	if validSignature("other", "order_1", "pay_1", good) {
		t.Error("signature made with a different secret accepted")
	}
	if validSignature("secret", "order_1", "pay_1", "") {
		t.Error("empty signature accepted")
	}
}

func TestRupeesToPaise(t *testing.T) {
	// 0.1+0.2 style float drift must not change the charged amount
	if got := rupeesToPaise(0.1 + 0.2); got != 30 {
		t.Errorf("got %d, want 30", got)
	}
	if got := rupeesToPaise(149.99); got != 14999 {
		t.Errorf("got %d, want 14999", got)
	}
}
